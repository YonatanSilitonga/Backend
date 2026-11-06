package kapten

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
	"backend/internal/pkg/timeutil"
)

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
		now := timeutil.NowWIB()
		if !timeutil.WithinMinutes(now, parseTimeMinutes(jamMulai), parseTimeMinutes(jamSelesai)) {
			log.Printf("[Kapten] input ditolak: waktu %02d:%02d WIB di luar window %s-%s (jenis=%s ritase_ke=%d)",
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