package kapten

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

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

// GetMySellerRitase mengambil daftar ritase yang hari ini singgah di seller kapten.
// GET /api/v1/kapten/my-seller-ritase
func (h *Handler) GetMySellerRitase(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	hariIni := time.Now().Format("2006-01-02")

	rows, err := h.DB.Query(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.ritase_ke, r.status,
		       r.jam_mulai, r.jam_selesai, r.jenis_ritase,
		       COALESCE(r.total_koli, 0), COALESCE(r.total_awb, 0),
		       COALESCE(d.nama_driver, 'Driver'), COALESCE(k.plat_nomor, '-'),
		       rs.id_stop, rs.urutan,
		       COALESCE(rs.foto_manifest_url, '')
		FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase AND rs.id_seller = $1
		JOIN driver d ON d.id_driver = r.id_driver
		JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.tanggal = $2
		  AND r.status != 'selesai'
		ORDER BY r.ritase_ke ASC, rs.urutan ASC
	`, sellerID, hariIni)
	if err != nil {
		log.Printf("[Kapten] gagal ambil ritase seller %d: %v", sellerID, err)
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
			// Jangan telan error diam-diam — baris gagal scan harus terlihat di log.
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
	IDRitase        int64   `json:"id_ritase"`
	IDStop          int64   `json:"id_stop"`
	NamaLokasi      string  `json:"nama_lokasi"`
	JenisRitase     string  `json:"jenis_ritase"` // 'outgoing' atau 'incoming'
	RitaseKe        int     `json:"ritase_ke"`    // 1, 2, 3, atau 4
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	JumlahAWB       int     `json:"jumlah_awb"`
	KoliJKT         int     `json:"koli_jkt"`
	KoliSEG         int     `json:"koli_seg"`
	KoliBTN         int     `json:"koli_btn"`
	EcerJKT         int     `json:"ecer_jkt"`
	EcerSEG         int     `json:"ecer_seg"`
	EcerBTN         int     `json:"ecer_btn"`
	KoliHVJKT       int     `json:"koli_hv_jkt"`
	KoliHVSEG       int     `json:"koli_hv_seg"`
	KoliHVBTN       int     `json:"koli_hv_btn"`
	EcerHVJKT       int     `json:"ecer_hv_jkt"`
	EcerHVSEG       int     `json:"ecer_hv_seg"`
	EcerHVBTN       int     `json:"ecer_hv_btn"`
	FotoManifestURL string  `json:"foto_manifest_url"`
	Catatan         string  `json:"catatan"`
}

// PostCargoInput mencatat input data muatan dari kapten ke input_kapten.
// POST /api/v1/kapten/cargo-input
// Tidak perlu id_ritase — data disimpan independen, di-link nanti saat admin generate ritase.
func (h *Handler) PostCargoInput(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
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

	// Ambil info seller untuk nama lokasi
	var namaSeller string
	_ = h.DB.QueryRow(ctx, `SELECT nama_seller FROM seller WHERE id_seller = $1`, sellerID).Scan(&namaSeller)
	if req.NamaLokasi == "" {
		req.NamaLokasi = namaSeller
	}

	// Ambil user_id dari JWT (kapten yang login)
	var userID int64
	if uid, ok := c.Get(appMiddleware.CtxUserID).(int64); ok && uid > 0 {
		userID = uid
	}

	var fotoURL interface{}
	if req.FotoManifestURL != "" {
		fotoURL = req.FotoManifestURL
	}

	var catatan interface{}
	if req.Catatan != "" {
		catatan = req.Catatan
	}

	// id_ritase SELALU NULL saat input — penautan ke ritase hanya lewat
	// konfirmasi manual kapten (POST /kapten/confirm-pickup). Tidak ada auto-link.
	var idRitase *int64 = nil

	// INSERT ke input_kapten
	var insertedID int64
	err := h.DB.QueryRow(ctx, `
		INSERT INTO input_kapten (
			id_user, id_seller, jenis_ritase, ritase_ke, catatan,
			jumlah_awb, koli_jkt, koli_seg, koli_btn,
			ecer_jkt, ecer_seg, ecer_btn,
			koli_hv_jkt, koli_hv_seg, koli_hv_btn,
			ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			foto_manifest_url, nama_lokasi, latitude, longitude,
			id_ritase
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23
		) RETURNING id
	`, userID, sellerID, req.JenisRitase, req.RitaseKe, catatan,
		req.JumlahAWB, req.KoliJKT, req.KoliSEG, req.KoliBTN,
		req.EcerJKT, req.EcerSEG, req.EcerBTN,
		req.KoliHVJKT, req.KoliHVSEG, req.KoliHVBTN,
		req.EcerHVJKT, req.EcerHVSEG, req.EcerHVBTN,
		fotoURL, req.NamaLokasi, req.Latitude, req.Longitude,
		idRitase,
	).Scan(&insertedID)

	if err != nil {
		log.Printf("[Kapten] gagal insert input_kapten: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan data muatan")
	}

	// Hitung total akumulasi untuk lokasi ini hari ini
	var locAwb, locKoli, locEcer, locHV int
	_ = h.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn), 0)
		FROM input_kapten
		WHERE id_seller = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND DATE(created_at) = CURRENT_DATE
	`, sellerID, req.JenisRitase, req.RitaseKe).Scan(&locAwb, &locKoli, &locEcer, &locHV)

	h.bus.Publish("force_refresh", "kapten_cargo_input")

	return response.Created(c, map[string]interface{}{
		"id":                insertedID,
		"total_awb_lokasi":  locAwb,
		"total_koli_lokasi": locKoli,
		"total_ecer_lokasi": locEcer,
		"total_hv_lokasi":   locHV,
	})
}

// GetSellerInfo mengambil info seller yang terkait dengan kapten.
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

// GetTodayCargo mengambil total akumulasi cargo kapten hari ini dari input_kapten.
// GET /api/v1/kapten/today-cargo?jenis_ritase=outgoing&ritase_ke=1
func (h *Handler) GetTodayCargo(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
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

	// Ambil nama seller
	var namaSeller string
	_ = h.DB.QueryRow(ctx, `SELECT nama_seller FROM seller WHERE id_seller = $1`, sellerID).Scan(&namaSeller)

	// Query dari input_kapten (bukan ritase_event)
	var awb, koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
	var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
	var fotoURL string
	var catatan string
	var isAda bool
	err := h.DB.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE((SELECT foto_manifest_url FROM input_kapten
		                 WHERE id_seller = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		                   AND DATE(created_at) = CURRENT_DATE AND foto_manifest_url IS NOT NULL
		                 ORDER BY id DESC LIMIT 1), ''),
		       COALESCE((SELECT catatan FROM input_kapten
		                 WHERE id_seller = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		                   AND DATE(created_at) = CURRENT_DATE AND catatan IS NOT NULL AND catatan != ''
		                 ORDER BY id DESC LIMIT 1), '')
		FROM input_kapten
		WHERE id_seller = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND DATE(created_at) = CURRENT_DATE
	`, sellerID, jenisRitase, ritaseKe).Scan(&awb, &koliJkt, &koliSeg, &koliBtn,
		&ecerJkt, &ecerSeg, &ecerBtn,
		&koliHvJkt, &koliHvSeg, &koliHvBtn,
		&ecerHvJkt, &ecerHvSeg, &ecerHvBtn, &fotoURL, &catatan)

	totalKoli := koliJkt + koliSeg + koliBtn
	totalEcer := ecerJkt + ecerSeg + ecerBtn
	totalHV := koliHvJkt + koliHvSeg + koliHvBtn + ecerHvJkt + ecerHvSeg + ecerHvBtn
	if err == nil && (totalKoli > 0 || totalEcer > 0 || totalHV > 0 || awb > 0) {
		isAda = true
	}

	return response.OK(c, map[string]interface{}{
		"nama_lokasi": namaSeller,
		"jumlah_awb":  awb,
		"koli_jkt":    koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
		"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
		"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
		"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
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

// SelectSellerRequest adalah body untuk select seller.
type SelectSellerRequest struct {
	SellerID int64 `json:"seller_id"`
}

// SelectSeller memilih seller → return JWT baru dengan seller_id yang dipilih.
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

// ConfirmPickupRequest adalah body konfirmasi driver pengambil oleh kapten.
// Satu konfirmasi menautkan SEMUA input hari ini yang masih NULL dalam grup
// (seller kapten + jenis_ritase + ritase_ke) ke satu ritase (trip) SEKALIGUS
// mencatat berapa yang diambil driver (diambil_*), foto bukti, dan catatan.
// Boleh dikonfirmasi berkali-kali (ambil bertahap) — sisa = input − akumulasi diambil.
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

	FotoPenjemputanURL string `json:"foto_penjemputan_url"` // opsional
	Catatan            string `json:"catatan"`              // opsional (mis. barang sisa)
}

// ConfirmPickup menautkan input kapten yang belum ter-link ke ritase pilihan kapten
// dan mencatat realisasi pengambilan (jumlah + foto + catatan).
// POST /api/v1/kapten/confirm-pickup
func (h *Handler) ConfirmPickup(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
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

	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	// 1. Validasi: ritase harus ada, belum selesai, singgah di seller kapten,
	//    dan jenis + ritase_ke-nya sama dengan grup yang dikonfirmasi.
	var foundID int64
	err = tx.QueryRow(ctx, `
		SELECT r.id_ritase FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase AND rs.id_seller = $1
		WHERE r.id_ritase = $2
		  AND r.status != 'selesai'
		  AND r.jenis_ritase = $3 AND r.ritase_ke = $4
		  AND r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
		                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
	`, sellerID, req.IDRitase, req.JenisRitase, req.RitaseKe).Scan(&foundID)
	if err != nil {
		log.Printf("[Kapten] konfirmasi ditolak seller %d -> ritase %d: %v", sellerID, req.IDRitase, err)
		return response.Error(c, http.StatusBadRequest, "ritase tidak valid untuk seller ini (bukan jadwal yang singgah ke sini)")
	}

	// 2. Catat kejadian serah terima (realisasi pengambilan).
	var userID interface{}
	if uid, ok := c.Get(appMiddleware.CtxUserID).(int64); ok && uid > 0 {
		userID = uid
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
	err = tx.QueryRow(ctx, `
		INSERT INTO konfirmasi_penjemputan (
			id_ritase, id_seller, id_user, jenis_ritase, ritase_ke,
			jumlah_awb,
			koli_jkt, koli_seg, koli_btn,
			ecer_jkt, ecer_seg, ecer_btn,
			koli_hv_jkt, koli_hv_seg, koli_hv_btn,
			ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			foto_penjemputan_url, catatan
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20
		) RETURNING id
	`, req.IDRitase, sellerID, userID, req.JenisRitase, req.RitaseKe,
		req.DiambilAWB,
		req.DiambilKoliJKT, req.DiambilKoliSEG, req.DiambilKoliBTN,
		req.DiambilEcerJKT, req.DiambilEcerSEG, req.DiambilEcerBTN,
		req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN,
		req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN,
		fotoURL, catatan,
	).Scan(&idKonfirmasi)
	if err != nil {
		log.Printf("[Kapten] gagal insert konfirmasi_penjemputan: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan serah terima")
	}

	// 3. Tautkan semua input hari ini (WIB) yang masih NULL dalam grup ini.
	//    Boleh 0 baris (pengambilan susulan) — bukan error.
	tag, err := tx.Exec(ctx, `
		UPDATE input_kapten
		SET id_ritase = $1, updated_at = NOW()
		WHERE id_seller = $2 AND jenis_ritase = $3 AND ritase_ke = $4
		  AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, req.IDRitase, sellerID, req.JenisRitase, req.RitaseKe)
	if err != nil {
		log.Printf("[Kapten] gagal link input saat confirm-pickup: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menautkan input")
	}
	linked := tag.RowsAffected()

	// 4. Hitung sisa grup hari ini (WIB): Σ input − Σ diambil (akumulasi semua
	//    kejadian hari ini dalam grup, boleh negatif = selisih lebih ambil).
	var inAWB, inKJ, inKS, inKB, inEJ, inES, inEB int
	var inKHJ, inKHS, inKHB, inEHJ, inEHS, inEHB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0)
		FROM input_kapten
		WHERE id_seller = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, sellerID, req.JenisRitase, req.RitaseKe).Scan(
		&inAWB, &inKJ, &inKS, &inKB, &inEJ, &inES, &inEB,
		&inKHJ, &inKHS, &inKHB, &inEHJ, &inEHS, &inEHB)

	var tkAWB, tkKJ, tkKS, tkKB, tkEJ, tkES, tkEB int
	var tkKHJ, tkKHS, tkKHB, tkEHJ, tkEHS, tkEHB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0)
		FROM konfirmasi_penjemputan
		WHERE id_seller = $1 AND jenis_ritase = $2 AND ritase_ke = $3
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, sellerID, req.JenisRitase, req.RitaseKe).Scan(
		&tkAWB, &tkKJ, &tkKS, &tkKB, &tkEJ, &tkES, &tkEB,
		&tkKHJ, &tkKHS, &tkKHB, &tkEHJ, &tkEHS, &tkEHB)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("[Kapten] gagal commit confirm-pickup: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan konfirmasi")
	}

	sisaAWB := inAWB - tkAWB
	sisaKJ, sisaKS, sisaKB := inKJ-tkKJ, inKS-tkKS, inKB-tkKB
	sisaEJ, sisaES, sisaEB := inEJ-tkEJ, inES-tkES, inEB-tkEB
	sisaKHJ, sisaKHS, sisaKHB := inKHJ-tkKHJ, inKHS-tkKHS, inKHB-tkKHB
	sisaEHJ, sisaEHS, sisaEHB := inEHJ-tkEHJ, inES-tkEHS, inEB-tkEB
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
			"total_koli": sisaKJ + sisaKS + sisaKB,
			"total_ecer": sisaEJ + sisaES + sisaEB,
			"total_hv":   sisaKHJ + sisaKHS + sisaKHB + sisaEHJ + sisaEHS + sisaEHB,
		},
	})
}

// GetPendingConfirmations mengambil grup input hari ini (WIB) milik seller kapten
// yang belum ter-link ke ritase — bahan layar "Konfirmasi Pengambilan".
// GET /api/v1/kapten/pending-confirmations
func (h *Handler) GetPendingConfirmations(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT jenis_ritase, ritase_ke, COUNT(*),
		       COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       MAX(created_at)
		FROM input_kapten
		WHERE id_seller = $1 AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		GROUP BY jenis_ritase, ritase_ke
		ORDER BY ritase_ke ASC
	`, sellerID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil pending seller %d: %v", sellerID, err)
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
		var lastAt time.Time
		if err := rows.Scan(&jenis, &ritKe, &count, &awb, &koli, &ecer, &hv,
			&koliJkt, &koliSeg, &koliBtn, &ecerJkt, &ecerSeg, &ecerBtn,
			&koliHvJkt, &koliHvSeg, &koliHvBtn, &ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
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
		})
	}

	return response.OK(c, list)
}

// GetKonfirmasiPenjemputan mengambil riwayat serah terima hari ini (WIB) milik
// seller kapten — lengkap dengan trip, driver, jumlah diambil, foto, catatan.
// GET /api/v1/kapten/konfirmasi-penjemputan
func (h *Handler) GetKonfirmasiPenjemputan(c echo.Context) error {
	sellerID, _ := c.Get(appMiddleware.CtxSellerID).(int64)
	if sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan seller")
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
		       COALESCE(kp.foto_penjemputan_url, ''),
		       COALESCE(kp.catatan, ''),
		       kp.created_at,
		       COALESCE(r.kode_ritase, ''),
		       COALESCE(d.nama_driver, '')
		FROM konfirmasi_penjemputan kp
		LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		WHERE kp.id_seller = $1
		  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		ORDER BY kp.created_at DESC
	`, sellerID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil riwayat penjemputan seller %d: %v", sellerID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id, idRitase int64
		var jenis string
		var ritKe int
		var awb, kj, ks, kb, ej, es, eb, khj, khs, khb, ehj, ehs, ehb int
		var foto, catatan, kode, driver string
		var createdAt time.Time
		if err := rows.Scan(&id, &idRitase, &jenis, &ritKe,
			&awb, &kj, &ks, &kb, &ej, &es, &eb, &khj, &khs, &khb, &ehj, &ehs, &ehb,
			&foto, &catatan, &createdAt, &kode, &driver); err != nil {
			log.Printf("[Kapten] scan riwayat penjemputan error (seller %d): %v", sellerID, err)
			continue
		}
		tKoli := kj + ks + kb
		tEcer := ej + es + eb
		tHV := khj + khs + khb + ehj + ehs + ehb
		list = append(list, map[string]interface{}{
			"id": id, "id_ritase": idRitase,
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"jumlah_awb": awb,
			"koli_jkt":   kj, "koli_seg": ks, "koli_btn": kb,
			"ecer_jkt": ej, "ecer_seg": es, "ecer_btn": eb,
			"koli_hv_jkt": khj, "koli_hv_seg": khs, "koli_hv_btn": khb,
			"ecer_hv_jkt": ehj, "ecer_hv_seg": ehs, "ecer_hv_btn": ehb,
			"total_koli": tKoli, "total_ecer": tEcer, "total_hv": tHV,
			"foto_penjemputan_url": foto,
			"catatan":              catatan,
			"created_at":           createdAt.Format(time.RFC3339),
			"kode_ritase":          kode,
			"nama_driver":          driver,
		})
	}

	// Sisa per grup hari ini (WIB): Σ input − Σ diambil (ringkas + per wilayah).
	sisaGrup := []map[string]interface{}{}
	sRows, err := h.DB.Query(ctx, `
		WITH inp AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb
			FROM input_kapten t
			WHERE t.id_seller = $1
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		),
		amb AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb
			FROM konfirmasi_penjemputan t
			WHERE t.id_seller = $1
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
		       COALESCE(i.w_ehb, 0) - COALESCE(a.w_ehb, 0)
		FROM inp i
		FULL OUTER JOIN amb a USING (jenis_ritase, ritase_ke)
		ORDER BY 2
	`, sellerID)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var jenis string
			var ritKe int
			var inAwb, inKoli, inEcer, inHV, amAwb, amKoli, amEcer, amHV int
			var sAwb, sKj, sKs, sKb, sEj, sEs, sEb int
			var sKhj, sKhs, sKhb, sEhj, sEhs, sEhb int
			if err := sRows.Scan(&jenis, &ritKe,
				&inAwb, &inKoli, &inEcer, &inHV, &amAwb, &amKoli, &amEcer, &amHV,
				&sAwb, &sKj, &sKs, &sKb, &sEj, &sEs, &sEb,
				&sKhj, &sKhs, &sKhb, &sEhj, &sEhs, &sEhb); err != nil {
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
				"sisa_koli_jkt": sKj, "sisa_koli_seg": sKs, "sisa_koli_btn": sKb,
				"sisa_ecer_jkt": sEj, "sisa_ecer_seg": sEs, "sisa_ecer_btn": sEb,
				"sisa_koli_hv_jkt": sKhj, "sisa_koli_hv_seg": sKhs, "sisa_koli_hv_btn": sKhb,
				"sisa_ecer_hv_jkt": sEhj, "sisa_ecer_hv_seg": sEhs, "sisa_ecer_hv_btn": sEhb,
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

// parseTimeMinutes mengkonversi string jam "HH:MM:SS" atau "HH:MM" ke menit dari midnight.
func parseTimeMinutes(s string) int {
	hour, min := 0, 0
	fmt.Sscanf(s, "%d:%d", &hour, &min)
	return hour*60 + min
}

// RegisterRoutes memasang route kapten di grup /api/v1.
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	kapten := g.Group("/kapten", authMW)
	kapten.GET("/my-seller-ritase", h.GetMySellerRitase)
	kapten.POST("/cargo-input", h.PostCargoInput)
	kapten.GET("/seller-info", h.GetSellerInfo)
	kapten.GET("/today-cargo", h.GetTodayCargo)
	kapten.GET("/my-sellers", h.GetMySellers)
	kapten.POST("/select-seller", h.SelectSeller)
	kapten.POST("/confirm-pickup", h.ConfirmPickup)
	kapten.GET("/pending-confirmations", h.GetPendingConfirmations)
	kapten.GET("/konfirmasi-penjemputan", h.GetKonfirmasiPenjemputan)
}
