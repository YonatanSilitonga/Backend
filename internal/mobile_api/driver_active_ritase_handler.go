package mobile_api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// ── JADWAL: helper untuk mencocokkan jam sekarang dengan jendela waktu ritase ──

func parseJam(s string) (time.Time, error) {
	if idx := strings.Index(s, "."); idx != -1 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if t, err := time.Parse("15:04:05", s); err == nil {
		return t, nil
	}
	return time.Parse("15:04", s)
}

// tripEndTime menghitung waktu-habis aktual sebuah ritase:
// tanggal + jam_selesai (+1 hari kalau melewati tengah malam).
// Return false bila tanggal/jam tidak bisa diparse (tidak bisa dinilai).
func tripEndTime(tanggalStr, jamMulaiStr, jamSelesaiStr string, loc *time.Location) (time.Time, bool) {
	tgl, err := time.ParseInLocation("2006-01-02", tanggalStr, loc)
	if err != nil {
		return time.Time{}, false
	}
	mulai, err := parseJam(jamMulaiStr)
	if err != nil {
		return time.Time{}, false
	}
	selesai, err := parseJam(jamSelesaiStr)
	if err != nil {
		return time.Time{}, false
	}
	end := time.Date(tgl.Year(), tgl.Month(), tgl.Day(),
		selesai.Hour(), selesai.Minute(), 0, 0, loc)
	start := time.Date(tgl.Year(), tgl.Month(), tgl.Day(),
		mulai.Hour(), mulai.Minute(), 0, 0, loc)
	if !end.After(start) {
		end = end.Add(24 * time.Hour)
	}
	return end, true
}

// ritaseCandidate menampung 1 baris ritase kandidat (belum selesai) milik seorang driver hari ini.
type ritaseCandidate struct {
	IDRitase       int64
	IDKendaraan    int64
	Status         string
	KodeRitase     string
	PlatNomor      string
	JenisKendaraan string
	RitaseKe       int
	JenisRitase    *string
	Tanggal        *string
	JamMulai       *string
	JamSelesai     *string
}

func (h *APIHandler) GetActiveRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	idDriverParam := c.QueryParam("id_driver")
	idDriver, _ := strconv.ParseInt(idDriverParam, 10, 64)

	if idDriver == 0 {
		return response.Error(c, http.StatusBadRequest, "id_driver harus diisi")
	}

	// ── PAKSA WAKTU JAKARTA DI GOLANG ──
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	hariIni := time.Now().In(loc).Format("2006-01-02")

	// 1. Ambil SEMUA ritase yang belum selesai untuk driver ini hari ini
	rowsCand, err := h.DB.Query(ctx, `
		SELECT r.id_ritase, r.id_kendaraan, r.status, r.kode_ritase,
			COALESCE(k.plat_nomor, ''), COALESCE(k.jenis_kendaraan, ''),
			r.ritase_ke, r.jenis_ritase,
			TO_CHAR(r.tanggal, 'YYYY-MM-DD'),
			TO_CHAR(r.jam_mulai, 'HH24:MI:SS'), TO_CHAR(r.jam_selesai, 'HH24:MI:SS')
		FROM ritase r
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.id_driver = $1 AND r.status != 'selesai' 
		  AND (
		    r.status = 'berjalan'
		    OR r.tanggal = $2 
		    OR r.tanggal IS NULL
		    OR (r.tanggal = ($2::date - INTERVAL '1 day')::date AND r.jam_selesai < r.jam_mulai)
		  )
		ORDER BY r.ritase_ke ASC, r.id_ritase ASC
		
	`, idDriver, hariIni)
	if err != nil {
		log.Printf("Query error on active ritase candidates: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil kandidat ritase")
	}

	var candidates []ritaseCandidate
	for rowsCand.Next() {
		var rc ritaseCandidate
		if err := rowsCand.Scan(&rc.IDRitase, &rc.IDKendaraan, &rc.Status, &rc.KodeRitase,
			&rc.PlatNomor, &rc.JenisKendaraan, &rc.RitaseKe, &rc.JenisRitase, &rc.Tanggal, &rc.JamMulai, &rc.JamSelesai); err == nil {
			candidates = append(candidates, rc)
		}
	}
	rowsCand.Close()
	waktuRequest := time.Now().In(loc).Format("2006-01-02 15:04:05")
	log.Printf("--------------------------------------------------")
	log.Printf("📱 [MOBILE_REQUEST] Driver ID: %d mengecek rute", idDriver)
	log.Printf("⏰ Jam Request Server : %s WIB", waktuRequest)
	log.Printf("📅 Tanggal Dicari      : %s", hariIni)
	log.Printf("📊 Jumlah Rute Valid  : %d kandidat", len(candidates))
	for _, cand := range candidates {
		jm, js := "-", "-"
		if cand.JamMulai != nil {
			jm = *cand.JamMulai
		}
		if cand.JamSelesai != nil {
			js = *cand.JamSelesai
		}
		log.Printf("   👉 Ritase %d (ID: %d | Plat: %s) [%s - %s]", cand.RitaseKe, cand.IDRitase, cand.PlatNomor, jm, js)
	}
	log.Printf("--------------------------------------------------")

	if len(candidates) == 0 {
		var countFinishedToday int
		_ = h.DB.QueryRow(ctx, `
			SELECT COUNT(*) FROM ritase
			WHERE id_driver = $1 AND status = 'selesai' AND tanggal = $2
		`, idDriver, hariIni).Scan(&countFinishedToday)

		return response.OK(c, map[string]interface{}{
			"has_active_ritase": false,
			"all_completed":     countFinishedToday > 0,
		})
	}

	// 2. Pilih trip — tiga lapis, tanpa penguncian jam:
	//  Lapis 1: yang sedang 'berjalan' selalu menang (tidak diganggu).
	//  Lapis 2: dari sisanya, buka yang waktu-habisnya paling dekat.
	//  Lapis 3: yang sudah lewat jam-habis tapi belum selesai = kadaluarsa,
	//           disembunyikan dari sopir (pengawas yang bereskan).
	now := time.Now().In(loc)

	var picked *ritaseCandidate
	var bestEnd time.Time
	var timeless *ritaseCandidate

	for i := range candidates {
		cand := &candidates[i]

		// Lapis 1: yang berjalan langsung menang.
		if cand.Status == "berjalan" {
			picked = cand
			break
		}

		// Tanpa info waktu → tidak bisa dinilai kadaluarsa; jadikan cadangan.
		if cand.Tanggal == nil || cand.JamMulai == nil || cand.JamSelesai == nil {
			if timeless == nil {
				timeless = cand
			}
			continue
		}

		end, ok := tripEndTime(*cand.Tanggal, *cand.JamMulai, *cand.JamSelesai, loc)
		if !ok {
			if timeless == nil {
				timeless = cand
			}
			continue
		}

		// Lapis 3: sudah lewat jam-habis → kadaluarsa, lewati.
		if now.After(end) {
			log.Printf("   ⏭️ Ritase %d (ID: %d) kadaluarsa (habis %s), disembunyikan",
				cand.RitaseKe, cand.IDRitase, end.Format("2006-01-02 15:04"))
			continue
		}

		// Lapis 2: yang habisnya paling dekat menang.
		if picked == nil || end.Before(bestEnd) {
			picked = cand
			bestEnd = end
		}
	}

	// Cadangan tanpa info waktu hanya dipakai bila tidak ada kandidat berwaktu.
	if picked == nil {
		picked = timeless
	}

	if picked == nil {
		var countFinishedToday int
		_ = h.DB.QueryRow(ctx, `
			SELECT COUNT(*) FROM ritase
			WHERE id_driver = $1 AND status = 'selesai' AND tanggal = $2
		`, idDriver, hariIni).Scan(&countFinishedToday)

		return response.OK(c, map[string]interface{}{
			"has_active_ritase": false,
			"all_completed":     countFinishedToday > 0,
			"message":           "Tidak ada jadwal ritase tersisa hari ini.",
		})
	}

	idRitase := picked.IDRitase
	assignedKendaraanID := picked.IDKendaraan
	statusRitase := picked.Status
	kodeRitase := picked.KodeRitase
	platNomor := picked.PlatNomor
	jenisKendaraan := picked.JenisKendaraan

	// 3. Ambil Urutan Stop (termasuk implant: join tabel implant agar stop
	// implant tidak jatuh ke fallback 'Gudang OG' tanpa koordinat)
	rows, err := h.DB.Query(ctx, `
		SELECT
			rs.id_stop, rs.urutan,
			CASE WHEN rs.id_implant IS NOT NULL THEN 'implant' ELSE rs.jenis_stop END AS jenis_stop,
			rs.id_seller, rs.id_implant, rs.id_drop_point, rs.id_gudang, rs.keterangan,
			COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Gudang OG') AS nama_lokasi,
			COALESCE(i.alamat, s.alamat, dp.alamat, g.alamat, 'Gudang Outgoing Utama') AS alamat,
			COALESCE(i.no_hp, s.no_hp, '-') AS no_hp,
			COALESCE(i.latitude, s.latitude, g.latitude) AS latitude,
			COALESCE(i.longitude, s.longitude, g.longitude) AS longitude,
			(COALESCE(s.is_implant, FALSE) OR rs.id_implant IS NOT NULL) AS is_implant
		FROM ritase_stop rs
		LEFT JOIN seller s ON s.id_seller = rs.id_seller
		LEFT JOIN implant i ON i.id_implant = rs.id_implant
		LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
		LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
		WHERE rs.id_ritase = $1
		ORDER BY rs.urutan ASC
	`, idRitase)

	if err != nil {
		log.Printf("Query error on active ritase stops: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data rute")
	}
	defer rows.Close()

	var stops []map[string]interface{}
	for rows.Next() {
		var idStop int64
		var urutan int
		var jenisStop, namaLokasi, alamat, noHp string
		var idSeller, idImplant, idDropPoint, idGudang *int64
		var keterangan *string
		var latitude, longitude *float64
		var isImplant bool

		if err := rows.Scan(&idStop, &urutan, &jenisStop, &idSeller, &idImplant, &idDropPoint, &idGudang, &keterangan, &namaLokasi, &alamat, &noHp, &latitude, &longitude, &isImplant); err == nil {
			stop := map[string]interface{}{
				"id_stop":       idStop,
				"urutan":        urutan,
				"jenis_stop":    jenisStop,
				"id_seller":     idSeller,
				"id_implant":    idImplant,
				"id_drop_point": idDropPoint,
				"id_gudang":     idGudang,
				"keterangan":    keterangan,
				"nama_lokasi":   namaLokasi,
				"alamat":        alamat,
				"no_hp":         noHp,
				"is_implant":    isImplant,
			}
			if latitude != nil {
				stop["latitude"] = *latitude
			}
			if longitude != nil {
				stop["longitude"] = *longitude
			}
			stops = append(stops, stop)
		}
	}

	// Catatan: kaki pulang otomatis ke Outgoing sengaja TIDAK ditempel di sini.
	// Pengembalian armada diatur murni lewat jadwal ritase (tambah stop gudang
	// bila perlu), agar mobil bisa lanjut ke implant/ritase berikutnya.
	var stageStartedAt *time.Time
	_ = h.DB.QueryRow(ctx, `
		SELECT MAX(created_at) FROM ritase_event WHERE id_ritase = $1
	`, idRitase).Scan(&stageStartedAt)

	var currentStopIndex int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM ritase_event WHERE id_ritase = $1 AND LOWER(status) = 'tiba'
	`, idRitase).Scan(&currentStopIndex)

	var lastStatus *string
	_ = h.DB.QueryRow(ctx, `
		SELECT status FROM ritase_event
		WHERE id_ritase = $1
		ORDER BY created_at DESC, id_event DESC
		LIMIT 1
	`, idRitase).Scan(&lastStatus)

	resp := map[string]interface{}{
		"has_active_ritase":  true,
		"id_ritase":          idRitase,
		"id_kendaraan":       assignedKendaraanID,
		"kode_ritase":        kodeRitase,
		"status":             statusRitase,
		"stops":              stops,
		"stage_started_at":   stageStartedAt,
		"current_stop_index": currentStopIndex,
		"last_status":        lastStatus,
		"plat_nomor":         platNomor,
		"jenis_kendaraan":    jenisKendaraan,
		"ritase_ke":          picked.RitaseKe,
		"jenis_ritase":       picked.JenisRitase,
		"jam_mulai":          picked.JamMulai,
		"jam_selesai":        picked.JamSelesai,
	}

	return response.OK(c, resp)
}