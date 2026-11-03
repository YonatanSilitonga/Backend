package mobile_api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

func (h *APIHandler) GetDriverHistoryRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 6*time.Second)
	defer cancel()

	idDriverParam := c.QueryParam("id_driver")
	idDriver, _ := strconv.ParseInt(idDriverParam, 10, 64)

	if idDriver == 0 {
		return response.Error(c, http.StatusBadRequest, "id_driver harus diisi")
	}

	filter := c.QueryParam("filter") // "today", "week", "month", "all"

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	nowWIB := time.Now().In(loc)

	query := `
		SELECT r.id_ritase, r.kode_ritase, COALESCE(TO_CHAR(r.tanggal, 'YYYY-MM-DD'), ''),
			r.status, COALESCE(r.ritase_ke, 1), COALESCE(r.jenis_ritase, 'Reguler'),
			COALESCE(k.plat_nomor, '-'), COALESCE(k.jenis_kendaraan, '-'),
			COALESCE(TO_CHAR(r.jam_mulai, 'HH24:MI'), ''),
			COALESCE(TO_CHAR(r.jam_selesai, 'HH24:MI'), ''),
			COALESCE((
				SELECT EXTRACT(EPOCH FROM (MAX(created_at) - MIN(created_at)))::int
				FROM ritase_event
				WHERE id_ritase = r.id_ritase
			), 0),
			COALESCE(sub_ev.total_koli, 0),
			COALESCE(sub_ev.total_ecer, 0),
			COALESCE(sub_ev.total_hv, 0),
			COALESCE(sub_stop.total_stops, 0)
		FROM ritase r
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		LEFT JOIN (
			SELECT id_ritase,
			       SUM(COALESCE(jumlah_koli, 0)) as total_koli,
			       SUM(COALESCE(jumlah_ecer, 0)) as total_ecer,
			       SUM(COALESCE(jumlah_high_value, 0)) as total_hv
			FROM ritase_event
			WHERE status IN ('Bongkar Muat Barang', 'Muat Barang')
			GROUP BY id_ritase
		) sub_ev ON sub_ev.id_ritase = r.id_ritase
		LEFT JOIN (
			SELECT id_ritase, COUNT(*) as total_stops
			FROM ritase_stop
			GROUP BY id_ritase
		) sub_stop ON sub_stop.id_ritase = r.id_ritase
		WHERE r.id_driver = $1 AND r.status = 'selesai'
	`

	var args []interface{}
	args = append(args, idDriver)

	if filter == "today" {
		query += fmt.Sprintf(" AND r.tanggal = '%s'", nowWIB.Format("2006-01-02"))
	} else if filter == "week" {
		startWeek := nowWIB.AddDate(0, 0, -int(nowWIB.Weekday()))
		query += fmt.Sprintf(" AND r.tanggal >= '%s'", startWeek.Format("2006-01-02"))
	} else if filter == "month" {
		startMonth := time.Date(nowWIB.Year(), nowWIB.Month(), 1, 0, 0, 0, 0, loc)
		query += fmt.Sprintf(" AND r.tanggal >= '%s'", startMonth.Format("2006-01-02"))
	}

	query += " ORDER BY r.tanggal DESC, r.id_ritase DESC LIMIT 50"

	rows, err := h.DB.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error get driver history: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil riwayat ritase")
	}
	defer rows.Close()

	type HistoryItem struct {
		IDRitase       int64  `json:"id_ritase"`
		KodeRitase     string `json:"kode_ritase"`
		Tanggal        string `json:"tanggal"`
		Status         string `json:"status"`
		RitaseKe       int    `json:"ritase_ke"`
		JenisRitase    string `json:"jenis_ritase"`
		PlatNomor      string `json:"plat_nomor"`
		JenisKendaraan string `json:"jenis_kendaraan"`
		JamMulai       string `json:"jam_mulai"`
		JamSelesai     string `json:"jam_selesai"`
		TotalDurasi    int    `json:"total_durasi"`
		TotalKoli      int    `json:"total_koli"`
		TotalEcer      int    `json:"total_ecer"`
		TotalHV        int    `json:"total_high_value"`
		TotalStops     int    `json:"total_stops"`
	}

	var list []HistoryItem
	for rows.Next() {
		var item HistoryItem
		if err := rows.Scan(
			&item.IDRitase, &item.KodeRitase, &item.Tanggal,
			&item.Status, &item.RitaseKe, &item.JenisRitase,
			&item.PlatNomor, &item.JenisKendaraan,
			&item.JamMulai, &item.JamSelesai,
			&item.TotalDurasi,
			&item.TotalKoli, &item.TotalEcer, &item.TotalHV,
			&item.TotalStops,
		); err == nil {
			list = append(list, item)
		}
	}

	if list == nil {
		list = []HistoryItem{}
	}

	return response.OK(c, list)
}

func (h *APIHandler) GetDriverHistoryDetail(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 6*time.Second)
	defer cancel()

	idRitase, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if idRitase == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase is required")
	}

	stopsRows, err := h.DB.Query(ctx, `
		SELECT rs.id_stop, rs.urutan, rs.jenis_stop,
		       COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, rs.keterangan, 'Lokasi') AS nama_lokasi,
		       COALESCE(i.alamat, s.alamat, dp.alamat, g.alamat, '') AS alamat,
		       COALESCE(i.no_hp, s.no_hp, '') AS no_hp,
		       COALESCE(ev.koli, 0) AS koli,
		       COALESCE(ev.ecer, 0) AS ecer,
		       COALESCE(ev.hv, 0) AS hv,
		       COALESCE(NULLIF(rs.foto_manifest_url, ''), ev.photo_url, '') AS photo_url
		FROM ritase_stop rs
		LEFT JOIN ritase r ON r.id_ritase = rs.id_ritase
		LEFT JOIN seller s ON s.id_seller = rs.id_seller
		LEFT JOIN implant i ON i.id_implant = rs.id_implant
		LEFT JOIN drop_point dp ON (dp.id_drop_point = rs.id_drop_point OR (rs.jenis_stop = 'gateway' AND rs.id_drop_point IS NULL AND dp.id_drop_point = r.id_drop_point))
		LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
		LEFT JOIN LATERAL (
			SELECT COALESCE(MAX(re.jumlah_koli), 0) as koli,
			       COALESCE(MAX(re.jumlah_ecer), 0) as ecer,
			       COALESCE(MAX(re.jumlah_high_value), 0) as hv,
			       MAX(COALESCE(re.foto_manifest_url, '')) as photo_url
			FROM ritase_event re
			WHERE re.id_ritase = rs.id_ritase 
			  AND (
			    re.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
			    OR (re.nama_lokasi IS NOT NULL AND POSITION(LOWER(re.nama_lokasi) in LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
			    OR (re.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER(re.nama_lokasi)) > 0)
			    OR re.status IN ('Bongkar Muat Barang', 'Muat Barang', 'Bongkar Barang')
			  )
		) ev ON true
		WHERE rs.id_ritase = $1
		ORDER BY rs.urutan ASC
	`, idRitase)

	if err != nil {
		log.Printf("Error get driver history detail: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil detail ritase")
	}
	defer stopsRows.Close()

	type StopItem struct {
		IDStop     int64  `json:"id_stop"`
		Urutan     int    `json:"urutan"`
		JenisStop  string `json:"jenis_stop"`
		NamaLokasi string `json:"nama_lokasi"`
		Alamat     string `json:"alamat"`
		Telepon    string `json:"telepon"`
		Koli       int    `json:"koli"`
		Ecer       int    `json:"ecer"`
		HighValue  int    `json:"high_value"`
		PhotoURL   string `json:"photo_url"`
	}

	var stops []StopItem
	for stopsRows.Next() {
		var st StopItem
		if err := stopsRows.Scan(
			&st.IDStop, &st.Urutan, &st.JenisStop,
			&st.NamaLokasi, &st.Alamat, &st.Telepon,
			&st.Koli, &st.Ecer, &st.HighValue,
			&st.PhotoURL,
		); err == nil {
			stops = append(stops, st)
		}
	}

	if stops == nil {
		stops = []StopItem{}
	}

	return response.OK(c, map[string]interface{}{
		"id_ritase": idRitase,
		"stops":     stops,
	})
}
