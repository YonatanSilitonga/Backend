package mobile_api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

type FixedStop struct {
	Urutan      int    `json:"urutan"`
	Jenis       string `json:"jenis_stop"`
	IDLokasi    int64  `json:"id_lokasi"`
	KolomLokasi string `json:"kolom_lokasi"`
	Keterangan  string `json:"keterangan"`
}

type FixedRoute struct {
	ID          int64       `json:"id"` // ID dari ritase_route_template (0 kalau hardcoded)
	IDDriver    int64       `json:"id_driver"`
	IDKendaraan int64       `json:"id_kendaraan"`
	IDDropPoint int64       `json:"id_drop_point"`
	RitaseKe    int         `json:"ritase_ke"`
	Jenis       string      `json:"jenis_ritase"` // "outgoing" atau "incoming"
	JamMulai    string      `json:"jam_mulai"`
	JamSelesai  string      `json:"jam_selesai"`
	Stops       []FixedStop `json:"stops"`
}

// ── JADWAL RITASE ──
// Tabel jadwal resmi per jenis + ritase_ke. Dipakai untuk mengisi otomatis
// kolom jam_mulai/jam_selesai saat generate/create ritase, dan dipakai lagi
// di sisi mobile (GetActiveRitase) untuk mencocokkan jam sekarang.
type JadwalRitase struct {
	JamMulai   string
	JamSelesai string
}

var jadwalRitaseMap = map[string]map[int]JadwalRitase{
	"outgoing": {
		1: {"00:00:00", "19:59:00"},
		2: {"16:00:00", "23:59:00"},
		3: {"20:00:00", "03:00:00"},
	},
	"incoming": {
		1: {"01:00:00", "04:30:00"},
		2: {"07:00:00", "10:30:00"},
		3: {"13:00:00", "16:30:00"},
		4: {"19:00:00", "22:30:00"},
	},
}

// ambilJadwal mengembalikan jam_mulai/jam_selesai untuk kombinasi jenis + ritase_ke,
// atau (nil, nil) kalau tidak ditemukan di tabel jadwal (mis. jenis kosong/tidak dikenal).
func ambilJadwal(jenis string, ritaseKe int) (jamMulai, jamSelesai interface{}) {
	if m, ok := jadwalRitaseMap[jenis]; ok {
		if j, ok2 := m[ritaseKe]; ok2 {
			return j.JamMulai, j.JamSelesai
		}
	}
	return nil, nil
}

// ambilJadwalDB — versi DB dari ambilJadwal. Query jadwal_ritase_config,
// fallback ke hardcoded kalau DB kosong/error.
func (h *APIHandler) ambilJadwalDB(ctx context.Context, jenis string, ritaseKe int) (jamMulai, jamSelesai interface{}) {
	var jm, js string
	err := h.DB.QueryRow(ctx,
		"SELECT jam_mulai::text, jam_selesai::text FROM jadwal_ritase_config WHERE jenis = $1 AND ritase_ke = $2",
		jenis, ritaseKe,
	).Scan(&jm, &js)
	if err != nil {
		// Fallback ke hardcoded
		return ambilJadwal(jenis, ritaseKe)
	}
	return jm, js
}

// Fixed Route Template Definitions for 1-Click Auto-Generate
var defaultFixedRoutes = []FixedRoute{
	{
		IDDriver: 3, IDKendaraan: 2, IDDropPoint: 2, RitaseKe: 1, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Mulai dari gudang origin"},
			{2, "seller", 3, "id_seller", "Ambil paket di Seller 3"},
			{3, "seller", 1, "id_seller", "Ambil paket di Seller 1"},
			{4, "gateway", 2, "id_drop_point", "Tujuan akhir Gateway 2"},
		},
	},
	{
		IDDriver: 3, IDKendaraan: 2, IDDropPoint: 2, RitaseKe: 2, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 3, "id_seller", "Seller 3"},
			{3, "gudang", 1, "id_gudang", "Gudang 1"},
			{4, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 2, IDKendaraan: 6, IDDropPoint: 2, RitaseKe: 1, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 2, "id_seller", "Seller 2"},
			{3, "gudang", 2, "id_gudang", "Gudang 2"},
			{4, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 2, IDKendaraan: 6, IDDropPoint: 2, RitaseKe: 2, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gateway", 2, "id_drop_point", "Gateway 2"},
			{2, "seller", 2, "id_seller", "Seller 2"},
			{3, "gudang", 2, "id_gudang", "Gudang 2"},
			{4, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 1, IDKendaraan: 11, IDDropPoint: 2, RitaseKe: 2, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gateway", 2, "id_drop_point", "Gateway 2"},
			{2, "seller", 4, "id_seller", "Seller 4"},
			{3, "seller", 1, "id_seller", "Seller 1"},
			{4, "gudang", 1, "id_gudang", "Gudang 1"},
			{5, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 4, IDKendaraan: 15, IDDropPoint: 2, RitaseKe: 2, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 7, "id_seller", "PGA2 Seller 7"},
			{3, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 15, IDKendaraan: 3, IDDropPoint: 2, RitaseKe: 3, Jenis: "outgoing",
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "gateway", 2, "id_drop_point", "Gateway 2"},
			{3, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 11, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 2, Jenis: "incoming",
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
	{
		IDDriver: 11, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 3, Jenis: "incoming",
		Stops: []FixedStop{
			{1, "gudang", 3, "id_gudang", "Gudang 3"},
			{2, "gudang", 2, "id_gudang", "Gudang 2"},
			{3, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
	{
		IDDriver: 10, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 1, Jenis: "incoming",
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
	{
		IDDriver: 10, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 4, Jenis: "incoming",
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
}

type PreviewRoute struct {
	IDDriver     int64         `json:"id_driver"`
	NamaDriver   string        `json:"nama_driver"`
	IDKendaraan  int64         `json:"id_kendaraan"`
	PlatNomor    string        `json:"plat_nomor"`
	RitaseKe     int           `json:"ritase_ke"`
	Jenis        string        `json:"jenis_ritase"`
	JamMulai     string        `json:"jam_mulai"`
	Tanggal      string        `json:"tanggal"`
	TanggalLabel string        `json:"tanggal_label"`
	Stops        []PreviewStop `json:"stops"`
}

type PreviewStop struct {
	Urutan     int    `json:"urutan"`
	JenisStop  string `json:"jenis_stop"`
	IDLokasi   int64  `json:"id_lokasi"`
	NamaLokasi string `json:"nama_lokasi"`
	Keterangan string `json:"keterangan"`
}

// Taruh fungsi ini di admin_ritase_handler.go
func tentukanJenisRitase(idDriver int64, ritaseKe int) string {
	// Driver D11 (Udin): Ritase 2 & 3 adalah incoming
	if idDriver == 11 && (ritaseKe == 2 || ritaseKe == 3) {
		return "incoming"
	}

	// Driver D10 (Gery): Ritase 1 & 4 adalah incoming
	if idDriver == 10 && (ritaseKe == 1 || ritaseKe == 4) {
		return "incoming"
	}

	// Selain kondisi di atas, semuanya dianggap outgoing
	return "outgoing"
}

// tentukanJenisRitaseDB — versi DB dari tentukanJenisRitase.
// Query driver_ritase_jenis, fallback ke hardcoded kalau DB kosong/error.
func (h *APIHandler) tentukanJenisRitaseDB(ctx context.Context, idDriver int64, ritaseKe int) string {
	var jenis string
	err := h.DB.QueryRow(ctx,
		"SELECT jenis FROM driver_ritase_jenis WHERE id_driver = $1 AND ritase_ke = $2",
		idDriver, ritaseKe,
	).Scan(&jenis)
	if err != nil {
		// Fallback ke hardcoded
		return tentukanJenisRitase(idDriver, ritaseKe)
	}
	return jenis
}

// muatRouteTemplateDB — load route templates dari DB, fallback ke hardcoded.
func (h *APIHandler) muatRouteTemplateDB(ctx context.Context) ([]FixedRoute, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT id, id_driver, id_kendaraan, id_drop_point, ritase_ke, COALESCE(jenis_ritase, 'outgoing')
		FROM ritase_route_template
		WHERE aktif = TRUE
		ORDER BY id
	`)
	if err != nil {
		// Fallback ke hardcoded
		return defaultFixedRoutes, nil
	}
	defer rows.Close()

	var routes []FixedRoute
	for rows.Next() {
		var r FixedRoute
		if err := rows.Scan(&r.ID, &r.IDDriver, &r.IDKendaraan, &r.IDDropPoint, &r.RitaseKe, &r.Jenis); err != nil {
			return defaultFixedRoutes, err
		}

		// Load stops untuk route ini
		stopRows, err := h.DB.Query(ctx, `
			SELECT ts.urutan, ts.jenis_stop,
			       CASE WHEN ts.jenis_stop = 'implant'
			            THEN COALESCE(kim.id_titik_implant, ts.id_lokasi)
			            ELSE ts.id_lokasi END,
			       ts.kolom_lokasi, COALESCE(ts.keterangan, '')
			FROM ritase_stop_template ts
			LEFT JOIN LATERAL (
				SELECT id_titik_implant FROM kapten_implant_map
				WHERE id_implant = ts.id_lokasi
				ORDER BY id_user LIMIT 1
			) kim ON ts.jenis_stop = 'implant'
			WHERE ts.id_route_template = $1
			ORDER BY ts.urutan
		`, r.ID)
		if err != nil {
			return defaultFixedRoutes, err
		}

		for stopRows.Next() {
			var s FixedStop
			if err := stopRows.Scan(&s.Urutan, &s.Jenis, &s.IDLokasi, &s.KolomLokasi, &s.Keterangan); err != nil {
				stopRows.Close()
				return defaultFixedRoutes, err
			}
			r.Stops = append(r.Stops, s)
		}
		stopRows.Close()

		routes = append(routes, r)
	}

	if len(routes) == 0 {
		// Fallback ke hardcoded
		return defaultFixedRoutes, nil
	}

	return routes, nil
}

// AdminPreviewGenerateDailyRitase mengembalikan semua rute dari defaultFixedRoutes
// dengan info tanggal (hari ini / besok) berdasarkan jam_mulai.
func (h *APIHandler) AdminPreviewGenerateDailyRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	// --- ZONA WAKTU JAKARTA ---
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	nowWIB := time.Now().In(loc)
	hariIniStr := nowWIB.Format("2006-01-02")
	besokStr := nowWIB.AddDate(0, 0, 1).Format("2006-01-02")

	// 1. Fetch Master Data to Maps
	drivers := make(map[int64]string)
	vehicles := make(map[int64]string)
	gudangs := make(map[int64]string)
	sellers := make(map[int64]string)
	implants := make(map[int64]string)
	dropPoints := make(map[int64]string)

	// Fetch Drivers
	rowsD, _ := h.DB.Query(ctx, "SELECT id_driver, nama_driver FROM driver")
	for rowsD.Next() {
		var id int64
		var name string
		if err := rowsD.Scan(&id, &name); err == nil {
			drivers[id] = name
		}
	}
	rowsD.Close()

	// Fetch Vehicles
	rowsV, _ := h.DB.Query(ctx, "SELECT id_kendaraan, plat_nomor FROM kendaraan")
	for rowsV.Next() {
		var id int64
		var plat string
		if err := rowsV.Scan(&id, &plat); err == nil {
			vehicles[id] = plat
		}
	}
	rowsV.Close()

	// Fetch Gudang
	rowsG, _ := h.DB.Query(ctx, "SELECT id_gudang, nama_gudang FROM gudang")
	for rowsG.Next() {
		var id int64
		var name string
		if err := rowsG.Scan(&id, &name); err == nil {
			gudangs[id] = name
		}
	}
	rowsG.Close()

	// Fetch Seller
	rowsS, _ := h.DB.Query(ctx, "SELECT id_seller, nama_seller FROM seller")
	for rowsS.Next() {
		var id int64
		var name string
		if err := rowsS.Scan(&id, &name); err == nil {
			sellers[id] = name
		}
	}
	rowsS.Close()

	// Fetch Implant
	rowsI, _ := h.DB.Query(ctx, "SELECT id_implant, nama_implant FROM implant")
	for rowsI.Next() {
		var id int64
		var name string
		if err := rowsI.Scan(&id, &name); err == nil {
			implants[id] = name
		}
	}
	rowsI.Close()

	// Fetch Drop Point
	rowsDP, _ := h.DB.Query(ctx, "SELECT id_drop_point, nama_drop_point FROM drop_point")
	for rowsDP.Next() {
		var id int64
		var name string
		if err := rowsDP.Scan(&id, &name); err == nil {
			dropPoints[id] = name
		}
	}
	rowsDP.Close()

	// 2. Load route templates dari DB (fallback ke hardcoded)
	routesToUse, _ := h.muatRouteTemplateDB(ctx)

	// 3. Map Routes to Preview Format
	var previewRoutes []PreviewRoute
	var countHariIni, countBesok int

	for _, fr := range routesToUse {
		driverName := fmt.Sprintf("Driver #%d", fr.IDDriver)
		if name, ok := drivers[fr.IDDriver]; ok {
			driverName = name
		}

		plat := fmt.Sprintf("Kendaraan #%d", fr.IDKendaraan)
		if p, ok := vehicles[fr.IDKendaraan]; ok {
			plat = p
		}

		// Hitung tanggal berdasarkan jam_mulai
		jenisPasti := fr.Jenis
		if jenisPasti == "" {
			jenisPasti = h.tentukanJenisRitaseDB(ctx, fr.IDDriver, fr.RitaseKe)
		}
		jm, _ := h.ambilJadwalDB(ctx, jenisPasti, fr.RitaseKe)

		var tanggalStr, tanggalLabel, jamMulaiStr string
		if jms, ok := jm.(string); ok && len(jms) >= 2 {
			jam, _ := strconv.Atoi(jms[:2])
			jamMulaiStr = jms
			if jam < 7 {
				tanggalStr = besokStr
				tanggalLabel = "Besok"
				countBesok++
			} else {
				tanggalStr = hariIniStr
				tanggalLabel = "Hari Ini"
				countHariIni++
			}
		} else {
			tanggalStr = hariIniStr
			tanggalLabel = "Hari Ini"
			countHariIni++
		}

		previewStops := make([]PreviewStop, 0)
		for _, fs := range fr.Stops {
			locName := fmt.Sprintf("Target #%d", fs.IDLokasi)
			if fs.Jenis == "gudang" {
				if n, ok := gudangs[fs.IDLokasi]; ok {
					locName = n
				}
			} else if fs.Jenis == "seller" {
				if n, ok := sellers[fs.IDLokasi]; ok {
					locName = n
				}
			} else if fs.Jenis == "implant" {
				if n, ok := implants[fs.IDLokasi]; ok {
					locName = n
				}
			} else if fs.Jenis == "drop_point" || fs.Jenis == "gateway" {
				if n, ok := dropPoints[fs.IDLokasi]; ok {
					locName = n
				}
			}

			previewStops = append(previewStops, PreviewStop{
				Urutan:     fs.Urutan,
				JenisStop:  fs.Jenis,
				IDLokasi:   fs.IDLokasi,
				NamaLokasi: locName,
				Keterangan: fs.Keterangan,
			})
		}

		previewRoutes = append(previewRoutes, PreviewRoute{
			IDDriver:     fr.IDDriver,
			NamaDriver:   driverName,
			IDKendaraan:  fr.IDKendaraan,
			PlatNomor:    plat,
			RitaseKe:     fr.RitaseKe,
			Jenis:        jenisPasti,
			JamMulai:     jamMulaiStr,
			Tanggal:      tanggalStr,
			TanggalLabel: tanggalLabel,
			Stops:        previewStops,
		})
	}

	return response.OK(c, map[string]interface{}{
		"total_preview":  len(previewRoutes),
		"total_hari_ini": countHariIni,
		"total_besok":    countBesok,
		"routes":         previewRoutes,
	})
}

// AdminGenerateDailyRitase
// FIX anti-duplikat: sebelum insert, cek dulu apakah kombinasi (id_driver, ritase_ke)
// untuk tanggal hari ini SUDAH ADA. Kalau sudah ada (otomatis berarti statusnya
// 'selesai', karena yang belum selesai sudah dihapus di step DELETE di atas), SKIP.
//
// FIX jadwal: jam_mulai/jam_selesai/jenis_ritase sekarang diisi otomatis dari
// tabel jadwalRitaseMap berdasarkan Jenis + RitaseKe tiap rute template.
func (h *APIHandler) AdminGenerateDailyRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	var req struct {
		Tanggal string       `json:"tanggal"`
		Routes  []FixedRoute `json:"routes"`
	}
	// Try parsing body if provided
	_ = c.Bind(&req)

	targetRoutes := defaultFixedRoutes
	if len(req.Routes) > 0 {
		targetRoutes = req.Routes
	} else {
		// Load dari DB
		dbRoutes, err := h.muatRouteTemplateDB(ctx)
		if err == nil && len(dbRoutes) > 0 {
			targetRoutes = dbRoutes
		}
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	// Extract user ID for audit
	userID, _ := c.Get(middleware.CtxUserID).(int64)

	// ── FIX ZONA WAKTU GOLANG ──
	// Paksa Golang membaca waktu Jakarta untuk variabel string hari ini
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	nowWIB := time.Now().In(loc)

	// Base date: gunakan tanggal dari request, fallback ke server today
	hariIni := nowWIB.Format("2006-01-02")
	hariBesok := nowWIB.AddDate(0, 0, 1).Format("2006-01-02")
	if req.Tanggal != "" {
		parsed, err := time.ParseInLocation("2006-01-02", req.Tanggal, loc)
		if err == nil {
			hariIni = parsed.Format("2006-01-02")
			hariBesok = parsed.AddDate(0, 0, 1).Format("2006-01-02")
		}
	}
	todayStr := strings.ReplaceAll(hariIni, "-", "")

	countGenerated := 0
	countSkipped := 0

	// ── LOG DETAIL GENERATE (TANGGAL, JAM, MENIT, DETIK) ──
	waktuEksekusi := nowWIB.Format("2006-01-02 15:04:05")
	log.Printf("==================================================")
	log.Printf("🚀 [WEB_GENERATE] Admin menjalankan Generate Ritase!")
	log.Printf("📅 Tanggal Format Kode  : TR-%s-...", todayStr)
	log.Printf("⏰ Jam Eksekusi Server  : %s WIB", waktuEksekusi)
	log.Printf("==================================================")

	// Mulai perulangan generate rute
	for _, route := range targetRoutes {
		if route.IDDropPoint <= 0 {
			route.IDDropPoint = 1
		}

		// Validasi status driver — skip driver nonaktif
		var driverStatus string
		_ = tx.QueryRow(ctx, "SELECT COALESCE(status_driver, 'on_duty') FROM driver WHERE id_driver = $1", route.IDDriver).Scan(&driverStatus)
		if driverStatus == "nonaktif" {
			log.Printf("⚠️ [SKIP] Driver %d nonaktif, dilewati", route.IDDriver)
			countSkipped++
			continue
		}

		baseKode := fmt.Sprintf("TR-%s-D%d-R%d", todayStr, route.IDDriver, route.RitaseKe)
		kodeRitase := baseKode
		counter := 1
		for {
			var count int
			_ = tx.QueryRow(ctx, "SELECT COUNT(*) FROM ritase WHERE kode_ritase = $1", kodeRitase).Scan(&count)
			if count == 0 {
				break
			}
			counter++
			kodeRitase = fmt.Sprintf("%s-%d", baseKode, counter)
		}

		finalDropPointID := getValidDropPointID(ctx, tx, route.IDDropPoint)

		// ── FIX jadwal & penentuan jenis: Pastikan backend yang menentukan jenisnya ──
		jenisPasti := h.tentukanJenisRitaseDB(ctx, route.IDDriver, route.RitaseKe)

		// Prioritas: jam_mulai/jam_selesai explicit > auto dari ambilJadwal
		var jamMulai, jamSelesai interface{}
		if route.JamMulai != "" && route.JamSelesai != "" {
			jamMulai = route.JamMulai
			jamSelesai = route.JamSelesai
		} else {
			jamMulai, jamSelesai = h.ambilJadwalDB(ctx, jenisPasti, route.RitaseKe)
		}

		// ── HITUNG TANGGAL: jam_mulai < 07:00 → tanggal besok, else hari ini ──
		tanggalRitase := hariIni
		if jm, ok := jamMulai.(string); ok && len(jm) >= 2 {
			jam, _ := strconv.Atoi(jm[:2])
			if jam < 7 {
				tanggalRitase = hariBesok
			}
		}

		// ── Anti-duplikat: cek apakah ritase untuk tanggal yang sudah dihitung sudah ada ──
		var existingCount int
		_ = tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM ritase
			WHERE tanggal = $1 AND id_driver = $2 AND ritase_ke = $3
		`, tanggalRitase, route.IDDriver, route.RitaseKe).Scan(&existingCount)

		if existingCount > 0 {
			countSkipped++
			continue
		}

		var idRitase int64
		err := tx.QueryRow(ctx, `
            INSERT INTO ritase (
                kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status,
                jenis_ritase, jam_mulai, jam_selesai, created_by
            ) VALUES (
                $1, $2, $3, $4, $5, $6, 'direncanakan', $7, $8, $9, $10
            ) RETURNING id_ritase
        `, kodeRitase, tanggalRitase, route.IDDriver, route.IDKendaraan, finalDropPointID, route.RitaseKe, jenisPasti, jamMulai, jamSelesai, userID).Scan(&idRitase)

		if err != nil {
			return response.Error(c, http.StatusInternalServerError, fmt.Sprintf("Gagal generate ritase D%d: %v", route.IDDriver, err))
		}

		for _, stop := range route.Stops {
			if stop.KolomLokasi == "" {
				if stop.Jenis == "gudang" {
					stop.KolomLokasi = "id_gudang"
				} else if stop.Jenis == "implant" {
					stop.KolomLokasi = "id_implant"
				} else if stop.Jenis == "seller" {
					stop.KolomLokasi = "id_seller"
				} else {
					stop.KolomLokasi = "id_drop_point"
				}
			}
			if !validKolom(stop.KolomLokasi) {
				return response.Error(c, http.StatusBadRequest, fmt.Sprintf("Kolom lokasi stop tidak valid: %q", stop.KolomLokasi))
			}
			if stop.IDLokasi <= 0 {
				return response.Error(c, http.StatusBadRequest, "ID lokasi stop wajib diisi")
			}
			// Validasi FK: pastikan ID lokasi benar-benar ada di tabel terkait
			if err := validasiStopID(ctx, tx, stop.Jenis, stop.IDLokasi); err != nil {
				return response.Error(c, http.StatusBadRequest, fmt.Sprintf("Stop D%d: %v", route.IDDriver, err))
			}

			query := fmt.Sprintf(`
				INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
				VALUES ($1, $2, $3, $4, $5)
			`, stop.KolomLokasi)

			if _, err := tx.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan); err != nil {
				return response.Error(c, http.StatusInternalServerError, fmt.Sprintf("Gagal menyimpan stop D%d: %v", route.IDDriver, err))
			}
		}
		// Penautan input_kapten ke ritase HANYA lewat konfirmasi manual kapten
		// (POST /kapten/confirm-pickup). Generate tidak menyentuh input_kapten.
		countGenerated++
	}

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal commit generate: "+err.Error())
	}

	h.bus.Publish("force_refresh", "admin_generate_ritase")
	log.Printf("✅ [GENERATE_SELESAI] Berhasil dibuat: %d | Dilewati: %d", countGenerated, countSkipped)
	return response.OK(c, map[string]interface{}{
		"total_generated": countGenerated,
		"total_skipped":   countSkipped,
		"message":         fmt.Sprintf("Berhasil generate %d ritase harian (%d rute dilewati karena sudah ada/selesai)!", countGenerated, countSkipped),
	})
}
