package gudang

// Handler muatan gudang (koor gudang): mirror alur kapten (input_kapten +
// konfirmasi_penjemputan) dengan cakupan id_gudang. V1 dikhususkan untuk
// Gudang Outgoing — gudang di-resolve server-side, tanpa klaim JWT tambahan.

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"backend/internal/eventbus"
	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

type Handler struct {
	DB  *pgxpool.Pool
	bus *eventbus.Bus
}

func NewHandler(db *pgxpool.Pool, bus *eventbus.Bus) *Handler {
	return &Handler{DB: db, bus: bus}
}

// resolveOutgoingGudang mengembalikan Gudang Outgoing (dinamis by tipe/nama).
func (h *Handler) resolveOutgoingGudang(ctx context.Context) (id int64, nama string, err error) {
	err = h.DB.QueryRow(ctx, `
		SELECT id_gudang, nama_gudang FROM gudang
		WHERE LOWER(tipe) = 'outgoing' OR LOWER(nama_gudang) LIKE '%outgoing%'
		ORDER BY id_gudang ASC LIMIT 1
	`).Scan(&id, &nama)
	return id, nama, err
}

// GudangMuatanInputRequest adalah body input muatan oleh koor gudang.
// Mirror KaptenCargoInputRequest (field jumlah identik agar UX konsisten).
type GudangMuatanInputRequest struct {
	JenisRitase string `json:"jenis_ritase"` // default 'outgoing'
	RitaseKe    int    `json:"ritase_ke"`    // 1..4
	Catatan     string `json:"catatan"`
	JumlahAWB   int    `json:"jumlah_awb"`
	KoliJKT     int    `json:"koli_jkt"`
	KoliSEG     int    `json:"koli_seg"`
	KoliBTN     int    `json:"koli_btn"`
	EcerJKT     int    `json:"ecer_jkt"`
	EcerSEG     int    `json:"ecer_seg"`
	EcerBTN     int    `json:"ecer_btn"`
	KoliHVJKT   int    `json:"koli_hv_jkt"`
	KoliHVSEG   int    `json:"koli_hv_seg"`
	KoliHVBTN   int    `json:"koli_hv_btn"`
	EcerHVJKT   int    `json:"ecer_hv_jkt"`
	EcerHVSEG   int    `json:"ecer_hv_seg"`
	EcerHVBTN   int    `json:"ecer_hv_btn"`
	AwbHvJKT    int    `json:"awb_hv_jkt"`
	AwbHvSEG    int    `json:"awb_hv_seg"`
	AwbHvBTN    int    `json:"awb_hv_btn"`
	// Alias lama (kompatibilitas): menang hanya bila ketiga region masih 0.
	KoliHV int `json:"koli_hv"`
	HvAwb  int `json:"hv_awb"`
	EcerHV int `json:"ecer_hv"`
	AwbHv  int `json:"awb_hv"`
}

// normalisasiHV menggabungkan alias lama + kolom per-wilayah (mirror kapten).
func normalisasiHV(alias int, altAlias int, jkt int, seg int, btn int) (int, int, int) {
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	jkt, seg, btn = clamp(jkt), clamp(seg), clamp(btn)
	if jkt == 0 && seg == 0 && btn == 0 {
		if v := clamp(alias); v > 0 {
			return v, 0, 0
		}
		if v := clamp(altAlias); v > 0 {
			return v, 0, 0
		}
	}
	return jkt, seg, btn
}

func parseTimeMinutes(s string) int {
	hour, min := 0, 0
	fmt.Sscanf(s, "%d:%d", &hour, &min)
	return hour*60 + min
}

// PostMuatanInput mencatat input muatan gudang outgoing oleh koor gudang.
// POST /api/v1/gudang-muatan/input
func (h *Handler) PostMuatanInput(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	idGudang, namaGudang, err := h.resolveOutgoingGudang(ctx0)
	if err != nil || idGudang == 0 {
		return response.Error(c, http.StatusBadRequest, "gudang outgoing tidak ditemukan")
	}

	var req GudangMuatanInputRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if req.RitaseKe == 0 {
		req.RitaseKe = 1
	}

	// Validasi window jadwal (mirror kapten): input hanya dalam jam ritase.
	var jamMulai, jamSelesai string
	errJadwal := h.DB.QueryRow(ctx, `
		SELECT jam_mulai::text, jam_selesai::text
		FROM jadwal_ritase_config
		WHERE jenis = $1 AND ritase_ke = $2
	`, req.JenisRitase, req.RitaseKe).Scan(&jamMulai, &jamSelesai)

	if errJadwal == nil && jamMulai != "" && jamSelesai != "" {
		now := time.Now()
		nowMin := now.Hour()*60 + now.Minute()
		mulaiMin := parseTimeMinutes(jamMulai)
		selesaiMin := parseTimeMinutes(jamSelesai)
		startMin, endMin := mulaiMin, selesaiMin
		if endMin <= startMin {
			endMin += 1440
		}
		checkMin := nowMin
		if checkMin < startMin {
			checkMin += 1440
		}
		if checkMin < startMin || checkMin > endMin {
			return response.Error(c, http.StatusBadRequest,
				fmt.Sprintf("input hanya bisa dilakukan pada jam %s - %s", jamMulai[:5], jamSelesai[:5]))
		}
	}

	clamp0 := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	req.JumlahAWB = clamp0(req.JumlahAWB)
	req.KoliJKT, req.KoliSEG, req.KoliBTN = clamp0(req.KoliJKT), clamp0(req.KoliSEG), clamp0(req.KoliBTN)
	req.EcerJKT, req.EcerSEG, req.EcerBTN = clamp0(req.EcerJKT), clamp0(req.EcerSEG), clamp0(req.EcerBTN)
	hvKoliJkt, hvKoliSeg, hvKoliBtn := normalisasiHV(req.KoliHV, 0, req.KoliHVJKT, req.KoliHVSEG, req.KoliHVBTN)
	hvEcerJkt, hvEcerSeg, hvEcerBtn := normalisasiHV(req.HvAwb, req.EcerHV, req.EcerHVJKT, req.EcerHVSEG, req.EcerHVBTN)
	hvAwbJkt, hvAwbSeg, hvAwbBtn := normalisasiHV(req.AwbHv, 0, req.AwbHvJKT, req.AwbHvSEG, req.AwbHvBTN)

	var userID interface{}
	if uid, ok := c.Get(appMiddleware.CtxUserID).(int64); ok && uid > 0 {
		userID = uid
	}

	var insertedID int64
	err = h.DB.QueryRow(ctx, `
		INSERT INTO input_kapten (
			id_user, id_gudang, jenis_ritase, ritase_ke, catatan,
			jumlah_awb, koli_jkt, koli_seg, koli_btn,
			ecer_jkt, ecer_seg, ecer_btn,
			koli_hv_jkt, koli_hv_seg, koli_hv_btn,
			ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			awb_hv_jkt, awb_hv_seg, awb_hv_btn,
			nama_lokasi, id_ritase
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20, $21,
			$22, NULL
		) RETURNING id
	`, userID, idGudang, req.JenisRitase, req.RitaseKe, req.Catatan,
		req.JumlahAWB, req.KoliJKT, req.KoliSEG, req.KoliBTN,
		req.EcerJKT, req.EcerSEG, req.EcerBTN,
		hvKoliJkt, hvKoliSeg, hvKoliBtn,
		hvEcerJkt, hvEcerSeg, hvEcerBtn,
		hvAwbJkt, hvAwbSeg, hvAwbBtn,
		namaGudang,
	).Scan(&insertedID)
	if err != nil {
		log.Printf("[Gudang] gagal insert input muatan: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan data muatan")
	}

	// Total akumulasi lokasi hari ini (WIB).
	var locAwb, locKoli, locEcer, locHV, locAwbHv int
	_ = h.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn + awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0)
		FROM input_kapten
		WHERE id_gudang = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, idGudang, req.JenisRitase, req.RitaseKe).Scan(&locAwb, &locKoli, &locEcer, &locHV, &locAwbHv)

	h.bus.Publish("force_refresh", "gudang_muatan_input")

	return response.Created(c, map[string]interface{}{
		"id":           insertedID,
		"id_gudang":    idGudang,
		"nama_gudang":  namaGudang,
		"total_awb":    locAwb,
		"total_koli":   locKoli,
		"total_ecer":   locEcer,
		"total_hv":     locHV,
		"total_awb_hv": locAwbHv,
	})
}

// GetPending mengambil grup input hari ini (WIB) milik gudang outgoing
// yang belum ter-link ke ritase — bahan layar konfirmasi.
// GET /api/v1/gudang-muatan/pending
func (h *Handler) GetPending(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	idGudang, _, err := h.resolveOutgoingGudang(ctx0)
	if err != nil || idGudang == 0 {
		return response.Error(c, http.StatusBadRequest, "gudang outgoing tidak ditemukan")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT jenis_ritase, ritase_ke, COUNT(*),
		       COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn + awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0),
		       MAX(created_at)
		FROM input_kapten
		WHERE id_gudang = $1 AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		GROUP BY jenis_ritase, ritase_ke
		ORDER BY ritase_ke ASC
	`, idGudang)
	if err != nil {
		log.Printf("[Gudang] gagal ambil pending: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data pending")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var jenis string
		var ritKe, count, awb, koli, ecer, hv int
		var lastAt time.Time
		if err := rows.Scan(&jenis, &ritKe, &count, &awb, &koli, &ecer, &hv, &lastAt); err != nil {
			continue
		}
		list = append(list, map[string]interface{}{
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"jumlah_input": count,
			"total_awb":    awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
		})
	}

	return response.OK(c, list)
}

// GetKonfirmasi mengambil riwayat serah terima + sisa per grup hari ini (WIB).
// GET /api/v1/gudang-muatan/konfirmasi
func (h *Handler) GetKonfirmasi(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	idGudang, _, err := h.resolveOutgoingGudang(ctx0)
	if err != nil || idGudang == 0 {
		return response.Error(c, http.StatusBadRequest, "gudang outgoing tidak ditemukan")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT kp.id, kp.id_ritase, kp.jenis_ritase, kp.ritase_ke,
		       COALESCE(kp.jumlah_awb, 0),
		       COALESCE(kp.koli_jkt, 0), COALESCE(kp.koli_seg, 0), COALESCE(kp.koli_btn, 0),
		       COALESCE(kp.ecer_jkt, 0), COALESCE(kp.ecer_seg, 0), COALESCE(kp.ecer_btn, 0),
		       COALESCE(kp.foto_penjemputan_url, ''),
		       COALESCE(kp.catatan, ''),
		       kp.created_at,
		       COALESCE(r.kode_ritase, ''),
		       COALESCE(d.nama_driver, '')
		FROM konfirmasi_penjemputan kp
		LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		WHERE kp.id_gudang = $1
		  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		ORDER BY kp.created_at DESC
	`, idGudang)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id, idRitase int64
		var jenis string
		var ritKe, awb, kj, ks, kb, ej, es, eb int
		var foto, catatan, kode, driver string
		var createdAt time.Time
		if err := rows.Scan(&id, &idRitase, &jenis, &ritKe,
			&awb, &kj, &ks, &kb, &ej, &es, &eb,
			&foto, &catatan, &createdAt, &kode, &driver); err != nil {
			continue
		}
		tKoli := kj + ks + kb
		tEcer := ej + es + eb
		list = append(list, map[string]interface{}{
			"id": id, "id_ritase": idRitase,
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"jumlah_awb": awb,
			"koli_jkt":   kj, "koli_seg": ks, "koli_btn": kb,
			"ecer_jkt": ej, "ecer_seg": es, "ecer_btn": eb,
			"total_koli": tKoli, "total_ecer": tEcer,
			"foto_penjemputan_url": foto, "catatan": catatan,
			"created_at":  createdAt.Format(time.RFC3339),
			"kode_ritase": kode, "nama_driver": driver,
		})
	}

	// Sisa per grup: input − diambil.
	sisaGrup := []map[string]interface{}{}
	sRows, err := h.DB.Query(ctx, `
		WITH inp AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb
			FROM input_kapten t
			WHERE t.id_gudang = $1
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		),
		amb AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb
			FROM konfirmasi_penjemputan t
			WHERE t.id_gudang = $1
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		)
		SELECT COALESCE(i.jenis_ritase, a.jenis_ritase, ''),
		       COALESCE(i.ritase_ke, a.ritase_ke, 0),
		       COALESCE(i.awb, 0), COALESCE(i.koli, 0), COALESCE(i.ecer, 0), COALESCE(i.hv, 0),
		       COALESCE(a.awb, 0), COALESCE(a.koli, 0), COALESCE(a.ecer, 0), COALESCE(a.hv, 0),
		       COALESCE(i.awb, 0) - COALESCE(a.awb, 0),
		       COALESCE(i.koli, 0) - COALESCE(a.koli, 0),
		       COALESCE(i.ecer, 0) - COALESCE(a.ecer, 0),
		       COALESCE(i.hv, 0) - COALESCE(a.hv, 0),
		       COALESCE(i.w_kj, 0) - COALESCE(a.w_kj, 0),
		       COALESCE(i.w_ks, 0) - COALESCE(a.w_ks, 0),
		       COALESCE(i.w_kb, 0) - COALESCE(a.w_kb, 0),
		       COALESCE(i.w_ej, 0) - COALESCE(a.w_ej, 0),
		       COALESCE(i.w_es, 0) - COALESCE(a.w_es, 0),
		       COALESCE(i.w_eb, 0) - COALESCE(a.w_eb, 0)
		FROM inp i
		FULL OUTER JOIN amb a USING (jenis_ritase, ritase_ke)
		ORDER BY 2
	`, idGudang)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var jenis string
			var ritKe, inAwb, inKoli, inEcer, inHV, amAwb, amKoli, amEcer, amHV int
			var sAwb, sKoli, sEcer, sHV int
			var sKj, sKs, sKb, sEj, sEs, sEb int
			if err := sRows.Scan(&jenis, &ritKe,
				&inAwb, &inKoli, &inEcer, &inHV, &amAwb, &amKoli, &amEcer, &amHV,
				&sAwb, &sKoli, &sEcer, &sHV,
				&sKj, &sKs, &sKb, &sEj, &sEs, &sEb); err != nil {
				continue
			}
			sisaGrup = append(sisaGrup, map[string]interface{}{
				"jenis_ritase": jenis, "ritase_ke": ritKe,
				"input_awb": inAwb, "input_koli": inKoli, "input_ecer": inEcer, "input_hv": inHV,
				"diambil_awb": amAwb, "diambil_koli": amKoli, "diambil_ecer": amEcer, "diambil_hv": amHV,
				"sisa_awb": sAwb, "sisa_koli": sKoli, "sisa_ecer": sEcer, "sisa_hv": sHV,
				"sisa_koli_jkt": sKj, "sisa_koli_seg": sKs, "sisa_koli_btn": sKb,
				"sisa_ecer_jkt": sEj, "sisa_ecer_seg": sEs, "sisa_ecer_btn": sEb,
			})
		}
	}

	return response.OK(c, map[string]interface{}{
		"riwayat":   list,
		"sisa_grup": sisaGrup,
	})
}

// GetRitase mengambil ritase hari ini (WIB) yang punya titik muat di gudang
// outgoing dan sudah ada driver + kendaraan — kandidat pengambilan. Titik start
// Rit 1 (urutan 1) tidak dihitung sebagai titik muat.
// GET /api/v1/gudang-muatan/ritase
func (h *Handler) GetRitase(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	idGudang, _, err := h.resolveOutgoingGudang(ctx0)
	if err != nil || idGudang == 0 {
		return response.Error(c, http.StatusBadRequest, "gudang outgoing tidak ditemukan")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	hariIni := time.Now().In(wib).Format("2006-01-02")

	rows, err := h.DB.Query(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.ritase_ke, r.status,
		       r.jam_mulai, r.jam_selesai, r.jenis_ritase,
		       COALESCE(d.nama_driver, 'Driver'), COALESCE(k.plat_nomor, '-'),
		       rs.id_stop, rs.urutan
		FROM ritase r
		JOIN LATERAL (
			SELECT rs.id_stop, rs.urutan
			FROM ritase_stop rs
			WHERE rs.id_ritase = r.id_ritase
			  AND rs.id_gudang = $1
			  AND NOT (COALESCE(r.ritase_ke, 1) = 1 AND rs.urutan = 1)
			ORDER BY rs.urutan DESC
			LIMIT 1
		) rs ON TRUE
		JOIN driver d ON d.id_driver = r.id_driver
		JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.tanggal = $2
		  AND r.status != 'selesai'
		ORDER BY r.ritase_ke ASC, r.id_ritase ASC
	`, idGudang, hariIni)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data ritase")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var idRitase, idStop int64
		var kode, status, namaDriver, plat string
		var ritKe, urutan int
		var jamMulai, jamSelesai, jenis *string
		if err := rows.Scan(&idRitase, &kode, &ritKe, &status,
			&jamMulai, &jamSelesai, &jenis,
			&namaDriver, &plat, &idStop, &urutan); err != nil {
			continue
		}
		list = append(list, map[string]interface{}{
			"id_ritase": idRitase, "kode_ritase": kode,
			"ritase_ke": ritKe, "status": status,
			"jam_mulai": jamMulai, "jam_selesai": jamSelesai,
			"jenis_ritase": jenis,
			"nama_driver":  namaDriver, "plat_nomor": plat,
			"id_stop": idStop, "urutan": urutan,
		})
	}

	return response.OK(c, list)
}

// ConfirmPickupRequest adalah body konfirmasi pengambilan oleh koor gudang.
type ConfirmPickupRequest struct {
	JenisRitase string `json:"jenis_ritase"`
	RitaseKe    int    `json:"ritase_ke"`
	IDRitase    int64  `json:"id_ritase"`
	IDStop      int64  `json:"id_stop"`

	DiambilAWB       int    `json:"diambil_awb"`
	DiambilKoliJKT   int    `json:"diambil_koli_jkt"`
	DiambilKoliSEG   int    `json:"diambil_koli_seg"`
	DiambilKoliBTN   int    `json:"diambil_koli_btn"`
	DiambilEcerJKT   int    `json:"diambil_ecer_jkt"`
	DiambilEcerSEG   int    `json:"diambil_ecer_seg"`
	DiambilEcerBTN   int    `json:"diambil_ecer_btn"`
	DiambilKoliHVJKT int    `json:"diambil_koli_hv_jkt"`
	DiambilKoliHVSEG int    `json:"diambil_koli_hv_seg"`
	DiambilKoliHVBTN int    `json:"diambil_koli_hv_btn"`
	DiambilEcerHVJKT int    `json:"diambil_ecer_hv_jkt"`
	DiambilEcerHVSEG int    `json:"diambil_ecer_hv_seg"`
	DiambilEcerHVBTN int    `json:"diambil_ecer_hv_btn"`
	DiambilAwbHVJKT  int    `json:"diambil_awb_hv_jkt"`
	DiambilAwbHVSEG  int    `json:"diambil_awb_hv_seg"`
	DiambilAwbHVBTN  int    `json:"diambil_awb_hv_btn"`
	Catatan          string `json:"catatan"`
}

// PostConfirmPickup menautkan input gudang ke ritase pilihan dan mencatat realisasi.
// POST /api/v1/gudang-muatan/confirm-pickup
func (h *Handler) PostConfirmPickup(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	idGudang, _, err := h.resolveOutgoingGudang(ctx0)
	if err != nil || idGudang == 0 {
		return response.Error(c, http.StatusBadRequest, "gudang outgoing tidak ditemukan")
	}

	var req ConfirmPickupRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if req.RitaseKe < 1 || req.RitaseKe > 4 {
		return response.Error(c, http.StatusBadRequest, "ritase_ke tidak valid")
	}
	if req.IDRitase <= 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase wajib dipilih")
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	req.DiambilAWB = clamp(req.DiambilAWB)
	req.DiambilKoliJKT = clamp(req.DiambilKoliJKT)
	req.DiambilKoliSEG = clamp(req.DiambilKoliSEG)
	req.DiambilKoliBTN = clamp(req.DiambilKoliBTN)
	req.DiambilEcerJKT = clamp(req.DiambilEcerJKT)
	req.DiambilEcerSEG = clamp(req.DiambilEcerSEG)
	req.DiambilEcerBTN = clamp(req.DiambilEcerBTN)
	req.DiambilKoliHVJKT = clamp(req.DiambilKoliHVJKT)
	req.DiambilKoliHVSEG = clamp(req.DiambilKoliHVSEG)
	req.DiambilKoliHVBTN = clamp(req.DiambilKoliHVBTN)
	req.DiambilEcerHVJKT = clamp(req.DiambilEcerHVJKT)
	req.DiambilEcerHVSEG = clamp(req.DiambilEcerHVSEG)
	req.DiambilEcerHVBTN = clamp(req.DiambilEcerHVBTN)
	req.DiambilAwbHVJKT = clamp(req.DiambilAwbHVJKT)
	req.DiambilAwbHVSEG = clamp(req.DiambilAwbHVSEG)
	req.DiambilAwbHVBTN = clamp(req.DiambilAwbHVBTN)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	// 1. Validasi: ritase ada, belum selesai, punya titik muat outgoing
	// (titik start Rit 1 tidak dihitung), jenis+rit sama.
	var foundID int64
	err = tx.QueryRow(ctx, `
		SELECT r.id_ritase FROM ritase r
		WHERE r.id_ritase = $2
		  AND r.status != 'selesai'
		  AND r.jenis_ritase = $3 AND r.ritase_ke = $4
		  AND r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
		                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
		  AND EXISTS (
			SELECT 1 FROM ritase_stop rs
			WHERE rs.id_ritase = r.id_ritase
			  AND rs.id_gudang = $1
			  AND NOT (COALESCE(r.ritase_ke, 1) = 1 AND rs.urutan = 1)
			  AND ($5 = 0 OR rs.id_stop = $5)
		  )
	`, idGudang, req.IDRitase, req.JenisRitase, req.RitaseKe, req.IDStop).Scan(&foundID)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "ritase tidak valid untuk gudang ini")
	}

	// 2. Catat serah terima.
	var userID interface{}
	if uid, ok := c.Get(appMiddleware.CtxUserID).(int64); ok && uid > 0 {
		userID = uid
	}
	var idKonfirmasi int64
	err = tx.QueryRow(ctx, `
		INSERT INTO konfirmasi_penjemputan (
			id_ritase, id_gudang, id_user, jenis_ritase, ritase_ke,
			jumlah_awb,
			koli_jkt, koli_seg, koli_btn,
			ecer_jkt, ecer_seg, ecer_btn,
			koli_hv_jkt, koli_hv_seg, koli_hv_btn,
			ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			awb_hv_jkt, awb_hv_seg, awb_hv_btn,
			catatan
		) VALUES (
			$1, $2, $3, $4, $5,
			$6,
			$7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20, $21,
			$22
		) RETURNING id
	`, req.IDRitase, idGudang, userID, req.JenisRitase, req.RitaseKe,
		req.DiambilAWB,
		req.DiambilKoliJKT, req.DiambilKoliSEG, req.DiambilKoliBTN,
		req.DiambilEcerJKT, req.DiambilEcerSEG, req.DiambilEcerBTN,
		req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN,
		req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN,
		req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN,
		req.Catatan,
	).Scan(&idKonfirmasi)
	if err != nil {
		log.Printf("[Gudang] gagal insert konfirmasi: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan serah terima")
	}

	// 3. Tautkan input hari ini yang masih NULL dalam grup ini.
	tag, err := tx.Exec(ctx, `
		UPDATE input_kapten
		SET id_ritase = $1, updated_at = NOW()
		WHERE id_gudang = $2 AND jenis_ritase = $3 AND ritase_ke = $4
		  AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, req.IDRitase, idGudang, req.JenisRitase, req.RitaseKe)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menautkan input")
	}
	linked := tag.RowsAffected()

	// 4. Hitung sisa grup.
	var inAWB, inKJ, inKS, inKB, inEJ, inES, inEB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0)
		FROM input_kapten
		WHERE id_gudang = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, idGudang, req.JenisRitase, req.RitaseKe).Scan(
		&inAWB, &inKJ, &inKS, &inKB, &inEJ, &inES, &inEB)

	var tkAWB, tkKJ, tkKS, tkKB, tkEJ, tkES, tkEB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0)
		FROM konfirmasi_penjemputan
		WHERE id_gudang = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, idGudang, req.JenisRitase, req.RitaseKe).Scan(
		&tkAWB, &tkKJ, &tkKS, &tkKB, &tkEJ, &tkES, &tkEB)

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan konfirmasi")
	}

	h.bus.Publish("force_refresh", "gudang_confirm_pickup")

	return response.OK(c, map[string]interface{}{
		"id_ritase":     req.IDRitase,
		"total_linked":  linked,
		"id_konfirmasi": idKonfirmasi,
		"sisa": map[string]interface{}{
			"jumlah_awb": inAWB - tkAWB,
			"koli_jkt":   inKJ - tkKJ, "koli_seg": inKS - tkKS, "koli_btn": inKB - tkKB,
			"ecer_jkt": inEJ - tkEJ, "ecer_seg": inES - tkES, "ecer_btn": inEB - tkEB,
		},
	})
}

// RegisterRoutes memasang route muatan gudang (butuh JWT + role).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW, roleMW echo.MiddlewareFunc) {
	gm := g.Group("/gudang-muatan", authMW, roleMW)
	gm.POST("/input", h.PostMuatanInput)
	gm.GET("/pending", h.GetPending)
	gm.GET("/konfirmasi", h.GetKonfirmasi)
	gm.GET("/ritase", h.GetRitase)
	gm.POST("/confirm-pickup", h.PostConfirmPickup)
}
