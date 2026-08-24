package mobile_api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/pkg/response"

	"github.com/jackc/pgx/v5"
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
	IDDriver    int64       `json:"id_driver"`
	IDKendaraan int64       `json:"id_kendaraan"`
	IDDropPoint int64       `json:"id_drop_point"`
	RitaseKe    int         `json:"ritase_ke"`
	Jenis       string      `json:"jenis_ritase"` // "outgoing" atau "incoming"
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
		1: {"16:00:00", "20:00:00"},
		2: {"20:01:00", "00:00:00"},
		3: {"00:01:00", "03:00:00"},
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

	// 2. Selalu pakai defaultFixedRoutes
	routesToUse := defaultFixedRoutes

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
			jenisPasti = tentukanJenisRitase(fr.IDDriver, fr.RitaseKe)
		}
		jm, _ := ambilJadwal(jenisPasti, fr.RitaseKe)

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
				Keterangan:   fs.Keterangan,
			})
		}

		previewRoutes = append(previewRoutes, PreviewRoute{
			IDDriver:     fr.IDDriver,
			NamaDriver:   driverName,
			IDKendaraan: fr.IDKendaraan,
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
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

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
		jenisPasti := tentukanJenisRitase(route.IDDriver, route.RitaseKe)

		jamMulai, jamSelesai := ambilJadwal(jenisPasti, route.RitaseKe)

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
                jenis_ritase, jam_mulai, jam_selesai
            ) VALUES (
                $1, $2, $3, $4, $5, $6, 'direncanakan', $7, $8, $9
            ) RETURNING id_ritase
        `, kodeRitase, tanggalRitase, route.IDDriver, route.IDKendaraan, finalDropPointID, route.RitaseKe, jenisPasti, jamMulai, jamSelesai).Scan(&idRitase)

		if err != nil {
			return response.Error(c, http.StatusInternalServerError, fmt.Sprintf("Gagal generate ritase D%d: %v", route.IDDriver, err))
		}

		for _, stop := range route.Stops {
			if stop.KolomLokasi == "" {
				if stop.Jenis == "gudang" {
					stop.KolomLokasi = "id_gudang"
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

			query := fmt.Sprintf(`
				INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
				VALUES ($1, $2, $3, $4, $5)
			`, stop.KolomLokasi)

			if _, err := tx.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan); err != nil {
				return response.Error(c, http.StatusInternalServerError, fmt.Sprintf("Gagal menyimpan stop D%d: %v", route.IDDriver, err))
			}
		}
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

// AdminGetRitases Ambil daftar ritase untuk tanggal tertentu (default hari ini)
func (h *APIHandler) AdminGetRitases(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tanggalParam := c.QueryParam("tanggal")
	if tanggalParam == "" {
		// Paksa ke waktu Jakarta
		loc, _ := time.LoadLocation("Asia/Jakarta")
		if loc == nil {
			loc = time.FixedZone("WIB", 7*60*60)
		}
		tanggalParam = time.Now().In(loc).Format("2006-01-02")
	}
	rows, err := h.DB.Query(ctx, `
		SELECT 
			r.id_ritase, r.kode_ritase, TO_CHAR(r.tanggal, 'YYYY-MM-DD') AS tanggal,
			r.id_driver, COALESCE(d.nama_driver, 'Driver #' || r.id_driver) AS nama_driver,
			COALESCE(d.jabatan, 'TRANSPORTER') AS jabatan_driver,
			r.id_kendaraan, COALESCE(k.plat_nomor, 'KD-' || r.id_kendaraan) AS nopol,
			r.id_drop_point, COALESCE(dp.nama_drop_point, 'Gateway #' || r.id_drop_point) AS nama_drop_point,
			r.ritase_ke, r.status, COALESCE(r.jenis_ritase, ''),
			TO_CHAR(r.jam_mulai, 'HH24:MI'), TO_CHAR(r.jam_selesai, 'HH24:MI')
		FROM ritase r
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		WHERE r.tanggal = $1::date
		ORDER BY r.id_driver ASC, r.ritase_ke ASC
	`, tanggalParam)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil daftar ritase: "+err.Error())
	}
	defer rows.Close()

	result := make([]map[string]interface{}, 0)

	for rows.Next() {
		var idRitase, idDriver, idKendaraan, idDropPoint int64
		var kodeRitase, tanggal, namaDriver, jabatanDriver, nopol, namaDropPoint, status, jenisRitase string
		var ritaseKe int
		var jamMulai, jamSelesai *string

		if err := rows.Scan(&idRitase, &kodeRitase, &tanggal, &idDriver, &namaDriver, &jabatanDriver, &idKendaraan, &nopol, &idDropPoint, &namaDropPoint, &ritaseKe, &status, &jenisRitase, &jamMulai, &jamSelesai); err != nil {
			continue
		}

		// Ambil stops untuk ritase ini (pastikan selalu slice kosong, bukan nil)
		stops := make([]map[string]interface{}, 0)
		stopRows, _ := h.DB.Query(ctx, `
			SELECT 
				rs.id_stop, rs.urutan, rs.jenis_stop,
				rs.id_seller, rs.id_drop_point, rs.id_gudang, rs.keterangan,
				COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Lokasi') AS nama_lokasi,
				re.jumlah_koli,
				re.jumlah_ecer,
				re.jumlah_high_value,
				re.durasi_detik
			FROM ritase_stop rs
			LEFT JOIN seller s ON s.id_seller = rs.id_seller
			LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
			LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
			LEFT JOIN LATERAL (
				SELECT ev.jumlah_koli, ev.jumlah_ecer, ev.jumlah_high_value, ev.durasi_detik
				FROM ritase_event ev
				WHERE ev.id_ritase = rs.id_ritase
				  AND (
				    ev.nama_lokasi = COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang)
				    OR (ev.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev.nama_lokasi) in LOWER(COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
				    OR (ev.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER(ev.nama_lokasi)) > 0)
				  )
				ORDER BY ev.created_at DESC, ev.id_event DESC
				LIMIT 1
			) re ON true
			WHERE rs.id_ritase = $1
			ORDER BY rs.urutan ASC
		`, idRitase)

		if stopRows != nil {
			for stopRows.Next() {
				var idStop int64
				var urutan int
				var jenisStop, namaLokasi string
				var idSeller, idDP, idGudang *int64
				var ket *string
				var koli, ecer, highValue, durasiDetik *int

				if err := stopRows.Scan(&idStop, &urutan, &jenisStop, &idSeller, &idDP, &idGudang, &ket, &namaLokasi, &koli, &ecer, &highValue, &durasiDetik); err == nil {
					stops = append(stops, map[string]interface{}{
						"id_stop":           idStop,
						"urutan":            urutan,
						"jenis_stop":        jenisStop,
						"id_seller":         idSeller,
						"id_drop_point":     idDP,
						"id_gudang":         idGudang,
						"keterangan":        ket,
						"nama_lokasi":       namaLokasi,
						"jumlah_koli":       koli,
						"jumlah_ecer":       ecer,
						"jumlah_high_value": highValue,
						"durasi_detik":      durasiDetik,
					})
				}
			}
			stopRows.Close()
		}

		result = append(result, map[string]interface{}{
			"id_ritase":       idRitase,
			"kode_ritase":     kodeRitase,
			"tanggal":         tanggal,
			"id_driver":       idDriver,
			"nama_driver":     namaDriver,
			"jabatan_driver":  jabatanDriver,
			"id_kendaraan":    idKendaraan,
			"nopol":           nopol,
			"id_drop_point":   idDropPoint,
			"nama_drop_point": namaDropPoint,
			"ritase_ke":       ritaseKe,
			"status":          status,
			"jenis_ritase":    jenisRitase,
			"jam_mulai":       jamMulai,
			"jam_selesai":     jamSelesai,
			"stops":           stops,
		})
	}

	return response.OK(c, result)
}

// AdminDeleteRitase Hapus ritase tertentu
func (h *APIHandler) AdminDeleteRitase(c echo.Context) error {
	idParam := c.Param("id")
	idRitase, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || idRitase == 0 {
		return response.Error(c, http.StatusBadRequest, "ID ritase tidak valid")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	_, _ = h.DB.Exec(ctx, "UPDATE armada_tracking SET id_ritase = NULL WHERE id_ritase = $1", idRitase)
	_, _ = h.DB.Exec(ctx, "DELETE FROM ritase_event WHERE id_ritase = $1", idRitase)
	_, _ = h.DB.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase = $1", idRitase)
	_, err = h.DB.Exec(ctx, "DELETE FROM ritase WHERE id_ritase = $1", idRitase)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal menghapus ritase: "+err.Error())
	}

	h.bus.Publish("force_refresh", "admin_delete_ritase")

	return response.OK(c, map[string]interface{}{
		"message": "Ritase berhasil dihapus",
	})
}

type UpdateRitaseRequest struct {
	IDDriver    int64       `json:"id_driver"`
	IDKendaraan int64       `json:"id_kendaraan"`
	IDDropPoint int64       `json:"id_drop_point"`
	RitaseKe    int         `json:"ritase_ke"`
	Status      string      `json:"status"`
	JenisRitase string      `json:"jenis_ritase"`
	Stops       []FixedStop `json:"stops"`
}

// AdminUpdateRitase Handler untuk memperbarui data ritase dan stops
func (h *APIHandler) AdminUpdateRitase(c echo.Context) error {
	idParam := c.Param("id")
	idRitase, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || idRitase == 0 {
		return response.Error(c, http.StatusBadRequest, "ID ritase tidak valid")
	}

	var req UpdateRitaseRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload tidak valid")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	// Kalau jenis_ritase atau ritase_ke berubah, hitung ulang jam_mulai/jam_selesai dari jadwal.
	var jamMulai, jamSelesai interface{}
	hasJadwalBaru := req.JenisRitase != "" && req.RitaseKe != 0
	if hasJadwalBaru {
		jamMulai, jamSelesai = ambilJadwal(req.JenisRitase, req.RitaseKe)
	}

	// Update Header Ritase
	tag, err := tx.Exec(ctx, `
		UPDATE ritase
		SET id_driver = COALESCE(NULLIF($1, 0), id_driver),
		    id_kendaraan = COALESCE(NULLIF($2, 0), id_kendaraan),
		    id_drop_point = COALESCE(NULLIF($3, 0), id_drop_point),
		    ritase_ke = COALESCE(NULLIF($4, 0), ritase_ke),
		    status = COALESCE(NULLIF($5, ''), status),
		    jenis_ritase = COALESCE(NULLIF($6, ''), jenis_ritase),
		    jam_mulai = COALESCE($7, jam_mulai),
		    jam_selesai = COALESCE($8, jam_selesai)
		WHERE id_ritase = $9
	`, req.IDDriver, req.IDKendaraan, req.IDDropPoint, req.RitaseKe, req.Status, req.JenisRitase, jamMulai, jamSelesai, idRitase)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengupdate ritase: "+err.Error())
	}
	if tag.RowsAffected() == 0 {
		return response.Error(c, http.StatusNotFound, "Ritase tidak ditemukan")
	}

	// Jika ada stops baru, perbarui ritase_stop (replace semua stop lama)
	if len(req.Stops) > 0 {
		if _, err := tx.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase = $1", idRitase); err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal membersihkan stop lama: "+err.Error())
		}

		for _, stop := range req.Stops {
			kolom, err := lokasiKolom(stop.Jenis)
			if err != nil {
				return response.Error(c, http.StatusBadRequest, err.Error())
			}
			if stop.IDLokasi <= 0 {
				return response.Error(c, http.StatusBadRequest, "ID lokasi stop wajib diisi")
			}

			query := fmt.Sprintf(`
				INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
				VALUES ($1, $2, $3, $4, $5)
			`, kolom)

			if _, err := tx.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan); err != nil {
				return response.Error(c, http.StatusInternalServerError, "Gagal menyimpan stop: "+err.Error())
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal commit update: "+err.Error())
	}

	h.bus.Publish("force_refresh", "admin_update_ritase")

	return response.OK(c, map[string]interface{}{
		"message": "Jadwal ritase berhasil diperbarui",
	})
}

type CreateRitaseRequest struct {
	Tanggal     string      `json:"tanggal"`
	IDDriver    int64       `json:"id_driver"`
	IDKendaraan int64       `json:"id_kendaraan"`
	IDDropPoint int64       `json:"id_drop_point"`
	RitaseKe    int         `json:"ritase_ke"`
	JenisRitase string      `json:"jenis_ritase"`
	Stops       []FixedStop `json:"stops"`
}

// AdminCreateRitase Handler untuk membuat ritase manual baru
func (h *APIHandler) AdminCreateRitase(c echo.Context) error {
	var req CreateRitaseRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload tidak valid")
	}

	if req.IDDriver == 0 || req.IDKendaraan == 0 || req.IDDropPoint == 0 {
		return response.Error(c, http.StatusBadRequest, "Driver, Kendaraan, dan Gateway wajib dipilih")
	}

	if req.Tanggal == "" {
		loc, _ := time.LoadLocation("Asia/Jakarta")
		if loc == nil {
			loc = time.FixedZone("WIB", 7*60*60)
		}
		req.Tanggal = time.Now().In(loc).Format("2006-01-02")
	}
	if req.RitaseKe == 0 {
		req.RitaseKe = 1
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	todayClean := strings.ReplaceAll(req.Tanggal, "-", "")
	baseKode := fmt.Sprintf("TR-%s-D%d-R%d", todayClean, req.IDDriver, req.RitaseKe)
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

	finalDropPointID := getValidDropPointID(ctx, tx, req.IDDropPoint)

	jamMulai, jamSelesai := ambilJadwal(req.JenisRitase, req.RitaseKe)
	var jenisVal interface{}
	if req.JenisRitase != "" {
		jenisVal = req.JenisRitase
	}

	var idRitase int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase (
			kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status,
			jenis_ritase, jam_mulai, jam_selesai
		) VALUES (
			$1, $2::date, $3, $4, $5, $6, 'direncanakan', $7, $8, $9
		) RETURNING id_ritase
	`, kodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, finalDropPointID, req.RitaseKe, jenisVal, jamMulai, jamSelesai).Scan(&idRitase)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal membuat ritase: "+err.Error())
	}

	for _, stop := range req.Stops {
		kolom, err := lokasiKolom(stop.Jenis)
		if err != nil {
			return response.Error(c, http.StatusBadRequest, err.Error())
		}
		if stop.IDLokasi <= 0 {
			return response.Error(c, http.StatusBadRequest, "ID lokasi stop wajib diisi")
		}

		query := fmt.Sprintf(`
			INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
			VALUES ($1, $2, $3, $4, $5)
		`, kolom)

		if _, err := tx.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan); err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal menyimpan stop: "+err.Error())
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal commit create: "+err.Error())
	}

	h.bus.Publish("force_refresh", "admin_create_ritase")

	return response.Created(c, map[string]interface{}{
		"id_ritase": idRitase,
		"message":   "Jadwal ritase baru berhasil dibuat!",
	})
}

// validKolom cek nama kolom lokasi di ritase_stop (whitelist — hindari SQL injection).
func validKolom(k string) bool {
	switch k {
	case "id_seller", "id_drop_point", "id_gudang":
		return true
	}
	return false
}

// lokasiKolom memetakan jenis_stop → kolom id lokasi di tabel ritase_stop.
func lokasiKolom(jenis string) (string, error) {
	switch jenis {
	case "gudang":
		return "id_gudang", nil
	case "drop_point":
		return "id_drop_point", nil
	case "gateway":
		return "id_drop_point", nil
	case "seller":
		return "id_seller", nil
	}
	return "", fmt.Errorf("jenis_stop tidak dikenal: %q", jenis)
}

// AdminGetMasterOptions Ambil opsi master data (drivers, kendaraan, drop_points, sellers, gudangs)
func (h *APIHandler) AdminGetMasterOptions(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	// 1. Drivers
	drivers := make([]map[string]interface{}, 0)
	dRows, _ := h.DB.Query(ctx, "SELECT id_driver, nama_driver, COALESCE(jabatan, 'TRANSPORTER') FROM driver ORDER BY id_driver ASC")
	if dRows != nil {
		for dRows.Next() {
			var id int64
			var nama, jabatan string
			if err := dRows.Scan(&id, &nama, &jabatan); err == nil {
				drivers = append(drivers, map[string]interface{}{"id_driver": id, "nama_driver": nama, "jabatan": jabatan})
			}
		}
		dRows.Close()
	}

	// 2. Kendaraan
	kendaraans := make([]map[string]interface{}, 0)
	kRows, _ := h.DB.Query(ctx, "SELECT id_kendaraan, plat_nomor, jenis_kendaraan FROM kendaraan ORDER BY id_kendaraan ASC")
	if kRows != nil {
		for kRows.Next() {
			var id int64
			var plat, jenis string
			if err := kRows.Scan(&id, &plat, &jenis); err == nil {
				kendaraans = append(kendaraans, map[string]interface{}{"id_kendaraan": id, "plat_nomor": plat, "jenis_kendaraan": jenis})
			}
		}
		kRows.Close()
	}

	// 3. Drop Points
	dropPoints := make([]map[string]interface{}, 0)
	dpRows, _ := h.DB.Query(ctx, "SELECT id_drop_point, nama_drop_point, kode_dp FROM drop_point ORDER BY id_drop_point ASC")
	if dpRows != nil {
		for dpRows.Next() {
			var id int64
			var nama, kode string
			if err := dpRows.Scan(&id, &nama, &kode); err == nil {
				dropPoints = append(dropPoints, map[string]interface{}{"id_drop_point": id, "nama_drop_point": nama, "kode_dp": kode})
			}
		}
		dpRows.Close()
	}

	// 4. Sellers
	sellers := make([]map[string]interface{}, 0)
	sRows, _ := h.DB.Query(ctx, "SELECT id_seller, nama_seller, kode_seller FROM seller ORDER BY id_seller ASC")
	if sRows != nil {
		for sRows.Next() {
			var id int64
			var nama, kode string
			if err := sRows.Scan(&id, &nama, &kode); err == nil {
				sellers = append(sellers, map[string]interface{}{"id_seller": id, "nama_seller": nama, "kode_seller": kode})
			}
		}
		sRows.Close()
	}

	// 5. Gudang
	gudangs := make([]map[string]interface{}, 0)
	gRows, err := h.DB.Query(ctx, "SELECT id_gudang, nama_gudang FROM gudang ORDER BY id_gudang ASC")
	if err != nil {
		log.Printf("Err query gudang: %v", err)
	}
	if gRows != nil {
		for gRows.Next() {
			var id int64
			var nama string
			if err := gRows.Scan(&id, &nama); err == nil {
				gudangs = append(gudangs, map[string]interface{}{"id_gudang": id, "nama_gudang": nama})
			}
		}
		gRows.Close()
	}

	return response.OK(c, map[string]interface{}{
		"drivers":     drivers,
		"kendaraan":   kendaraans,
		"drop_points": dropPoints,
		"sellers":     sellers,
		"gudangs":     gudangs,
	})
}

// getValidDropPointID memastikan id_drop_point yang dimasukkan selalu terdaftar di database (bukan null / 0).
func getValidDropPointID(ctx context.Context, db interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, requestedID int64) int64 {
	if requestedID > 0 {
		var exists bool
		_ = db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM drop_point WHERE id_drop_point = $1)", requestedID).Scan(&exists)
		if exists {
			return requestedID
		}
	}
	var fallbackID int64
	_ = db.QueryRow(ctx, "SELECT id_drop_point FROM drop_point ORDER BY id_drop_point ASC LIMIT 1").Scan(&fallbackID)
	if fallbackID > 0 {
		return fallbackID
	}
	return 2
}
