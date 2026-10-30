package kapten

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"backend/internal/eventbus"
	appJWT "backend/internal/pkg/jwt"
	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

type Handler struct {
	DB  *pgxpool.Pool
	bus *eventbus.Bus
	jwt *appJWT.Manager
}

func NewHandler(db *pgxpool.Pool, bus *eventbus.Bus, jwtMgr *appJWT.Manager) *Handler {
	return &Handler{DB: db, bus: bus, jwt: jwtMgr}
}

// lokasiKapten membaca konteks lokasi kapten dari JWT: implant (baru) atau
// seller (legacy). Selalu mengembalikan keduanya â€” sisi yang kosong di-resolve
// via padanan kode (implant.kode_implant = seller.kode_seller). Bila keduanya
// nol, kapten belum memilih lokasi.
func (h *Handler) lokasiKapten(ctx context.Context, c echo.Context) (implantID, sellerID int64) {
	implantID, _ = c.Get(appMiddleware.CtxImplantID).(int64)
	sellerID, _ = c.Get(appMiddleware.CtxSellerID).(int64)
	// Token lama (dibuat sebelum pemisahan seller/implant) tidak memuat klaim
	// lokasi. Bila kapten hanya memegang satu implant aktif, pakai itu agar
	// sesi lama tetap jalan; multi-implant tetap wajib select-implant.
	if implantID == 0 && sellerID == 0 {
		if userID, _ := c.Get(appMiddleware.CtxUserID).(int64); userID > 0 {
			var count int
			var only int64
			err := h.DB.QueryRow(ctx, `
				SELECT COUNT(*), COALESCE(MIN(kim.id_implant), 0)
				FROM kapten_implant_map kim
				JOIN implant i ON i.id_implant = kim.id_implant
				WHERE kim.id_user = $1 AND i.status = 'aktif'`, userID).Scan(&count, &only)
			if err == nil && count == 1 {
				implantID = only
			}
		}
	}
	if implantID == 0 && sellerID > 0 {
		_ = h.DB.QueryRow(ctx, `
			SELECT i.id_implant FROM implant i
			JOIN seller s ON s.kode_seller = i.kode_implant
			WHERE s.id_seller = $1`, sellerID).Scan(&implantID)
	}
	if sellerID == 0 && implantID > 0 {
		_ = h.DB.QueryRow(ctx, `
			SELECT s.id_seller FROM seller s
			JOIN implant i ON i.kode_implant = s.kode_seller
			WHERE i.id_implant = $1`, implantID).Scan(&sellerID)
	}
	return implantID, sellerID
}

// namaLokasiKapten mengembalikan nama lokasi dari sisi yang tersedia.
func (h *Handler) namaLokasiKapten(ctx context.Context, implantID, sellerID int64) string {
	if implantID > 0 {
		var nama string
		if err := h.DB.QueryRow(ctx, `SELECT nama_implant FROM implant WHERE id_implant = $1`, implantID).Scan(&nama); err == nil && nama != "" {
			return nama
		}
	}
	if sellerID > 0 {
		var nama string
		_ = h.DB.QueryRow(ctx, `SELECT nama_seller FROM seller WHERE id_seller = $1`, sellerID).Scan(&nama)
		return nama
	}
	return ""
}

// GetMySellerRitase mengambil daftar ritase yang hari ini singgah di lokasi kapten.
// GET /api/v1/kapten/my-seller-ritase
func (h *Handler) GetMySellerRitase(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	// Tanggal "hari ini" harus WIB (seperti endpoint lain), bukan zona server,
	// agar filter tidak salah hari di sekitar tengah malam.
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	hariIni := time.Now().In(wib).Format("2006-01-02")

	rows, err := h.DB.Query(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.ritase_ke, r.status,
		       r.jam_mulai, r.jam_selesai, r.jenis_ritase,
		       COALESCE(r.total_koli, 0), COALESCE(r.total_awb, 0),
		       COALESCE(d.nama_driver, 'Driver'), COALESCE(k.plat_nomor, '-'),
		       rs.id_stop, rs.urutan,
		       COALESCE(rs.foto_manifest_url, '')
		FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase AND (rs.id_implant = $1 OR rs.id_seller = $2)
		JOIN driver d ON d.id_driver = r.id_driver
		JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.tanggal = $3
		  AND r.status != 'selesai'
		ORDER BY r.ritase_ke ASC, rs.urutan ASC
	`, implantID, sellerID, hariIni)
	if err != nil {
		log.Printf("[Kapten] gagal ambil ritase lokasi implant %d seller %d: %v", implantID, sellerID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data ritase")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var (
			idRitase     int64
			kodeRitase   string
			ritaseKe     int
			status       string
			jamMulai     *string
			jamSelesai   *string
			jenisRitase  *string
			totalKoli    int
			totalAWB     int
			namaDriver   string
			platNomor    string
			idStop       int64
			urutan       int
			fotoManifest string
		)
		if err := rows.Scan(&idRitase, &kodeRitase, &ritaseKe, &status,
			&jamMulai, &jamSelesai, &jenisRitase,
			&totalKoli, &totalAWB,
			&namaDriver, &platNomor,
			&idStop, &urutan, &fotoManifest); err != nil {
			// Jangan telan error diam-diam â€” baris gagal scan harus terlihat di log.
			log.Printf("[Kapten] scan my-seller-ritase error (seller %d): %v", sellerID, err)
			continue
		}

		item := map[string]interface{}{
			"id_ritase":     idRitase,
			"kode_ritase":   kodeRitase,
			"ritase_ke":     ritaseKe,
			"status":        status,
			"jam_mulai":     jamMulai,
			"jam_selesai":   jamSelesai,
			"jenis_ritase":  jenisRitase,
			"total_koli":    totalKoli,
			"total_awb":     totalAWB,
			"nama_driver":   namaDriver,
			"plat_nomor":    platNomor,
			"id_stop":       idStop,
			"urutan":        urutan,
			"foto_manifest": fotoManifest,
		}
		list = append(list, item)
	}

	if list == nil {
		list = []map[string]interface{}{}
	}

	return response.OK(c, list)
}

// KaptenCargoInputRequest adalah body untuk input cargo oleh kapten.
type KaptenCargoInputRequest struct {
	IDRitase    int64   `json:"id_ritase"`
	IDStop      int64   `json:"id_stop"`
	NamaLokasi  string  `json:"nama_lokasi"`
	JenisRitase string  `json:"jenis_ritase"` // 'outgoing' atau 'incoming'
	RitaseKe    int     `json:"ritase_ke"`    // 1, 2, 3, atau 4
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	JumlahAWB   int     `json:"jumlah_awb"`
	KoliJKT     int     `json:"koli_jkt"`
	KoliSEG     int     `json:"koli_seg"`
	KoliBTN     int     `json:"koli_btn"`
	EcerJKT     int     `json:"ecer_jkt"`
	EcerSEG     int     `json:"ecer_seg"`
	EcerBTN     int     `json:"ecer_btn"`
	KoliHVJKT   int     `json:"koli_hv_jkt"`
	KoliHVSEG   int     `json:"koli_hv_seg"`
	KoliHVBTN   int     `json:"koli_hv_btn"`
	EcerHVJKT   int     `json:"ecer_hv_jkt"`
	EcerHVSEG   int     `json:"ecer_hv_seg"`
	EcerHVBTN   int     `json:"ecer_hv_btn"`
	// AWB HV per-wilayah (kolom baru, bukan alias).
	AwbHvJKT int `json:"awb_hv_jkt"`
	AwbHvSEG int `json:"awb_hv_seg"`
	AwbHvBTN int `json:"awb_hv_btn"`
	// Opsi A (legacy, gabungan 1 field): alias menang hanya bila ketiga
	// kolom region-nya masih 0 â€” untuk kompatibilitas app lama.
	// HvAwb = label lama "HV AWB" (pengganti Ecer HV, satuan sama).
	KoliHV          int    `json:"koli_hv"`
	HvAwb           int    `json:"hv_awb"`
	EcerHV          int    `json:"ecer_hv"`
	AwbHv           int    `json:"awb_hv"`
	FotoManifestURL string `json:"foto_manifest_url"`
	Catatan         string `json:"catatan"`
}

// normalisasiHV menggabungkan alias lama (Opsi A, 1 field) + kolom per-wilayah.
// Aturan: bila ketiga kolom region masih 0 dan alias > 0 (app lama),
// nilai alias dipakai sebagai jkt. Selain itu kolom region dipakai apa adanya
// (input baru yang sudah dipecah JKT/SEG/BTN).
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

// PostCargoInput mencatat input data muatan dari kapten ke input_kapten.
// POST /api/v1/kapten/cargo-input
// Tidak perlu id_ritase â€” data disimpan independen, di-link nanti saat admin generate ritase.
func (h *Handler) PostCargoInput(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	var req KaptenCargoInputRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// Default jenis_ritase ke outgoing jika kosong
	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if req.RitaseKe == 0 {
		req.RitaseKe = 1
	}

	// Validasi waktu: pastikan sekarang dalam window dari jadwal_ritase_config
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

		startMin := mulaiMin
		endMin := selesaiMin
		if endMin <= startMin {
			endMin += 1440
		}
		checkMin := nowMin
		if checkMin < startMin {
			checkMin += 1440
		}

		diluarWindow := checkMin < startMin || checkMin > endMin
		if diluarWindow {
			log.Printf("[Kapten] input ditolak: waktu %02d:%02d di luar window %s-%s (jenis=%s ritase_ke=%d)",
				now.Hour(), now.Minute(), jamMulai[:5], jamSelesai[:5], req.JenisRitase, req.RitaseKe)
			return response.Error(c, http.StatusBadRequest,
				fmt.Sprintf("input hanya bisa dilakukan pada jam %s - %s", jamMulai[:5], jamSelesai[:5]))
		}
	}

	// Ambil info lokasi untuk nama lokasi (implant diutamakan)
	namaLokasi := h.namaLokasiKapten(ctx, implantID, sellerID)
	if req.NamaLokasi == "" {
		req.NamaLokasi = namaLokasi
	}

	// Ambil user_id dari JWT (kapten yang login)
	var userID int64
	if uid, ok := c.Get(appMiddleware.CtxUserID).(int64); ok && uid > 0 {
		userID = uid
	}

	// Titik penjemputan mengikuti mapping admin; fallback ke implant sendiri.
	titikID := implantID
	if userID > 0 && implantID > 0 {
		titikID, _ = h.implantPointForUser(ctx, h.DB, userID, implantID)
	}

	var fotoURL interface{}
	if req.FotoManifestURL != "" {
		fotoURL = req.FotoManifestURL
	}

	var catatan interface{}
	if req.Catatan != "" {
		catatan = req.Catatan
	}

	// id_ritase SELALU NULL saat input â€” penautan ke ritase hanya lewat
	// konfirmasi manual kapten (POST /kapten/confirm-pickup). Tidak ada auto-link.
	var idRitase *int64 = nil

	// HV per-wilayah (JKT/SEG/BTN): Koli HV, Ecer HV, AWB HV.
	// Alias lama (Opsi A) hanya jadi fallback bila ketiga region masih 0.
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

	// INSERT ke input_kapten (dual-write: id_implant sumber baru, id_seller legacy)
	var insertedID int64
	var colImplant, colSeller, colTitik interface{}
	if implantID > 0 {
		colImplant = implantID
	}
	if sellerID > 0 {
		colSeller = sellerID
	}
	if titikID > 0 {
		colTitik = titikID
	}
	err := h.DB.QueryRow(ctx, `
		INSERT INTO input_kapten (
			id_user, id_seller, id_implant, jenis_ritase, ritase_ke, catatan,
			jumlah_awb, koli_jkt, koli_seg, koli_btn,
			ecer_jkt, ecer_seg, ecer_btn,
			koli_hv_jkt, koli_hv_seg, koli_hv_btn,
			ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			awb_hv_jkt, awb_hv_seg, awb_hv_btn,
			foto_manifest_url, nama_lokasi, latitude, longitude,
			id_ritase, id_titik_implant
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22,
			$23, $24, $25, $26, $27, $28
		) RETURNING id
	`, userID, colSeller, colImplant, req.JenisRitase, req.RitaseKe, catatan,
		req.JumlahAWB, req.KoliJKT, req.KoliSEG, req.KoliBTN,
		req.EcerJKT, req.EcerSEG, req.EcerBTN,
		hvKoliJkt, hvKoliSeg, hvKoliBtn,
		hvEcerJkt, hvEcerSeg, hvEcerBtn,
		hvAwbJkt, hvAwbSeg, hvAwbBtn,
		fotoURL, req.NamaLokasi, req.Latitude, req.Longitude,
		idRitase, colTitik,
	).Scan(&insertedID)

	if err != nil {
		log.Printf("[Kapten] gagal insert input_kapten: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan data muatan")
	}

	// Hitung total akumulasi untuk lokasi ini hari ini
	var locAwb, locKoli, locEcer, locHV, locAwbHv int
	_ = h.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn + awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0)
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND jenis_ritase = $3 AND ritase_ke = $4
		  AND DATE(created_at) = CURRENT_DATE
	`, implantID, sellerID, req.JenisRitase, req.RitaseKe).Scan(&locAwb, &locKoli, &locEcer, &locHV, &locAwbHv)

	h.bus.Publish("force_refresh", "kapten_cargo_input")

	return response.Created(c, map[string]interface{}{
		"id":                  insertedID,
		"total_awb_lokasi":    locAwb,
		"total_koli_lokasi":   locKoli,
		"total_ecer_lokasi":   locEcer,
		"total_hv_lokasi":     locHV,
		"total_awb_hv_lokasi": locAwbHv,
	})
}

// GetSellerInfo mengambil info seller yang terkait dengan kapten (legacy).
// GET /api/v1/kapten/seller-info
func (h *Handler) GetSellerInfo(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var (
		namaSeller string
		alamat     string
		pic        string
		noHP       string
		isImplant  bool
	)
	err := h.DB.QueryRow(ctx, `
		SELECT nama_seller, COALESCE(alamat, ''), COALESCE(pic, ''), COALESCE(no_hp, ''),
		       COALESCE(is_implant, FALSE)
		FROM seller WHERE id_seller = $1
	`, sellerID).Scan(&namaSeller, &alamat, &pic, &noHP, &isImplant)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "seller tidak ditemukan")
	}

	return response.OK(c, map[string]interface{}{
		"id_seller":   sellerID,
		"nama_seller": namaSeller,
		"alamat":      alamat,
		"pic":         pic,
		"no_hp":       noHP,
		"is_implant":  isImplant,
	})
}

// GetImplantInfo mengambil info implant yang terkait dengan kapten.
// GET /api/v1/kapten/implant-info
func (h *Handler) GetImplantInfo(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, _ := h.lokasiKapten(ctx0, c)
	if implantID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var (
		namaImplant string
		alamat      string
		kota        string
		noHP        string
		kapten      string
	)
	err := h.DB.QueryRow(ctx, `
		SELECT i.nama_implant, COALESCE(i.alamat, ''), COALESCE(i.kota, ''), COALESCE(i.no_hp, ''),
		       COALESCE((SELECT STRING_AGG(k.nama, ', ' ORDER BY CASE WHEN kim.peran = 'utama' THEN 0 ELSE 1 END, k.nama)
		                 FROM kapten_implant_map kim JOIN kapten k ON k.id_user = kim.id_user
		                 WHERE kim.id_implant = i.id_implant), '')
		FROM implant WHERE id_implant = $1
	`, implantID).Scan(&namaImplant, &alamat, &kota, &noHP, &kapten)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "implant tidak ditemukan")
	}

	return response.OK(c, map[string]interface{}{
		"id_implant":   implantID,
		"nama_implant": namaImplant,
		"alamat":       alamat,
		"kota":         kota,
		"no_hp":        noHP,
		"kapten":       kapten,
	})
}

// GetTodayCargo mengambil total akumulasi cargo kapten hari ini dari input_kapten.
// GET /api/v1/kapten/today-cargo?jenis_ritase=outgoing&ritase_ke=1
func (h *Handler) GetTodayCargo(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	jenisRitase := c.QueryParam("jenis_ritase")
	if jenisRitase == "" {
		jenisRitase = "outgoing"
	}
	ritaseKe := 0
	fmt.Sscanf(c.QueryParam("ritase_ke"), "%d", &ritaseKe)
	if ritaseKe == 0 {
		ritaseKe = 1
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// Ambil nama lokasi (implant diutamakan)
	namaLokasi := h.namaLokasiKapten(ctx, implantID, sellerID)

	// Query dari input_kapten (bukan ritase_event)
	var awb, koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
	var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
	var awbHvJkt, awbHvSeg, awbHvBtn int
	var fotoURL string
	var catatan string
	var isAda bool
	err := h.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0),
		       COALESCE((SELECT foto_manifest_url FROM input_kapten
		                 WHERE (id_implant = $1 OR id_seller = $2) AND jenis_ritase = $3 AND ritase_ke = $4
		                   AND DATE(created_at) = CURRENT_DATE AND foto_manifest_url IS NOT NULL
		                 ORDER BY id DESC LIMIT 1), ''),
		       COALESCE((SELECT catatan FROM input_kapten
		                 WHERE (id_implant = $1 OR id_seller = $2) AND jenis_ritase = $3 AND ritase_ke = $4
		                   AND DATE(created_at) = CURRENT_DATE AND catatan IS NOT NULL AND catatan != ''
		                 ORDER BY id DESC LIMIT 1), '')
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND jenis_ritase = $3 AND ritase_ke = $4
		  AND DATE(created_at) = CURRENT_DATE
	`, implantID, sellerID, jenisRitase, ritaseKe).Scan(&awb, &koliJkt, &koliSeg, &koliBtn,
		&ecerJkt, &ecerSeg, &ecerBtn,
		&koliHvJkt, &koliHvSeg, &koliHvBtn,
		&ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
		&awbHvJkt, &awbHvSeg, &awbHvBtn, &fotoURL, &catatan)

	totalKoli := koliJkt + koliSeg + koliBtn
	totalEcer := ecerJkt + ecerSeg + ecerBtn
	totalHV := koliHvJkt + koliHvSeg + koliHvBtn + ecerHvJkt + ecerHvSeg + ecerHvBtn + awbHvJkt + awbHvSeg + awbHvBtn
	if err == nil && (totalKoli > 0 || totalEcer > 0 || totalHV > 0 || awb > 0) {
		isAda = true
	}

	return response.OK(c, map[string]interface{}{
		"nama_lokasi": namaLokasi,
		"jumlah_awb":  awb,
		"koli_jkt":    koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
		"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
		"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
		"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
		"awb_hv_jkt": awbHvJkt, "awb_hv_seg": awbHvSeg, "awb_hv_btn": awbHvBtn,
		"koli_hv":     koliHvJkt + koliHvSeg + koliHvBtn,
		"hv_awb":      ecerHvJkt + ecerHvSeg + ecerHvBtn,
		"ecer_hv":     ecerHvJkt + ecerHvSeg + ecerHvBtn,
		"awb_hv":      awbHvJkt + awbHvSeg + awbHvBtn,
		"foto_url":    fotoURL,
		"catatan":     catatan,
		"is_ada_data": isAda,
	})
}

// GetMySellers mengambil daftar seller yang bisa diakses kapten (dari kapten_seller_map).
// GET /api/v1/kapten/my-sellers
func (h *Handler) GetMySellers(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT s.id_seller, COALESCE(s.kode_seller, ''), COALESCE(s.nama_seller, '')
		FROM seller s
		JOIN kapten_seller_map ksm ON ksm.id_seller = s.id_seller
		WHERE ksm.id_user = $1 AND s.status = 'aktif'
		ORDER BY s.nama_seller
	`, userID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil my-sellers untuk user %d: %v", userID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil daftar seller")
	}
	defer rows.Close()

	var sellers []map[string]interface{}
	for rows.Next() {
		var (
			idSeller   int64
			kodeSeller string
			namaSeller string
		)
		if err := rows.Scan(&idSeller, &kodeSeller, &namaSeller); err != nil {
			continue
		}
		sellers = append(sellers, map[string]interface{}{
			"id_seller":   idSeller,
			"kode_seller": kodeSeller,
			"nama_seller": namaSeller,
		})
	}

	if sellers == nil {
		sellers = []map[string]interface{}{}
	}

	return response.OK(c, map[string]interface{}{
		"sellers": sellers,
	})
}

// GetMyImplants mengambil daftar implant yang bisa diakses kapten (dari kapten_implant_map).
// GET /api/v1/kapten/my-implants
func (h *Handler) GetMyImplants(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT i.id_implant, COALESCE(i.kode_implant, ''), COALESCE(i.nama_implant, ''),
		       COALESCE(kim.peran, 'utama'),
		       COALESCE(kim.id_titik_implant, i.id_implant),
		       COALESCE(ht.nama_implant, i.nama_implant)
		FROM implant i
		JOIN kapten_implant_map kim ON kim.id_implant = i.id_implant
		LEFT JOIN implant ht ON ht.id_implant = COALESCE(kim.id_titik_implant, i.id_implant)
		WHERE kim.id_user = $1 AND i.status = 'aktif'
		ORDER BY COALESCE(ht.nama_implant, i.nama_implant), i.nama_implant
	`, userID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil my-implants untuk user %d: %v", userID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil daftar implant")
	}
	defer rows.Close()

	var implants []map[string]interface{}
	for rows.Next() {
		var (
			idImplant   int64
			kodeImplant string
			namaImplant string
			peran       string
			idTitik     int64
			namaTitik   string
		)
		if err := rows.Scan(&idImplant, &kodeImplant, &namaImplant, &peran, &idTitik, &namaTitik); err != nil {
			continue
		}
		implants = append(implants, map[string]interface{}{
			"id_implant":         idImplant,
			"kode_implant":       kodeImplant,
			"nama_implant":       namaImplant,
			"peran":              peran,
			"id_titik_implant":   idTitik,
			"nama_titik_implant": namaTitik,
		})
	}

	if implants == nil {
		implants = []map[string]interface{}{}
	}

	return response.OK(c, map[string]interface{}{
		"implants": implants,
	})
}

// SelectSellerRequest adalah body untuk select seller.
type SelectSellerRequest struct {
	SellerID int64 `json:"seller_id"`
}

// SelectSeller memilih seller â†’ return JWT baru dengan seller_id yang dipilih.
// POST /api/v1/kapten/select-seller
func (h *Handler) SelectSeller(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	username, _ := c.Get(appMiddleware.CtxUsername).(string)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}

	var req SelectSellerRequest
	if err := c.Bind(&req); err != nil || req.SellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "seller_id wajib diisi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// Validasi: user punya akses ke seller ini?
	var count int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM kapten_seller_map
		WHERE id_user = $1 AND id_seller = $2
	`, userID, req.SellerID).Scan(&count)
	if count == 0 {
		return response.Error(c, http.StatusForbidden, "anda tidak memiliki akses ke seller ini")
	}

	// Generate JWT baru dengan seller_id yang dipilih
	token, err := h.jwt.Generate(userID, username, "kapten", 0, req.SellerID)
	if err != nil {
		log.Printf("[Kapten] gagal generate token untuk user %d: %v", userID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal membuat token")
	}

	return response.OK(c, map[string]interface{}{
		"token":     token,
		"seller_id": req.SellerID,
	})
}

// SelectImplantRequest adalah body untuk select implant.
type SelectImplantRequest struct {
	ImplantID int64 `json:"implant_id"`
}

// SelectImplant memilih implant â†’ return JWT baru dengan id_implant (+id_seller
// padanan untuk kompatibilitas) yang dipilih.
// POST /api/v1/kapten/select-implant
func (h *Handler) SelectImplant(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	username, _ := c.Get(appMiddleware.CtxUsername).(string)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}

	var req SelectImplantRequest
	if err := c.Bind(&req); err != nil || req.ImplantID == 0 {
		return response.Error(c, http.StatusBadRequest, "implant_id wajib diisi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// Validasi: user punya akses ke implant ini?
	var count int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM kapten_implant_map
		WHERE id_user = $1 AND id_implant = $2
	`, userID, req.ImplantID).Scan(&count)
	if count == 0 {
		return response.Error(c, http.StatusForbidden, "anda tidak memiliki akses ke implant ini")
	}

	// Padanan seller legacy untuk kompatibilitas (boleh 0 bila tak ada).
	var sellerID int64
	_ = h.DB.QueryRow(ctx, `
		SELECT s.id_seller FROM seller s
		JOIN implant i ON i.kode_implant = s.kode_seller
		WHERE i.id_implant = $1`, req.ImplantID).Scan(&sellerID)

	// Generate JWT baru dengan id_implant (+id_seller padanan) yang dipilih
	token, err := h.jwt.Generate(userID, username, "kapten", 0, sellerID, req.ImplantID)
	if err != nil {
		log.Printf("[Kapten] gagal generate token implant untuk user %d: %v", userID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal membuat token")
	}

	return response.OK(c, map[string]interface{}{
		"token":      token,
		"implant_id": req.ImplantID,
		"seller_id":  sellerID,
	})
}

// ConfirmPickupRequest adalah body konfirmasi driver pengambil oleh kapten.
// Satu konfirmasi menautkan SEMUA input hari ini yang masih NULL dalam grup
// (seller kapten + jenis_ritase + ritase_ke) ke satu ritase (trip) SEKALIGUS
// mencatat berapa yang diambil driver (diambil_*), foto bukti, dan catatan.
// Boleh dikonfirmasi berkali-kali (ambil bertahap) â€” sisa = input âˆ’ akumulasi diambil.
type ConfirmPickupRequest struct {
	JenisRitase string `json:"jenis_ritase"` // 'outgoing' (default) atau 'incoming'
	RitaseKe    int    `json:"ritase_ke"`    // 1, 2, atau 3
	IDRitase    int64  `json:"id_ritase"`    // trip yang dipilih kapten

	DiambilAWB       int `json:"diambil_awb"`
	DiambilKoliJKT   int `json:"diambil_koli_jkt"`
	DiambilKoliSEG   int `json:"diambil_koli_seg"`
	DiambilKoliBTN   int `json:"diambil_koli_btn"`
	DiambilEcerJKT   int `json:"diambil_ecer_jkt"`
	DiambilEcerSEG   int `json:"diambil_ecer_seg"`
	DiambilEcerBTN   int `json:"diambil_ecer_btn"`
	DiambilKoliHVJKT int `json:"diambil_koli_hv_jkt"`
	DiambilKoliHVSEG int `json:"diambil_koli_hv_seg"`
	DiambilKoliHVBTN int `json:"diambil_koli_hv_btn"`
	DiambilEcerHVJKT int `json:"diambil_ecer_hv_jkt"`
	DiambilEcerHVSEG int `json:"diambil_ecer_hv_seg"`
	DiambilEcerHVBTN int `json:"diambil_ecer_hv_btn"`
	DiambilAwbHVJKT  int `json:"diambil_awb_hv_jkt"`
	DiambilAwbHVSEG  int `json:"diambil_awb_hv_seg"`
	DiambilAwbHVBTN  int `json:"diambil_awb_hv_btn"`
	// Alias lama (Opsi A, gabungan): menang hanya bila ketiga region masih 0.
	DiambilKoliHV int `json:"diambil_koli_hv"`
	DiambilHvAwb  int `json:"diambil_hv_awb"`
	DiambilEcerHV int `json:"diambil_ecer_hv"`
	DiambilAwbHv  int `json:"diambil_awb_hv"`

	FotoPenjemputanURL string `json:"foto_penjemputan_url"` // opsional
	Catatan            string `json:"catatan"`              // opsional (mis. barang sisa)
}

// ConfirmPickup menautkan input kapten yang belum ter-link ke ritase pilihan kapten
// dan mencatat realisasi pengambilan (jumlah + foto + catatan).
// POST /api/v1/kapten/confirm-pickup
func (h *Handler) ConfirmPickup(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
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
	// Pertahanan: angka negatif dijepit ke 0.
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
	// Alias lama menang hanya bila ketiga region masih 0 (kompatibilitas app lama).
	req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN =
		normalisasiHV(req.DiambilKoliHV, 0, req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN)
	req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN =
		normalisasiHV(req.DiambilHvAwb, req.DiambilEcerHV, req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN)
	req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN =
		normalisasiHV(req.DiambilAwbHv, 0, req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	uid, _ := c.Get(appMiddleware.CtxUserID).(int64)
	pointID := implantID
	if uid > 0 && implantID > 0 {
		pointID, _ = h.implantPointForUser(ctx, tx, uid, implantID)
	}

	// 1. Validasi: ritase harus ada, belum selesai, singgah di lokasi kapten,
	//    dan jenis + ritase_ke-nya sama dengan grup yang dikonfirmasi.
	var foundID int64
	err = tx.QueryRow(ctx, `
		SELECT r.id_ritase FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
		                   AND (rs.id_implant = $1 OR rs.id_implant = $2 OR rs.id_seller = $3)
		WHERE r.id_ritase = $4
		  AND r.status != 'selesai'
		  AND r.jenis_ritase = $5 AND r.ritase_ke = $6
		  AND r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
		                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
	`, implantID, pointID, sellerID, req.IDRitase, req.JenisRitase, req.RitaseKe).Scan(&foundID)
	if err != nil {
		log.Printf("[Kapten] konfirmasi ditolak implant %d seller %d -> ritase %d: %v", implantID, sellerID, req.IDRitase, err)
		return response.Error(c, http.StatusBadRequest, "ritase tidak valid untuk lokasi ini (bukan jadwal yang singgah ke sini)")
	}

	// 2. Catat kejadian serah terima (realisasi pengambilan).
	var userID interface{}
	if uid > 0 {
		userID = uid
	}
	var colTitik interface{}
	if pointID > 0 {
		colTitik = pointID
	}
	var fotoURL interface{}
	if req.FotoPenjemputanURL != "" {
		fotoURL = req.FotoPenjemputanURL
	}
	var catatan interface{}
	if req.Catatan != "" {
		catatan = req.Catatan
	}
	var idKonfirmasi int64
	var colImplant, colSeller interface{}
	if implantID > 0 {
		colImplant = implantID
	}
	if sellerID > 0 {
		colSeller = sellerID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO konfirmasi_penjemputan (
			id_ritase, id_seller, id_implant, id_user, jenis_ritase, ritase_ke,
			jumlah_awb,
			koli_jkt, koli_seg, koli_btn,
			ecer_jkt, ecer_seg, ecer_btn,
			koli_hv_jkt, koli_hv_seg, koli_hv_btn,
			ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			awb_hv_jkt, awb_hv_seg, awb_hv_btn,
			foto_penjemputan_url, catatan, id_titik_implant
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22,
			$23, $24, $25
		) RETURNING id
	`, req.IDRitase, colSeller, colImplant, userID, req.JenisRitase, req.RitaseKe,
		req.DiambilAWB,
		req.DiambilKoliJKT, req.DiambilKoliSEG, req.DiambilKoliBTN,
		req.DiambilEcerJKT, req.DiambilEcerSEG, req.DiambilEcerBTN,
		req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN,
		req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN,
		req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN,
		fotoURL, catatan, colTitik,
	).Scan(&idKonfirmasi)
	if err != nil {
		log.Printf("[Kapten] gagal insert konfirmasi_penjemputan: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan serah terima")
	}

	// 3. Tautkan semua input hari ini (WIB) yang masih NULL dalam grup ini.
	//    Boleh 0 baris (pengambilan susulan) â€” bukan error.
	tag, err := tx.Exec(ctx, `
		UPDATE input_kapten
		SET id_ritase = $1, updated_at = NOW()
		WHERE (id_implant = $2 OR id_seller = $3) AND jenis_ritase = $4 AND ritase_ke = $5
		  AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, req.IDRitase, implantID, sellerID, req.JenisRitase, req.RitaseKe)
	if err != nil {
		log.Printf("[Kapten] gagal link input saat confirm-pickup: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menautkan input")
	}
	linked := tag.RowsAffected()

	// 4. Hitung sisa grup hari ini (WIB): Î£ input âˆ’ Î£ diambil (akumulasi semua
	//    kejadian hari ini dalam grup, boleh negatif = selisih lebih ambil).
	var inAWB, inKJ, inKS, inKB, inEJ, inES, inEB int
	var inKHJ, inKHS, inKHB, inEHJ, inEHS, inEHB int
	var inAHJ, inAHS, inAHB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0)
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND jenis_ritase = $3 AND ritase_ke = $4
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, implantID, sellerID, req.JenisRitase, req.RitaseKe).Scan(
		&inAWB, &inKJ, &inKS, &inKB, &inEJ, &inES, &inEB,
		&inKHJ, &inKHS, &inKHB, &inEHJ, &inEHS, &inEHB, &inAHJ, &inAHS, &inAHB)

	var tkAWB, tkKJ, tkKS, tkKB, tkEJ, tkES, tkEB int
	var tkKHJ, tkKHS, tkKHB, tkEHJ, tkEHS, tkEHB int
	var tkAHJ, tkAHS, tkAHB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0)
		FROM konfirmasi_penjemputan t
		WHERE (t.id_implant = $1 OR t.id_seller = $2) AND t.jenis_ritase = $3 AND t.ritase_ke = $4
		  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		  AND EXISTS (
			SELECT 1 FROM input_kapten ik
			WHERE ik.id_ritase = t.id_ritase
			  AND ik.jenis_ritase = t.jenis_ritase
			  AND ik.ritase_ke = t.ritase_ke
			  AND (ik.id_implant = t.id_implant OR ik.id_seller = t.id_seller)
			  AND (ik.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		  )
	`, implantID, sellerID, req.JenisRitase, req.RitaseKe).Scan(
		&tkAWB, &tkKJ, &tkKS, &tkKB, &tkEJ, &tkES, &tkEB,
		&tkKHJ, &tkKHS, &tkKHB, &tkEHJ, &tkEHS, &tkEHB, &tkAHJ, &tkAHS, &tkAHB)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("[Kapten] gagal commit confirm-pickup: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan konfirmasi")
	}

	sisaAWB := inAWB - tkAWB
	sisaKJ, sisaKS, sisaKB := inKJ-tkKJ, inKS-tkKS, inKB-tkKB
	sisaEJ, sisaES, sisaEB := inEJ-tkEJ, inES-tkES, inEB-tkEB
	sisaKHJ, sisaKHS, sisaKHB := inKHJ-tkKHJ, inKHS-tkKHS, inKHB-tkKHB
	sisaEHJ, sisaEHS, sisaEHB := inEHJ-tkEHJ, inEHS-tkEHS, inEHB-tkEHB
	sisaAHJ, sisaAHS, sisaAHB := inAHJ-tkAHJ, inAHS-tkAHS, inAHB-tkAHB
	log.Printf("[Kapten] seller %d konfirmasi ritase %d (%s rit %d): %d baris ter-link, sisa koli=%d",
		sellerID, req.IDRitase, req.JenisRitase, req.RitaseKe, linked,
		sisaKJ+sisaKS+sisaKB)

	h.bus.Publish("force_refresh", "kapten_confirm_pickup")

	return response.OK(c, map[string]interface{}{
		"id_ritase":     req.IDRitase,
		"total_linked":  linked,
		"id_konfirmasi": idKonfirmasi,
		"sisa": map[string]interface{}{
			"jumlah_awb": sisaAWB,
			"koli_jkt":   sisaKJ, "koli_seg": sisaKS, "koli_btn": sisaKB,
			"ecer_jkt": sisaEJ, "ecer_seg": sisaES, "ecer_btn": sisaEB,
			"koli_hv_jkt": sisaKHJ, "koli_hv_seg": sisaKHS, "koli_hv_btn": sisaKHB,
			"ecer_hv_jkt": sisaEHJ, "ecer_hv_seg": sisaEHS, "ecer_hv_btn": sisaEHB,
			"awb_hv_jkt": sisaAHJ, "awb_hv_seg": sisaAHS, "awb_hv_btn": sisaAHB,
			"koli_hv":    sisaKHJ + sisaKHS + sisaKHB,
			"hv_awb":     sisaEHJ + sisaEHS + sisaEHB,
			"ecer_hv":    sisaEHJ + sisaEHS + sisaEHB,
			"awb_hv":     sisaAHJ + sisaAHS + sisaAHB,
			"total_koli": sisaKJ + sisaKS + sisaKB,
			"total_ecer": sisaEJ + sisaES + sisaEB,
			"total_hv":   sisaKHJ + sisaKHS + sisaKHB + sisaEHJ + sisaEHS + sisaEHB + sisaAHJ + sisaAHS + sisaAHB,
		},
	})
}

// GetPendingConfirmations mengambil grup input hari ini (WIB) milik lokasi kapten
// yang belum ter-link ke ritase â€” bahan layar "Konfirmasi Pengambilan".
// GET /api/v1/kapten/pending-confirmations
func (h *Handler) GetPendingConfirmations(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT jenis_ritase, ritase_ke, COUNT(*),
		       COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn + awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0),
		       MAX(created_at)
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		GROUP BY jenis_ritase, ritase_ke
		ORDER BY ritase_ke ASC
	`, implantID, sellerID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil pending implant %d seller %d: %v", implantID, sellerID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data pending")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var jenis string
		var ritKe, count int
		var awb, koli, ecer, hv int
		var koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
		var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
		var awbHvJkt, awbHvSeg, awbHvBtn int
		var lastAt time.Time
		if err := rows.Scan(&jenis, &ritKe, &count, &awb, &koli, &ecer, &hv,
			&koliJkt, &koliSeg, &koliBtn, &ecerJkt, &ecerSeg, &ecerBtn,
			&koliHvJkt, &koliHvSeg, &koliHvBtn, &ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
			&awbHvJkt, &awbHvSeg, &awbHvBtn,
			&lastAt); err != nil {
			log.Printf("[Kapten] scan pending error (seller %d): %v", sellerID, err)
			continue
		}
		list = append(list, map[string]interface{}{
			"jenis_ritase": jenis,
			"ritase_ke":    ritKe,
			"jumlah_input": count,
			"total_awb":    awb,
			"total_koli":   koli,
			"total_ecer":   ecer,
			"total_hv":     hv,
			"koli_jkt":     koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
			"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
			"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
			"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
			"awb_hv_jkt": awbHvJkt, "awb_hv_seg": awbHvSeg, "awb_hv_btn": awbHvBtn,
		})
	}

	return response.OK(c, list)
}

// GetKonfirmasiPenjemputan mengambil riwayat serah terima hari ini (WIB) milik
// lokasi kapten â€” lengkap dengan trip, driver, jumlah diambil, foto, catatan.
// GET /api/v1/kapten/konfirmasi-penjemputan
func (h *Handler) GetKonfirmasiPenjemputan(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT kp.id, kp.id_ritase, kp.jenis_ritase, kp.ritase_ke,
		       COALESCE(kp.jumlah_awb, 0),
		       COALESCE(kp.koli_jkt, 0), COALESCE(kp.koli_seg, 0), COALESCE(kp.koli_btn, 0),
		       COALESCE(kp.ecer_jkt, 0), COALESCE(kp.ecer_seg, 0), COALESCE(kp.ecer_btn, 0),
		       COALESCE(kp.koli_hv_jkt, 0), COALESCE(kp.koli_hv_seg, 0), COALESCE(kp.koli_hv_btn, 0),
		       COALESCE(kp.ecer_hv_jkt, 0), COALESCE(kp.ecer_hv_seg, 0), COALESCE(kp.ecer_hv_btn, 0),
		       COALESCE(kp.awb_hv_jkt, 0), COALESCE(kp.awb_hv_seg, 0), COALESCE(kp.awb_hv_btn, 0),
		       COALESCE(kp.foto_penjemputan_url, ''),
		       COALESCE(kp.catatan, ''),
		       kp.created_at,
		       COALESCE(r.kode_ritase, ''),
		       COALESCE(d.nama_driver, '')
		FROM konfirmasi_penjemputan kp
		LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		WHERE (kp.id_implant = $1 OR kp.id_seller = $2)
		  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		ORDER BY kp.created_at DESC
	`, implantID, sellerID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil riwayat penjemputan implant %d seller %d: %v", implantID, sellerID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id, idRitase int64
		var jenis string
		var ritKe int
		var awb, kj, ks, kb, ej, es, eb, khj, khs, khb, ehj, ehs, ehb, ahj, ahs, ahb int
		var foto, catatan, kode, driver string
		var createdAt time.Time
		if err := rows.Scan(&id, &idRitase, &jenis, &ritKe,
			&awb, &kj, &ks, &kb, &ej, &es, &eb, &khj, &khs, &khb, &ehj, &ehs, &ehb, &ahj, &ahs, &ahb,
			&foto, &catatan, &createdAt, &kode, &driver); err != nil {
			log.Printf("[Kapten] scan riwayat penjemputan error (seller %d): %v", sellerID, err)
			continue
		}
		tKoli := kj + ks + kb
		tEcer := ej + es + eb
		tHV := khj + khs + khb + ehj + ehs + ehb + ahj + ahs + ahb
		list = append(list, map[string]interface{}{
			"id": id, "id_ritase": idRitase,
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"jumlah_awb": awb,
			"koli_jkt":   kj, "koli_seg": ks, "koli_btn": kb,
			"ecer_jkt": ej, "ecer_seg": es, "ecer_btn": eb,
			"koli_hv_jkt": khj, "koli_hv_seg": khs, "koli_hv_btn": khb,
			"ecer_hv_jkt": ehj, "ecer_hv_seg": ehs, "ecer_hv_btn": ehb,
			"awb_hv_jkt": ahj, "awb_hv_seg": ahs, "awb_hv_btn": ahb,
			"awb_hv":     ahj + ahs + ahb,
			"total_koli": tKoli, "total_ecer": tEcer, "total_hv": tHV,
			"foto_penjemputan_url": foto,
			"catatan":              catatan,
			"created_at":           createdAt.Format(time.RFC3339),
			"kode_ritase":          kode,
			"nama_driver":          driver,
		})
	}

	// Sisa per grup hari ini (WIB): Î£ input âˆ’ Î£ diambil (ringkas + per wilayah).
	sisaGrup := []map[string]interface{}{}
	sRows, err := h.DB.Query(ctx, `
		WITH inp AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb,
			       COALESCE(SUM(t.awb_hv_jkt), 0) AS w_ahj, COALESCE(SUM(t.awb_hv_seg), 0) AS w_ahs, COALESCE(SUM(t.awb_hv_btn), 0) AS w_ahb
			FROM input_kapten t
			WHERE (t.id_implant = $1 OR t.id_seller = $2)
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		),
		amb AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb,
			       COALESCE(SUM(t.awb_hv_jkt), 0) AS w_ahj, COALESCE(SUM(t.awb_hv_seg), 0) AS w_ahs, COALESCE(SUM(t.awb_hv_btn), 0) AS w_ahb
			FROM konfirmasi_penjemputan t
			WHERE (t.id_implant = $1 OR t.id_seller = $2)
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		)
		SELECT COALESCE(i.jenis_ritase, a.jenis_ritase, ''),
		       COALESCE(i.ritase_ke, a.ritase_ke, 0),
		       COALESCE(i.awb, 0), COALESCE(i.koli, 0), COALESCE(i.ecer, 0), COALESCE(i.hv, 0),
		       COALESCE(a.awb, 0), COALESCE(a.koli, 0), COALESCE(a.ecer, 0), COALESCE(a.hv, 0),
		       COALESCE(i.w_awb, 0) - COALESCE(a.w_awb, 0),
		       COALESCE(i.w_kj, 0) - COALESCE(a.w_kj, 0),
		       COALESCE(i.w_ks, 0) - COALESCE(a.w_ks, 0),
		       COALESCE(i.w_kb, 0) - COALESCE(a.w_kb, 0),
		       COALESCE(i.w_ej, 0) - COALESCE(a.w_ej, 0),
		       COALESCE(i.w_es, 0) - COALESCE(a.w_es, 0),
		       COALESCE(i.w_eb, 0) - COALESCE(a.w_eb, 0),
		       COALESCE(i.w_khj, 0) - COALESCE(a.w_khj, 0),
		       COALESCE(i.w_khs, 0) - COALESCE(a.w_khs, 0),
		       COALESCE(i.w_khb, 0) - COALESCE(a.w_khb, 0),
		       COALESCE(i.w_ehj, 0) - COALESCE(a.w_ehj, 0),
		       COALESCE(i.w_ehs, 0) - COALESCE(a.w_ehs, 0),
		       COALESCE(i.w_ehb, 0) - COALESCE(a.w_ehb, 0),
		       COALESCE(i.w_ahj, 0) - COALESCE(a.w_ahj, 0),
		       COALESCE(i.w_ahs, 0) - COALESCE(a.w_ahs, 0),
		       COALESCE(i.w_ahb, 0) - COALESCE(a.w_ahb, 0)
		FROM inp i
		FULL OUTER JOIN amb a USING (jenis_ritase, ritase_ke)
		ORDER BY 2
	`, implantID, sellerID)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var jenis string
			var ritKe int
			var inAwb, inKoli, inEcer, inHV, amAwb, amKoli, amEcer, amHV int
			var sAwb, sKj, sKs, sKb, sEj, sEs, sEb int
			var sKhj, sKhs, sKhb, sEhj, sEhs, sEhb int
			var sAhj, sAhs, sAhb int
			if err := sRows.Scan(&jenis, &ritKe,
				&inAwb, &inKoli, &inEcer, &inHV, &amAwb, &amKoli, &amEcer, &amHV,
				&sAwb, &sKj, &sKs, &sKb, &sEj, &sEs, &sEb,
				&sKhj, &sKhs, &sKhb, &sEhj, &sEhs, &sEhb, &sAhj, &sAhs, &sAhb); err != nil {
				log.Printf("[Kapten] scan sisa grup error (seller %d): %v", sellerID, err)
				continue
			}
			sisaGrup = append(sisaGrup, map[string]interface{}{
				"jenis_ritase": jenis, "ritase_ke": ritKe,
				"input_awb": inAwb, "input_koli": inKoli, "input_ecer": inEcer, "input_hv": inHV,
				"diambil_awb": amAwb, "diambil_koli": amKoli, "diambil_ecer": amEcer, "diambil_hv": amHV,
				"sisa_awb": inAwb - amAwb, "sisa_koli": inKoli - amKoli,
				"sisa_ecer": inEcer - amEcer, "sisa_hv": inHV - amHV,
				"sisa_jumlah_awb": sAwb,
				"sisa_koli_jkt":   sKj, "sisa_koli_seg": sKs, "sisa_koli_btn": sKb,
				"sisa_ecer_jkt": sEj, "sisa_ecer_seg": sEs, "sisa_ecer_btn": sEb,
				"sisa_koli_hv_jkt": sKhj, "sisa_koli_hv_seg": sKhs, "sisa_koli_hv_btn": sKhb,
				"sisa_ecer_hv_jkt": sEhj, "sisa_ecer_hv_seg": sEhs, "sisa_ecer_hv_btn": sEhb,
				"sisa_awb_hv_jkt": sAhj, "sisa_awb_hv_seg": sAhs, "sisa_awb_hv_btn": sAhb,
				"sisa_koli_hv": sKhj + sKhs + sKhb,
				"sisa_hv_awb":  sEhj + sEhs + sEhb,
				"sisa_ecer_hv": sEhj + sEhs + sEhb,
				"sisa_awb_hv":  sAhj + sAhs + sAhb,
			})
		}
	} else {
		log.Printf("[Kapten] gagal ambil sisa grup seller %d: %v", sellerID, err)
	}

	return response.OK(c, map[string]interface{}{
		"riwayat":   list,
		"sisa_grup": sisaGrup,
	})
}

// GetKonfirmasiHistory mengambil riwayat konfirmasi seluruh implant kapten:
// konfirmasi tersimpan (diambil/terlambat) + barang terlewat yang dihitung
// dari input pending dan trip asal yang sudah selesai.
// GET /api/v1/kapten/konfirmasi-history
func (h *Handler) GetKonfirmasiHistory(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()

	ids, names, err := h.myImplantIDs(ctx, userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil daftar implant")
	}
	if len(ids) == 0 {
		return response.OK(c, map[string]interface{}{"riwayat": []map[string]interface{}{}})
	}
	points, pointNames := h.implantPointsForUser(ctx, userID)
	allowed := map[int64]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	filterImplant := int64(0)
	if raw := strings.TrimSpace(c.QueryParam("id_implant")); raw != "" {
		fmt.Sscanf(raw, "%d", &filterImplant)
		if filterImplant > 0 && !allowed[filterImplant] {
			return response.Error(c, http.StatusBadRequest, "implant di luar akses")
		}
	}

	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	today := time.Now().In(wib)
	todayStr := today.Format("2006-01-02")
	windows := h.loadRitWindows(ctx)
	startDate, endDate := "", ""
	filter := strings.ToLower(strings.TrimSpace(c.QueryParam("filter")))
	switch filter {
	case "today":
		startDate, endDate = todayStr, todayStr
	case "week", "7d":
		startDate, endDate = today.AddDate(0, 0, -6).Format("2006-01-02"), todayStr
	case "month":
		startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, wib).Format("2006-01-02")
		endDate = todayStr
	case "all":
		startDate, endDate = "2000-01-01", todayStr
	default:
		startDate = strings.TrimSpace(c.QueryParam("start_date"))
		endDate = strings.TrimSpace(c.QueryParam("end_date"))
		if startDate == "" && endDate == "" {
			startDate, endDate = todayStr, todayStr
		} else if startDate == "" {
			startDate = endDate
		} else if endDate == "" {
			endDate = todayStr
		}
	}
	if startDate != "" {
		if _, err := time.Parse("2006-01-02", startDate); err != nil {
			return response.Error(c, http.StatusBadRequest, "start_date tidak valid")
		}
	}
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err != nil {
			return response.Error(c, http.StatusBadRequest, "end_date tidak valid")
		}
	}

	intVal := func(v interface{}) int {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			var out int
			fmt.Sscanf(fmt.Sprint(v), "%d", &out)
			return out
		}
	}
	result := []map[string]interface{}{}
	for _, id := range ids {
		if filterImplant > 0 && id != filterImplant {
			continue
		}
		sellerID := h.sellerForImplant(ctx, id)
		pointID := points[id]
		if pointID == 0 {
			pointID = id
		}
		pointName := pointNames[pointID]
		args := []interface{}{id, sellerID}
		where := `(kp.id_implant = $1 OR kp.id_seller = $2)`
		if startDate != "" {
			args = append(args, startDate)
			where += fmt.Sprintf(` AND (kp.created_at + interval '7 hours')::date >= $%d::date`, len(args))
		}
		if endDate != "" {
			args = append(args, endDate)
			where += fmt.Sprintf(` AND (kp.created_at + interval '7 hours')::date <= $%d::date`, len(args))
		}
		rows, qerr := h.DB.Query(ctx, `
			SELECT kp.id, kp.id_ritase, COALESCE(kp.jenis_ritase, ''), kp.ritase_ke,
			       COALESCE(r.ritase_ke, kp.ritase_ke),
			       COALESCE(r.tanggal, (kp.created_at + interval '7 hours')::date),
			       COALESCE(kp.jumlah_awb, 0),
			       COALESCE(kp.koli_jkt, 0) + COALESCE(kp.koli_seg, 0) + COALESCE(kp.koli_btn, 0),
			       COALESCE(kp.ecer_jkt, 0) + COALESCE(kp.ecer_seg, 0) + COALESCE(kp.ecer_btn, 0),
			       COALESCE(kp.koli_hv_jkt, 0) + COALESCE(kp.koli_hv_seg, 0) + COALESCE(kp.koli_hv_btn, 0) +
			           COALESCE(kp.ecer_hv_jkt, 0) + COALESCE(kp.ecer_hv_seg, 0) + COALESCE(kp.ecer_hv_btn, 0) +
			           COALESCE(kp.awb_hv_jkt, 0) + COALESCE(kp.awb_hv_seg, 0) + COALESCE(kp.awb_hv_btn, 0),
			       COALESCE(kp.foto_penjemputan_url, ''), COALESCE(kp.catatan, ''),
			       kp.created_at, COALESCE(r.kode_ritase, ''), COALESCE(d.nama_driver, '')
			FROM konfirmasi_penjemputan kp
			LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
			LEFT JOIN driver d ON d.id_driver = r.id_driver
			WHERE `+where+`
			ORDER BY kp.created_at DESC
			LIMIT 500
		`, args...)
		if qerr == nil {
			for rows.Next() {
				var kid, kRitase int64
				var kAsal, kAktual, awb, koli, ecer, hv int
				var jenis, foto, catatan, kode, driver string
				var tgl, created time.Time
				if err := rows.Scan(&kid, &kRitase, &jenis, &kAsal, &kAktual, &tgl,
					&awb, &koli, &ecer, &hv, &foto, &catatan, &created, &kode, &driver); err != nil {
					continue
				}
				status := "diambil"
				if kAktual > kAsal {
					status = "terlambat"
				}
				result = append(result, map[string]interface{}{
					"id": kid, "id_ritase": kRitase,
					"jenis_ritase": jenis, "ritase_ke": kAsal, "ritase_ke_aktual": kAktual,
					"status":        status,
					"tanggal_input": tgl.Format("2006-01-02"), "tanggal_aktual": tgl.Format("2006-01-02"),
					"jumlah_awb": awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
					"foto_penjemputan_url": foto, "catatan": catatan,
					"created_at":  created.Format(time.RFC3339),
					"kode_ritase": kode, "nama_driver": driver,
					"id_implant": id, "nama_implant": names[id],
					"id_titik_implant": pointID, "nama_titik_implant": pointName,
				})
			}
			rows.Close()
		}

		pending := h.batchPendingBetween(ctx, id, sellerID, startDate, endDate)
		for _, p := range pending {
			jenis := fmt.Sprint(p["jenis_ritase"])
			ritKe := intVal(p["ritase_ke"])
			tgl := fmt.Sprint(p["tanggal_input"])
			trip := h.batchTripForGroupDate(ctx, id, pointID, sellerID, jenis, ritKe, tgl)
			if !pendingOverdue(p, windows, today) {
				continue
			}
			entry := make(map[string]interface{}, len(p)+16)
			for k, v := range p {
				entry[k] = v
			}
			entry["id"] = 0
			entry["status"] = "terlewat"
			entry["terlambat"] = true
			entry["ritase_ke_aktual"] = 0
			entry["tanggal_aktual"] = ""
			entry["id_ritase"] = int64(0)
			entry["kode_ritase"] = ""
			entry["nama_driver"] = ""
			if trip != nil {
				entry["id_ritase"] = trip["id_ritase"]
				entry["kode_ritase"] = trip["kode_ritase"]
				entry["nama_driver"] = trip["nama_driver"]
			}
			entry["foto_penjemputan_url"] = ""
			entry["catatan"] = ""
			entry["id_implant"] = id
			entry["nama_implant"] = names[id]
			entry["id_titik_implant"] = pointID
			entry["nama_titik_implant"] = pointName
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return fmt.Sprint(result[i]["created_at"]) > fmt.Sprint(result[j]["created_at"])
	})
	if len(result) > 200 {
		result = result[:200]
	}
	return response.OK(c, map[string]interface{}{"riwayat": result})
}

// parseTimeMinutes mengkonversi string jam "HH:MM:SS" atau "HH:MM" ke menit dari midnight.
func parseTimeMinutes(s string) int {
	hour, min := 0, 0
	fmt.Sscanf(s, "%d:%d", &hour, &min)
	return hour*60 + min
}

type ritWindow struct {
	startMin int
	endMin   int
}

var defaultRitWindows = map[string]ritWindow{
	"outgoing|1": {startMin: 0, endMin: 19*60 + 59},
	"outgoing|2": {startMin: 16 * 60, endMin: 23*60 + 59},
	"outgoing|3": {startMin: 20 * 60, endMin: 3 * 60},
	"incoming|1": {startMin: 1 * 60, endMin: 4*60 + 30},
	"incoming|2": {startMin: 7 * 60, endMin: 10*60 + 30},
	"incoming|3": {startMin: 13 * 60, endMin: 16*60 + 30},
	"incoming|4": {startMin: 19 * 60, endMin: 22*60 + 30},
}

func ritWindowKey(jenis string, ritKe int) string {
	return fmt.Sprintf("%s|%d", jenis, ritKe)
}

func (h *Handler) loadRitWindows(ctx context.Context) map[string]ritWindow {
	out := make(map[string]ritWindow, len(defaultRitWindows))
	for k, v := range defaultRitWindows {
		out[k] = v
	}
	rows, err := h.DB.Query(ctx,
		`SELECT jenis, ritase_ke, jam_mulai::text, jam_selesai::text
		 FROM jadwal_ritase_config`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var jenis, mulai, selesai string
		var ritKe int
		if err := rows.Scan(&jenis, &ritKe, &mulai, &selesai); err != nil {
			continue
		}
		out[ritWindowKey(jenis, ritKe)] = ritWindow{
			startMin: parseTimeMinutes(mulai),
			endMin:   parseTimeMinutes(selesai),
		}
	}
	return out
}

func ritDeadline(createdAt time.Time, w ritWindow, loc *time.Location) time.Time {
	createdAt = createdAt.In(loc)
	year, month, day := createdAt.Date()
	deadline := time.Date(
		year, month, day,
		w.endMin/60, w.endMin%60, 59, int(999*time.Millisecond), loc,
	)
	if w.endMin <= w.startMin {
		createdMin := createdAt.Hour()*60 + createdAt.Minute()
		if createdMin >= w.startMin {
			deadline = deadline.AddDate(0, 0, 1)
		}
	}
	return deadline
}

func ritOverdue(createdAt time.Time, w ritWindow, now time.Time) bool {
	loc := now.Location()
	return now.After(ritDeadline(createdAt, w, loc))
}

func pendingOverdue(p map[string]interface{}, windows map[string]ritWindow, now time.Time) bool {
	raw := fmt.Sprint(p["created_at"])
	createdAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	var ritKe int
	fmt.Sscanf(fmt.Sprint(p["ritase_ke"]), "%d", &ritKe)
	jenis := fmt.Sprint(p["jenis_ritase"])
	w, ok := windows[ritWindowKey(jenis, ritKe)]
	if !ok {
		return false
	}
	return ritOverdue(createdAt, w, now)
}

func ritDateOverdue(tanggal, jenis string, ritKe int, windows map[string]ritWindow, now time.Time) bool {
	w, ok := windows[ritWindowKey(jenis, ritKe)]
	if !ok {
		return false
	}
	loc := now.Location()
	day, err := time.ParseInLocation("2006-01-02", tanggal, loc)
	if err != nil {
		return false
	}
	anchor := time.Date(
		day.Year(), day.Month(), day.Day(),
		w.startMin/60, w.startMin%60, 0, 0, loc,
	)
	return now.After(ritDeadline(anchor, w, loc))
}

// myImplantIDs mengembalikan id implant milik kapten yang login.
func (h *Handler) myImplantIDs(ctx context.Context, userID int64) ([]int64, map[int64]string, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT i.id_implant, COALESCE(i.nama_implant, '')
		FROM implant i
		JOIN kapten_implant_map kim ON kim.id_implant = i.id_implant
		WHERE kim.id_user = $1 AND i.status = 'aktif'
		ORDER BY i.nama_implant
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []int64
	names := map[int64]string{}
	for rows.Next() {
		var id int64
		var nama string
		if err := rows.Scan(&id, &nama); err == nil {
			ids = append(ids, id)
			names[id] = nama
		}
	}
	return ids, names, rows.Err()
}

// implantPointsForUser memetakan implant sumber -> titik penjemputan.
func (h *Handler) implantPointsForUser(ctx context.Context, userID int64) (map[int64]int64, map[int64]string) {
	points := map[int64]int64{}
	names := map[int64]string{}
	rows, err := h.DB.Query(ctx, `
		SELECT kim.id_implant, COALESCE(kim.id_titik_implant, i.id_implant),
		       COALESCE(ht.nama_implant, i.nama_implant)
		FROM kapten_implant_map kim
		JOIN implant i ON i.id_implant = kim.id_implant
		LEFT JOIN implant ht ON ht.id_implant = COALESCE(kim.id_titik_implant, i.id_implant)
		WHERE kim.id_user = $1`, userID)
	if err != nil {
		return points, names
	}
	defer rows.Close()
	for rows.Next() {
		var source, point int64
		var name string
		if err := rows.Scan(&source, &point, &name); err == nil {
			points[source] = point
			names[point] = name
		}
	}
	return points, names
}

// implantPointForUser mengambil titik satu implant. Jika mapping tidak ada,
// implant dianggap titik sendiri (kompatibilitas data lama).
func (h *Handler) implantPointForUser(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, userID, implantID int64) (int64, string) {
	var point int64
	var name string
	err := q.QueryRow(ctx, `
		SELECT COALESCE(kim.id_titik_implant, i.id_implant), COALESCE(ht.nama_implant, i.nama_implant)
		FROM kapten_implant_map kim
		JOIN implant i ON i.id_implant = kim.id_implant
		LEFT JOIN implant ht ON ht.id_implant = COALESCE(kim.id_titik_implant, i.id_implant)
		WHERE kim.id_user = $1 AND kim.id_implant = $2`, userID, implantID).Scan(&point, &name)
	if err != nil {
		return implantID, ""
	}
	return point, name
}

// sellerForImplant mencari padanan seller legacy sebuah implant (0 bila tak ada).
func (h *Handler) sellerForImplant(ctx context.Context, implantID int64) int64 {
	var sellerID int64
	_ = h.DB.QueryRow(ctx, `
		SELECT s.id_seller FROM seller s
		JOIN implant i ON i.kode_implant = s.kode_seller
		WHERE i.id_implant = $1`, implantID).Scan(&sellerID)
	return sellerID
}

// batchPending mengambil grup input NULL satu implant dalam 7 hari terakhir.
// Dikelompokkan per tanggal input agar barang yang terlewat beberapa hari tetap
// bisa dikonfirmasi memakai Rit berikutnya.
func (h *Handler) batchPending(ctx context.Context, implantID, sellerID int64) []map[string]interface{} {
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	end := time.Now().In(wib).Format("2006-01-02")
	start := time.Now().In(wib).AddDate(0, 0, -6).Format("2006-01-02")
	return h.batchPendingBetween(ctx, implantID, sellerID, start, end)
}

// batchPendingBetween mengambil grup input NULL pada rentang tanggal WIB.
func (h *Handler) batchPendingBetween(ctx context.Context, implantID, sellerID int64, startDate, endDate string) []map[string]interface{} {
	list := []map[string]interface{}{}
	rows, err := h.DB.Query(ctx, `
		SELECT (created_at + interval '7 hours')::date AS tgl,
		       jenis_ritase, ritase_ke, COUNT(*),
		       COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn + awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0),
		       MAX(created_at)
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date BETWEEN $3::date AND $4::date
		GROUP BY 1, 2, 3
		ORDER BY 1 DESC, 3 ASC
	`, implantID, sellerID, startDate, endDate)
	if err != nil {
		return list
	}
	defer rows.Close()
	for rows.Next() {
		var tgl, lastAt time.Time
		var jenis string
		var ritKe, count int
		var awb, koli, ecer, hv int
		var koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
		var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
		var awbHvJkt, awbHvSeg, awbHvBtn int
		if err := rows.Scan(&tgl, &jenis, &ritKe, &count, &awb, &koli, &ecer, &hv,
			&koliJkt, &koliSeg, &koliBtn, &ecerJkt, &ecerSeg, &ecerBtn,
			&koliHvJkt, &koliHvSeg, &koliHvBtn, &ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
			&awbHvJkt, &awbHvSeg, &awbHvBtn,
			&lastAt); err != nil {
			continue
		}
		list = append(list, map[string]interface{}{
			"jenis_ritase": jenis, "ritase_ke": ritKe, "jumlah_input": count,
			"tanggal_input": tgl.Format("2006-01-02"),
			"created_at":    lastAt.Format(time.RFC3339),
			"total_awb":     awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
			"koli_jkt": koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
			"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
			"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
			"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
			"awb_hv_jkt": awbHvJkt, "awb_hv_seg": awbHvSeg, "awb_hv_btn": awbHvBtn,
		})
	}
	return list
}

// batchTripForGroupDate mengambil trip terakhir sebuah grup pada rentang tanggal
// input. Trip aktif menang atas trip selesai agar grup tidak dianggap terlewat
// saat Rit baru untuk grup yang sama masih berjalan.
func (h *Handler) batchTripForGroupDate(
	ctx context.Context,
	implantID, pointID, sellerID int64,
	jenis string,
	ritKe int,
	tanggal string,
) map[string]interface{} {
	if pointID == 0 {
		pointID = implantID
	}
	if tanggal == "" {
		wib, _ := time.LoadLocation("Asia/Jakarta")
		if wib == nil {
			wib = time.FixedZone("WIB", 7*60*60)
		}
		tanggal = time.Now().In(wib).Format("2006-01-02")
	}
	var idRitase int64
	var kode, status, namaDriver, plat string
	var rKe int
	var rJenis *string
	var tgl time.Time
	var jamMulai, jamSelesai *string
	err := h.DB.QueryRow(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.status, r.ritase_ke, r.jenis_ritase,
		       r.tanggal, r.jam_mulai, r.jam_selesai,
		       COALESCE(d.nama_driver, ''), COALESCE(k.plat_nomor, '')
		FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
		                   AND (rs.id_implant = $1 OR rs.id_implant = $2 OR rs.id_seller = $3)
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.jenis_ritase = $4 AND r.ritase_ke = $5
		  AND r.tanggal BETWEEN $6::date - 1 AND $6::date + 1
		ORDER BY (r.status <> 'selesai') DESC, r.tanggal DESC, r.id_ritase DESC
		LIMIT 1
	`, implantID, pointID, sellerID, jenis, ritKe, tanggal).Scan(
		&idRitase, &kode, &status, &rKe, &rJenis, &tgl, &jamMulai, &jamSelesai,
		&namaDriver, &plat,
	)
	if err != nil {
		return nil
	}
	statusGrup := "aktif"
	if status == "selesai" {
		statusGrup = "selesai"
	}
	jamMulaiStr, jamSelesaiStr := "", ""
	if jamMulai != nil {
		jamMulaiStr = *jamMulai
	}
	if jamSelesai != nil {
		jamSelesaiStr = *jamSelesai
	}
	jenisStr := jenis
	if rJenis != nil && *rJenis != "" {
		jenisStr = *rJenis
	}
	return map[string]interface{}{
		"status_grup":    statusGrup,
		"status_ritase":  status,
		"id_ritase":      idRitase,
		"kode_ritase":    kode,
		"jenis_ritase":   jenisStr,
		"ritase_ke":      rKe,
		"tanggal_ritase": tgl.Format("2006-01-02"),
		"jam_mulai":      jamMulaiStr,
		"jam_selesai":    jamSelesaiStr,
		"nama_driver":    namaDriver,
		"plat_nomor":     plat,
	}
}

// batchSisa mengambil sisa per grup satu implant (bentuk sama dengan sisa_grup).
func (h *Handler) batchSisa(ctx context.Context, implantID, sellerID int64) []map[string]interface{} {
	sisaGrup := []map[string]interface{}{}
	sRows, err := h.DB.Query(ctx, `
		WITH inp AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb,
			       COALESCE(SUM(t.awb_hv_jkt), 0) AS w_ahj, COALESCE(SUM(t.awb_hv_seg), 0) AS w_ahs, COALESCE(SUM(t.awb_hv_btn), 0) AS w_ahb
			FROM input_kapten t
			WHERE (t.id_implant = $1 OR t.id_seller = $2)
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		),
		amb AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb,
			       COALESCE(SUM(t.awb_hv_jkt), 0) AS w_ahj, COALESCE(SUM(t.awb_hv_seg), 0) AS w_ahs, COALESCE(SUM(t.awb_hv_btn), 0) AS w_ahb
			FROM konfirmasi_penjemputan t
			WHERE (t.id_implant = $1 OR t.id_seller = $2)
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			  AND EXISTS (
				SELECT 1 FROM input_kapten ik
				WHERE ik.id_ritase = t.id_ritase
				  AND ik.jenis_ritase = t.jenis_ritase
				  AND ik.ritase_ke = t.ritase_ke
				  AND (ik.id_implant = t.id_implant OR ik.id_seller = t.id_seller)
				  AND (ik.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			  )
			GROUP BY t.jenis_ritase, t.ritase_ke
		)
		SELECT COALESCE(i.jenis_ritase, a.jenis_ritase, ''),
		       COALESCE(i.ritase_ke, a.ritase_ke, 0),
		       COALESCE(i.awb, 0), COALESCE(i.koli, 0), COALESCE(i.ecer, 0), COALESCE(i.hv, 0),
		       COALESCE(a.awb, 0), COALESCE(a.koli, 0), COALESCE(a.ecer, 0), COALESCE(a.hv, 0),
		       COALESCE(i.w_awb, 0) - COALESCE(a.w_awb, 0),
		       COALESCE(i.w_kj, 0) - COALESCE(a.w_kj, 0),
		       COALESCE(i.w_ks, 0) - COALESCE(a.w_ks, 0),
		       COALESCE(i.w_kb, 0) - COALESCE(a.w_kb, 0),
		       COALESCE(i.w_ej, 0) - COALESCE(a.w_ej, 0),
		       COALESCE(i.w_es, 0) - COALESCE(a.w_es, 0),
		       COALESCE(i.w_eb, 0) - COALESCE(a.w_eb, 0),
		       COALESCE(i.w_khj, 0) - COALESCE(a.w_khj, 0),
		       COALESCE(i.w_khs, 0) - COALESCE(a.w_khs, 0),
		       COALESCE(i.w_khb, 0) - COALESCE(a.w_khb, 0),
		       COALESCE(i.w_ehj, 0) - COALESCE(a.w_ehj, 0),
		       COALESCE(i.w_ehs, 0) - COALESCE(a.w_ehs, 0),
		       COALESCE(i.w_ehb, 0) - COALESCE(a.w_ehb, 0),
		       COALESCE(i.w_ahj, 0) - COALESCE(a.w_ahj, 0),
		       COALESCE(i.w_ahs, 0) - COALESCE(a.w_ahs, 0),
		       COALESCE(i.w_ahb, 0) - COALESCE(a.w_ahb, 0)
		FROM inp i
		FULL OUTER JOIN amb a USING (jenis_ritase, ritase_ke)
		ORDER BY 2
	`, implantID, sellerID)
	if err != nil {
		return sisaGrup
	}
	defer sRows.Close()
	for sRows.Next() {
		var jenis string
		var ritKe int
		var inAwb, inKoli, inEcer, inHV, amAwb, amKoli, amEcer, amHV int
		var sAwb, sKj, sKs, sKb, sEj, sEs, sEb int
		var sKhj, sKhs, sKhb, sEhj, sEhs, sEhb int
		var sAhj, sAhs, sAhb int
		if err := sRows.Scan(&jenis, &ritKe,
			&inAwb, &inKoli, &inEcer, &inHV, &amAwb, &amKoli, &amEcer, &amHV,
			&sAwb, &sKj, &sKs, &sKb, &sEj, &sEs, &sEb,
			&sKhj, &sKhs, &sKhb, &sEhj, &sEhs, &sEhb, &sAhj, &sAhs, &sAhb); err != nil {
			continue
		}
		sisaGrup = append(sisaGrup, map[string]interface{}{
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"input_awb": inAwb, "input_koli": inKoli, "input_ecer": inEcer, "input_hv": inHV,
			"diambil_awb": amAwb, "diambil_koli": amKoli, "diambil_ecer": amEcer, "diambil_hv": amHV,
			"sisa_awb": inAwb - amAwb, "sisa_koli": inKoli - amKoli,
			"sisa_ecer": inEcer - amEcer, "sisa_hv": inHV - amHV,
			"sisa_jumlah_awb": sAwb,
			"sisa_koli_jkt":   sKj, "sisa_koli_seg": sKs, "sisa_koli_btn": sKb,
			"sisa_ecer_jkt": sEj, "sisa_ecer_seg": sEs, "sisa_ecer_btn": sEb,
			"sisa_koli_hv_jkt": sKhj, "sisa_koli_hv_seg": sKhs, "sisa_koli_hv_btn": sKhb,
			"sisa_ecer_hv_jkt": sEhj, "sisa_ecer_hv_seg": sEhs, "sisa_ecer_hv_btn": sEhb,
			"sisa_awb_hv_jkt": sAhj, "sisa_awb_hv_seg": sAhs, "sisa_awb_hv_btn": sAhb,
			"sisa_koli_hv": sKhj + sKhs + sKhb,
			"sisa_hv_awb":  sEhj + sEhs + sEhb,
			"sisa_ecer_hv": sEhj + sEhs + sEhb,
			"sisa_awb_hv":  sAhj + sAhs + sAhb,
		})
	}
	return sisaGrup
}

// GetBatchOverview mengembalikan pending + sisa + riwayat SEMUA implant
// kapten sekaligus â€” bahan layar konfirmasi gabungan satu tap.
// GET /api/v1/kapten/batch-overview
func (h *Handler) GetBatchOverview(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	ids, names, err := h.myImplantIDs(ctx, userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil daftar implant")
	}
	points, pointNames := h.implantPointsForUser(ctx, userID)
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(wib)
	todayStr := now.Format("2006-01-02")
	windows := h.loadRitWindows(ctx)
	intVal := func(v interface{}) int {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			var out int
			fmt.Sscanf(fmt.Sprint(v), "%d", &out)
			return out
		}
	}
	out := []map[string]interface{}{}
	for _, id := range ids {
		sellerID := h.sellerForImplant(ctx, id)
		pointID := points[id]
		if pointID == 0 {
			pointID = id
		}
		pointName := pointNames[pointID]
		if pointName == "" {
			pointName = names[pointID]
		}
		pendingAll := h.batchPending(ctx, id, sellerID)
		sisa := h.batchSisa(ctx, id, sellerID)
		for _, p := range pendingAll {
			p["id_titik_implant"] = pointID
			p["nama_titik_implant"] = pointName
		}
		for _, s := range sisa {
			s["id_titik_implant"] = pointID
			s["nama_titik_implant"] = pointName
		}
		// Riwayat hari ini (ringkas: id, ritase, jumlah, waktu, driver).
		riwayat := []map[string]interface{}{}
		rRows, err := h.DB.Query(ctx, `
			SELECT kp.id, kp.id_ritase, kp.jenis_ritase, kp.ritase_ke,
			       COALESCE(kp.jumlah_awb, 0),
			       COALESCE(kp.koli_jkt, 0) + COALESCE(kp.koli_seg, 0) + COALESCE(kp.koli_btn, 0),
			       COALESCE(kp.ecer_jkt, 0) + COALESCE(kp.ecer_seg, 0) + COALESCE(kp.ecer_btn, 0),
			       COALESCE(kp.koli_hv_jkt, 0) + COALESCE(kp.koli_hv_seg, 0) + COALESCE(kp.koli_hv_btn, 0) +
			           COALESCE(kp.ecer_hv_jkt, 0) + COALESCE(kp.ecer_hv_seg, 0) + COALESCE(kp.ecer_hv_btn, 0) +
			           COALESCE(kp.awb_hv_jkt, 0) + COALESCE(kp.awb_hv_seg, 0) + COALESCE(kp.awb_hv_btn, 0),
			       kp.created_at,
			       COALESCE(r.kode_ritase, ''), COALESCE(d.nama_driver, ''),
			       COALESCE(kp.foto_penjemputan_url, ''), COALESCE(kp.catatan, '')
			FROM konfirmasi_penjemputan kp
			LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
			LEFT JOIN driver d ON d.id_driver = r.id_driver
			WHERE (kp.id_implant = $1 OR kp.id_seller = $2)
			  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			ORDER BY kp.created_at DESC
		`, id, sellerID)
		if err == nil {
			for rRows.Next() {
				var rid, rrit int64
				var rke, awb, koli, ecer, hv int
				var jenis, kode, driver, foto, catatan string
				var created time.Time
				if err := rRows.Scan(&rid, &rrit, &jenis, &rke, &awb, &koli, &ecer, &hv,
					&created, &kode, &driver, &foto, &catatan); err == nil {
					riwayat = append(riwayat, map[string]interface{}{
						"id": rid, "id_ritase": rrit, "jenis_ritase": jenis, "ritase_ke": rke,
						"jumlah_awb": awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
						"created_at":  created.Format(time.RFC3339),
						"kode_ritase": kode, "nama_driver": driver,
						"foto_penjemputan_url": foto, "catatan": catatan,
						"id_implant": id, "nama_implant": names[id],
						"id_titik_implant": pointID, "nama_titik_implant": pointName,
					})
				}
			}
			rRows.Close()
		}

		pendingOut := []map[string]interface{}{}
		normalSeen := map[string]bool{}
		overdueTodayKeys := map[string]bool{}
		ritGroupMap := map[string]map[string]interface{}{}
		addGroupInfo := func(key, jenis string, ritKe int, status string, trip map[string]interface{}) {
			info := map[string]interface{}{
				"jenis_ritase": jenis, "ritase_ke": ritKe, "status_grup": status,
			}
			if trip != nil {
				for k, v := range trip {
					info[k] = v
				}
			}
			if old, ok := ritGroupMap[key]; ok && old["status_grup"] == "aktif" && status != "aktif" {
				return
			}
			ritGroupMap[key] = info
		}
		for _, p := range pendingAll {
			jenis := fmt.Sprint(p["jenis_ritase"])
			ritKe := intVal(p["ritase_ke"])
			tgl := fmt.Sprint(p["tanggal_input"])
			key := fmt.Sprintf("%s|%d", jenis, ritKe)
			if pendingOverdue(p, windows, now) {
				if tgl == todayStr {
					overdueTodayKeys[key] = true
				}
				continue
			}
			trip := h.batchTripForGroupDate(ctx, id, pointID, sellerID, jenis, ritKe, tgl)
			status := "belum_ada_trip"
			if trip != nil {
				status = fmt.Sprint(trip["status_grup"])
			}
			entry := make(map[string]interface{}, len(p)+12)
			for k, v := range p {
				entry[k] = v
			}
			entry["status_grup"] = status
			if trip != nil {
				for k, v := range trip {
					entry[k] = v
				}
			}
			addGroupInfo(key, jenis, ritKe, status, trip)
			// Pending lama tanpa trip tidak ditampilkan sebagai form biasa.
			if status == "belum_ada_trip" && tgl != todayStr {
				continue
			}
			if normalSeen[key] {
				continue
			}
			normalSeen[key] = true
			entry["status"] = "aktif"
			entry["terlambat"] = false
			pendingOut = append(pendingOut, entry)
		}
		// Anotasi sisa grup dengan status trip agar UI tidak menampilkan form
		// untuk grup yang trip asalnya sudah selesai.
		for _, s := range sisa {
			jenis := fmt.Sprint(s["jenis_ritase"])
			ritKe := intVal(s["ritase_ke"])
			key := fmt.Sprintf("%s|%d", jenis, ritKe)
			if overdueTodayKeys[key] {
				continue
			}
			trip := h.batchTripForGroupDate(ctx, id, pointID, sellerID, jenis, ritKe, todayStr)
			status := "belum_ada_trip"
			if trip != nil {
				status = fmt.Sprint(trip["status_grup"])
			}
			s["status_grup"] = status
			addGroupInfo(key, jenis, ritKe, status, trip)
		}
		ritGroups := make([]map[string]interface{}, 0, len(ritGroupMap))
		for _, g := range ritGroupMap {
			ritGroups = append(ritGroups, g)
		}
		sort.Slice(ritGroups, func(i, j int) bool {
			ri, rj := intVal(ritGroups[i]["ritase_ke"]), intVal(ritGroups[j]["ritase_ke"])
			if ri != rj {
				return ri < rj
			}
			return fmt.Sprint(ritGroups[i]["jenis_ritase"]) < fmt.Sprint(ritGroups[j]["jenis_ritase"])
		})
		out = append(out, map[string]interface{}{
			"id_implant": id, "nama_implant": names[id],
			"id_titik_implant": pointID, "nama_titik_implant": pointName,
			"pending": pendingOut, "late_pending": []map[string]interface{}{},
			"sisa_grup": sisa, "riwayat": riwayat, "rit_groups": ritGroups,
		})
	}
	return response.OK(c, map[string]interface{}{"implants": out})
}

// GetBatchRitase mengembalikan ritase hari ini (WIB) yang menyinggahi SEMUA
// implant yang diminta + sudah ada driver & kendaraan â€” kandidat satu driver.
// GET /api/v1/kapten/batch-ritase?implant_ids=1,2,3
func (h *Handler) GetBatchRitase(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	var ids []int64
	pointMode := false
	pointID := int64(0)
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	myIDs, _, _ := h.myImplantIDs(ctx0, userID)
	points, _ := h.implantPointsForUser(ctx0, userID)
	cancel0()
	allowed := map[int64]bool{}
	for _, id := range myIDs {
		allowed[id] = true
	}
	if raw := strings.TrimSpace(c.QueryParam("titik_id")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &pointID); err != nil || pointID <= 0 {
			return response.Error(c, http.StatusBadRequest, "titik_id tidak valid")
		}
		pointMode = true
		if !allowed[pointID] {
			return response.Error(c, http.StatusBadRequest, "titik tidak valid / di luar akses")
		}
		ids = append(ids, pointID)
		for source, host := range points {
			if host == pointID && source != pointID {
				ids = append(ids, source)
			}
		}
	} else {
		for _, part := range strings.Split(c.QueryParam("implant_ids"), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			var id int64
			if _, err := fmt.Sscanf(part, "%d", &id); err != nil || id <= 0 || !allowed[id] {
				return response.Error(c, http.StatusBadRequest, "daftar implant tidak valid / di luar akses")
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return response.Error(c, http.StatusBadRequest, "titik_id atau implant_ids wajib diisi")
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	condition := `(SELECT COUNT(DISTINCT rs.id_implant) FROM ritase_stop rs
		       WHERE rs.id_ritase = r.id_ritase AND rs.id_implant = ANY($1)
		      ) = array_length($1, 1)`
	if pointMode {
		condition = `EXISTS (SELECT 1 FROM ritase_stop rs
		       WHERE rs.id_ritase = r.id_ritase AND rs.id_implant = ANY($1))`
	}
	query := `
		SELECT r.id_ritase, r.kode_ritase, r.ritase_ke, r.status,
		       r.jam_mulai, r.jam_selesai, r.jenis_ritase, r.tanggal,
		       COALESCE(d.nama_driver, 'Driver'), COALESCE(k.plat_nomor, '-')
		FROM ritase r
		JOIN driver d ON d.id_driver = r.id_driver
		JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
		                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
		  AND r.status != 'selesai'
		  AND ` + condition + `
		ORDER BY r.ritase_ke ASC, r.id_ritase ASC
	`
	rows, err := h.DB.Query(ctx, query, ids)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data ritase")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	seen := map[int64]bool{}
	for rows.Next() {
		var idRitase int64
		var kode, status, namaDriver, plat string
		var ritKe int
		var jamMulai, jamSelesai, jenis *string
		var tanggal time.Time
		if err := rows.Scan(&idRitase, &kode, &ritKe, &status,
			&jamMulai, &jamSelesai, &jenis, &tanggal,
			&namaDriver, &plat); err != nil {
			continue
		}
		if seen[idRitase] {
			continue
		}
		seen[idRitase] = true
		list = append(list, map[string]interface{}{
			"id_ritase": idRitase, "kode_ritase": kode,
			"ritase_ke": ritKe, "status": status,
			"jam_mulai": jamMulai, "jam_selesai": jamSelesai,
			"jenis_ritase": jenis,
			"tanggal":      tanggal.Format("2006-01-02"),
			"nama_driver":  namaDriver, "plat_nomor": plat,
		})
	}
	return response.OK(c, list)
}

// BatchConfirmItem adalah satu titik dalam konfirmasi gabungan.
type BatchConfirmItem struct {
	IDImplant        int64 `json:"id_implant"`
	DiambilAWB       int   `json:"diambil_awb"`
	DiambilKoliJKT   int   `json:"diambil_koli_jkt"`
	DiambilKoliSEG   int   `json:"diambil_koli_seg"`
	DiambilKoliBTN   int   `json:"diambil_koli_btn"`
	DiambilEcerJKT   int   `json:"diambil_ecer_jkt"`
	DiambilEcerSEG   int   `json:"diambil_ecer_seg"`
	DiambilEcerBTN   int   `json:"diambil_ecer_btn"`
	DiambilKoliHVJKT int   `json:"diambil_koli_hv_jkt"`
	DiambilKoliHVSEG int   `json:"diambil_koli_hv_seg"`
	DiambilKoliHVBTN int   `json:"diambil_koli_hv_btn"`
	DiambilEcerHVJKT int   `json:"diambil_ecer_hv_jkt"`
	DiambilEcerHVSEG int   `json:"diambil_ecer_hv_seg"`
	DiambilEcerHVBTN int   `json:"diambil_ecer_hv_btn"`
	DiambilAwbHVJKT  int   `json:"diambil_awb_hv_jkt"`
	DiambilAwbHVSEG  int   `json:"diambil_awb_hv_seg"`
	DiambilAwbHVBTN  int   `json:"diambil_awb_hv_btn"`
	DiambilKoliHV    int   `json:"diambil_koli_hv"`
	DiambilHvAwb     int   `json:"diambil_hv_awb"`
	DiambilEcerHV    int   `json:"diambil_ecer_hv"`
	DiambilAwbHv     int   `json:"diambil_awb_hv"`
}

// BatchConfirmRequest adalah body konfirmasi gabungan: SATU ritase/driver
// untuk BANYAK implant sekaligus (satu tap, satu transaksi atomik).
type BatchConfirmRequest struct {
	JenisRitase        string             `json:"jenis_ritase"`
	RitaseKe           int                `json:"ritase_ke"`
	RitaseKeAsal       int                `json:"ritase_ke_asal"`
	TanggalAsal        string             `json:"tanggal_asal"`
	IDRitase           int64              `json:"id_ritase"`
	FotoPenjemputanURL string             `json:"foto_penjemputan_url"`
	Catatan            string             `json:"catatan"`
	Items              []BatchConfirmItem `json:"items"`
}

// PostBatchConfirmPickup menautkan input NULL semua implant terpilih ke satu
// ritase + mencatat realisasi per implant â€” atomik (semua/sama-sekali).
// POST /api/v1/kapten/confirm-pickup-batch
func (h *Handler) PostBatchConfirmPickup(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	var req BatchConfirmRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if req.RitaseKe < 1 || req.RitaseKe > 4 {
		return response.Error(c, http.StatusBadRequest, "ritase_ke tidak valid")
	}
	if req.RitaseKeAsal == 0 {
		req.RitaseKeAsal = req.RitaseKe
	}
	if req.RitaseKeAsal < 1 || req.RitaseKeAsal != req.RitaseKe {
		return response.Error(c, http.StatusBadRequest, "konfirmasi hanya untuk Rit asal; barang terlambat masuk Riwayat")
	}
	if req.IDRitase <= 0 || len(req.Items) == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase dan items wajib diisi")
	}
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(wib)
	if req.TanggalAsal == "" {
		req.TanggalAsal = now.Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", req.TanggalAsal); err != nil {
		return response.Error(c, http.StatusBadRequest, "tanggal_asal tidak valid")
	}
	hariIni := now.Format("2006-01-02")
	kemarin := now.AddDate(0, 0, -1).Format("2006-01-02")
	if req.TanggalAsal != hariIni && req.TanggalAsal != kemarin {
		return response.Error(c, http.StatusBadRequest, "tanggal_asal di luar rentang")
	}

	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	myIDs, _, _ := h.myImplantIDs(ctx0, userID)
	cancel0()
	allowed := map[int64]bool{}
	for _, id := range myIDs {
		allowed[id] = true
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	seenImplant := map[int64]bool{}
	for i := range req.Items {
		it := &req.Items[i]
		if !allowed[it.IDImplant] {
			return response.Error(c, http.StatusBadRequest, "implant di luar akses")
		}
		if seenImplant[it.IDImplant] {
			return response.Error(c, http.StatusBadRequest, "implant duplikat dalam satu konfirmasi")
		}
		seenImplant[it.IDImplant] = true
		it.DiambilAWB = clamp(it.DiambilAWB)
		it.DiambilKoliJKT = clamp(it.DiambilKoliJKT)
		it.DiambilKoliSEG = clamp(it.DiambilKoliSEG)
		it.DiambilKoliBTN = clamp(it.DiambilKoliBTN)
		it.DiambilEcerJKT = clamp(it.DiambilEcerJKT)
		it.DiambilEcerSEG = clamp(it.DiambilEcerSEG)
		it.DiambilEcerBTN = clamp(it.DiambilEcerBTN)
		it.DiambilKoliHVJKT, it.DiambilKoliHVSEG, it.DiambilKoliHVBTN =
			normalisasiHV(it.DiambilKoliHV, 0, it.DiambilKoliHVJKT, it.DiambilKoliHVSEG, it.DiambilKoliHVBTN)
		it.DiambilEcerHVJKT, it.DiambilEcerHVSEG, it.DiambilEcerHVBTN =
			normalisasiHV(it.DiambilHvAwb, it.DiambilEcerHV, it.DiambilEcerHVJKT, it.DiambilEcerHVSEG, it.DiambilEcerHVBTN)
		it.DiambilAwbHVJKT, it.DiambilAwbHVSEG, it.DiambilAwbHVBTN =
			normalisasiHV(it.DiambilAwbHv, 0, it.DiambilAwbHVJKT, it.DiambilAwbHVSEG, it.DiambilAwbHVBTN)
	}

	var fotoURL interface{}
	if req.FotoPenjemputanURL != "" {
		fotoURL = req.FotoPenjemputanURL
	}
	var kaptenUID interface{}
	if userID > 0 {
		kaptenUID = userID
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()
	windows := h.loadRitWindows(ctx)
	if ritDateOverdue(req.TanggalAsal, req.JenisRitase, req.RitaseKe, windows, now) {
		return response.Error(c, http.StatusConflict, "waktu Rit sudah habis; barang terlambat masuk Riwayat")
	}
	if w, ok := windows[ritWindowKey(req.JenisRitase, req.RitaseKe)]; ok && w.endMin <= w.startMin {
		nowMin := now.Hour()*60 + now.Minute()
		if nowMin >= w.endMin && nowMin < w.startMin {
			return response.Error(c, http.StatusConflict, "di luar jam Rit; barang terlambat masuk Riwayat")
		}
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	results := []map[string]interface{}{}
	sellerIDs := map[int64]int64{}
	for _, it := range req.Items {
		// 1. Validasi: ritase ada, belum selesai, dan singgah di titik implant
		//    (fallback: implant sumber untuk rute lama). Satu ritase boleh
		//    menyinggahi beberapa titik dalam satu konfirmasi.
		pointID, _ := h.implantPointForUser(ctx, tx, userID, it.IDImplant)
		if pointID == 0 {
			pointID = it.IDImplant
		}
		var foundID int64
		err = tx.QueryRow(ctx, `
			SELECT r.id_ritase FROM ritase r
			JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
			                   AND (rs.id_implant = $1 OR rs.id_implant = $2)
			WHERE r.id_ritase = $3
			  AND r.status != 'selesai'
			  AND r.jenis_ritase = $4 AND r.ritase_ke = $5
			  AND r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
			                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
		`, it.IDImplant, pointID, req.IDRitase, req.JenisRitase, req.RitaseKe).Scan(&foundID)
		if err != nil {
			return response.Error(c, http.StatusBadRequest, "ritase tidak valid untuk salah satu implant")
		}

		var sellerID int64
		_ = tx.QueryRow(ctx, `
			SELECT s.id_seller FROM seller s
			JOIN implant i ON i.kode_implant = s.kode_seller
			WHERE i.id_implant = $1`, it.IDImplant).Scan(&sellerID)
		sellerIDs[it.IDImplant] = sellerID
		var colSeller interface{}
		if sellerID > 0 {
			colSeller = sellerID
		}

		// 2. Catat serah terima titik ini.
		var idKonfirmasi int64
		err = tx.QueryRow(ctx, `
			INSERT INTO konfirmasi_penjemputan (
				id_ritase, id_seller, id_implant, id_user, jenis_ritase, ritase_ke,
				jumlah_awb,
				koli_jkt, koli_seg, koli_btn,
				ecer_jkt, ecer_seg, ecer_btn,
				koli_hv_jkt, koli_hv_seg, koli_hv_btn,
				ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
				awb_hv_jkt, awb_hv_seg, awb_hv_btn,
				foto_penjemputan_url, catatan, id_titik_implant
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, $12, $13, $14, $15,
				$16, $17, $18, $19, $20, $21, $22, $23, $24, $25
			) RETURNING id
		`, req.IDRitase, colSeller, it.IDImplant, kaptenUID, req.JenisRitase, req.RitaseKeAsal,
			it.DiambilAWB,
			it.DiambilKoliJKT, it.DiambilKoliSEG, it.DiambilKoliBTN,
			it.DiambilEcerJKT, it.DiambilEcerSEG, it.DiambilEcerBTN,
			it.DiambilKoliHVJKT, it.DiambilKoliHVSEG, it.DiambilKoliHVBTN,
			it.DiambilEcerHVJKT, it.DiambilEcerHVSEG, it.DiambilEcerHVBTN,
			it.DiambilAwbHVJKT, it.DiambilAwbHVSEG, it.DiambilAwbHVBTN,
			fotoURL, req.Catatan, pointID,
		).Scan(&idKonfirmasi)
		if err != nil {
			log.Printf("[Kapten] gagal insert batch konfirmasi implant %d: %v", it.IDImplant, err)
			return response.Error(c, http.StatusInternalServerError, "gagal menyimpan serah terima")
		}

		// 3. Tautkan input NULL grup asal.
		tag, err := tx.Exec(ctx, `
			UPDATE input_kapten
			SET id_ritase = $1, updated_at = NOW()
			WHERE (id_implant = $2 OR id_seller = $3) AND jenis_ritase = $4 AND ritase_ke = $5
			  AND id_ritase IS NULL
			  AND (created_at + interval '7 hours')::date = $6::date
		`, req.IDRitase, it.IDImplant, sellerID, req.JenisRitase, req.RitaseKeAsal, req.TanggalAsal)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal menautkan input")
		}

		results = append(results, map[string]interface{}{
			"id_implant":       it.IDImplant,
			"id_titik_implant": pointID,
			"id_konfirmasi":    idKonfirmasi,
			"total_linked":     tag.RowsAffected(),
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan konfirmasi")
	}

	ctxSisa, cancelSisa := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancelSisa()
	for i := range req.Items {
		id := req.Items[i].IDImplant
		var sisaGroup map[string]interface{}
		for _, s := range h.batchSisa(ctxSisa, id, sellerIDs[id]) {
			if s["jenis_ritase"] == req.JenisRitase && s["ritase_ke"] == req.RitaseKeAsal {
				sisaGroup = s
				break
			}
		}
		if sisaGroup != nil {
			results[i]["sisa"] = sisaGroup
			results[i]["sisa_awb"] = sisaGroup["sisa_awb"]
		}
	}

	h.bus.Publish("force_refresh", "kapten_batch_confirm_pickup")

	return response.OK(c, map[string]interface{}{
		"id_ritase":      req.IDRitase,
		"ritase_ke":      req.RitaseKe,
		"ritase_ke_asal": req.RitaseKeAsal,
		"tanggal_asal":   req.TanggalAsal,
		"items":          results,
	})
}

// RegisterRoutes memasang route kapten di grup /api/v1.
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	kapten := g.Group("/kapten", authMW)
	kapten.GET("/my-seller-ritase", h.GetMySellerRitase)
	kapten.POST("/cargo-input", h.PostCargoInput)
	kapten.GET("/seller-info", h.GetSellerInfo)
	kapten.GET("/implant-info", h.GetImplantInfo)
	kapten.GET("/today-cargo", h.GetTodayCargo)
	kapten.GET("/my-sellers", h.GetMySellers)
	kapten.POST("/select-seller", h.SelectSeller)
	kapten.GET("/my-implants", h.GetMyImplants)
	kapten.POST("/select-implant", h.SelectImplant)
	kapten.POST("/confirm-pickup", h.ConfirmPickup)
	kapten.GET("/pending-confirmations", h.GetPendingConfirmations)
	kapten.GET("/konfirmasi-penjemputan", h.GetKonfirmasiPenjemputan)
	kapten.GET("/batch-overview", h.GetBatchOverview)
	kapten.GET("/batch-ritase", h.GetBatchRitase)
	kapten.POST("/confirm-pickup-batch", h.PostBatchConfirmPickup)
	kapten.GET("/konfirmasi-history", h.GetKonfirmasiHistory)
}
