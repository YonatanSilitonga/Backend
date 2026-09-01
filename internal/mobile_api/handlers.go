package mobile_api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/eventbus"
	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

type APIHandler struct {
	DB         *pgxpool.Pool
	bus        *eventbus.Bus
	trackerKey string
}

func NewAPIHandler(db *pgxpool.Pool, bus *eventbus.Bus, trackerKey string) *APIHandler {
	return &APIHandler{DB: db, bus: bus, trackerKey: trackerKey}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type AppVersionResponse struct {
	VersionCode int    `json:"version_code"`
	VersionName string `json:"version_name"`
	DownloadURL string `json:"download_url"`
	ForceUpdate bool   `json:"force_update"`
	ReleaseNotes string `json:"release_notes"`
}

func (h *APIHandler) GetAppVersion(c echo.Context) error {
	return c.JSON(http.StatusOK, AppVersionResponse{
		VersionCode:  8,
		VersionName:  "1.0.7",
		DownloadURL:  "https://api.controltowerslb.tech/uploads/apk/tower-control-latest.apk",
		ForceUpdate:  false,
		ReleaseNotes: "Pembaruan sistem error handling, perlindungan offline, dan peningkatan keamanan.",
	})
}

func (h *APIHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	identifier := req.Username
	if identifier == "" {
		identifier = req.Email
	}

	if identifier == "" || req.Password == "" {
		return response.Error(c, http.StatusBadRequest, "Username/email dan password wajib diisi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var idUser int
	var dbUsername, dbPassword, role string
	err := h.DB.QueryRow(ctx, `
		SELECT id_user, username, password, role 
		FROM users 
		WHERE LOWER(username) = LOWER($1)
		LIMIT 1
	`, identifier).Scan(&idUser, &dbUsername, &dbPassword, &role)

	if err != nil {
		return response.Error(c, http.StatusUnauthorized, "Username/email atau password salah")
	}

	// Cek password bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
	if err != nil {
		return response.Error(c, http.StatusUnauthorized, "Password salah")
	}

	// Token dummy/JWT sederhana
	token := "token_driver_" + dbUsername

	return response.OK(c, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id_user":  idUser,
			"username": dbUsername,
			"role":     role,
		},
	})
}

func (h *APIHandler) GetSellers(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_seller, COALESCE(kode_seller, ''), nama_seller, COALESCE(alamat, ''), COALESCE(kota, ''), COALESCE(pic, ''), COALESCE(no_hp, '')
		FROM seller
		ORDER BY id_seller ASC
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data seller")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id int
		var kode, nama, alamat, kota, pic, noHp string
		if err := rows.Scan(&id, &kode, &nama, &alamat, &kota, &pic, &noHp); err == nil {
			list = append(list, map[string]interface{}{
				"id":      id,
				"code":    kode,
				"name":    nama,
				"address": alamat,
				"city":    kota,
				"pic":     pic,
				"no_hp":   noHp,
			})
		}
	}

	return response.OK(c, list)
}

func (h *APIHandler) GetDrivers(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_driver, nama_driver, COALESCE(no_hp, ''), status_driver
		FROM driver
		ORDER BY id_driver ASC
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data driver")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id int
		var nama, noHp, status string
		if err := rows.Scan(&id, &nama, &noHp, &status); err == nil {
			list = append(list, map[string]interface{}{
				"id":     id,
				"name":   nama,
				"no_hp":  noHp,
				"status": status,
			})
		}
	}

	return response.OK(c, list)
}

func (h *APIHandler) GetVehicles(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, COALESCE(jenis_kendaraan, ''), COALESCE(kapasitas_kg, 0), status_kendaraan
		FROM kendaraan
		ORDER BY id_kendaraan ASC
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data kendaraan")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, kapasitasKg int
		var plat, jenis, status string
		if err := rows.Scan(&id, &plat, &jenis, &kapasitasKg, &status); err == nil {
			list = append(list, map[string]interface{}{
				"id":          id,
				"plat":        plat,
				"type":        jenis,
				"capacity_kg": kapasitasKg,
				"status":      status,
			})
		}
	}

	return response.OK(c, list)
}

type CreateTrackingRequest struct {
	IDRitase        int64   `json:"id_ritase"`
	IDKendaraan     int64   `json:"id_kendaraan"`
	IDDriver        int64   `json:"id_driver"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Kecepatan       *int    `json:"kecepatan"`
	Arah            *int    `json:"arah"`
	Status          *string `json:"status"`
	JumlahKoli      int     `json:"jumlah_koli"`
	JumlahEcer      int     `json:"jumlah_ecer"`
	JumlahHighValue int     `json:"jumlah_high_value"`
	DurasiDetik     *int    `json:"durasi_detik"`
	NamaLokasi      *string `json:"nama_lokasi"`
	Offline         *bool   `json:"offline"`
}

func (h *APIHandler) PostTracking(c echo.Context) error {
	var req CreateTrackingRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid: "+err.Error())
	}

	// id_driver WAJIB dari token JWT (authMW) — body tidak bisa memalsukan identitas.
	if driverID, ok := c.Get(middleware.CtxDriverID).(int64); ok && driverID > 0 {
		req.IDDriver = driverID
	} else {
		return response.Error(c, http.StatusUnauthorized, "token tidak memuat id_driver yang valid")
	}

	if req.IDKendaraan == 0 {
		return response.Error(c, http.StatusBadRequest, "id_kendaraan wajib diisi")
	}

	// Sinyal "app berhenti" (onDestroy service) → cap kendaraan langsung OFFLINE
	// tanpa mengubah posisi terakhir. last_update di-stamp basi (1 jam lalu)
	// biar query offline langsung membaca true, tanpa nunggu ambang menit.
	if req.Offline != nil && *req.Offline {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		_, err := h.DB.Exec(ctx, `
			UPDATE armada_tracking
			SET last_update = now() - interval '1 hour'
			WHERE id_kendaraan = $1 OR id_driver = $2
		`, req.IDKendaraan, req.IDDriver)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal menandai offline: "+err.Error())
		}
		h.bus.Publish("force_refresh", "mobile_offline_signal")
		return response.OK(c, "ok")
	}

	if req.Status != nil {
		// Bug 4: filter status internal mobile yang tidak boleh tampil ke dashboard.
		// "Background" = heartbeat foreground service (layar mati), "app_stopped" = sinyal keluar.
		// Keduanya hanya update posisi GPS tanpa mengganti status operasional.
		lower := strings.ToLower(*req.Status)
		if lower == "background" || lower == "app_stopped" {
			req.Status = nil // jangan update kolom status di armada_tracking
		} else {
			// Bug 3: normalisasi semua varian "tiba" ke "Tiba" (Title Case) agar
			// konsisten dengan yang disimpan PostTripStatus ke ritase_event.
			switch *req.Status {
			case "mulai_loading":
				s := "Bongkar Muat Barang"
				req.Status = &s
			case "berangkat_gudang":
				s := "Keluar Gudang"
				req.Status = &s
			case "menuju_seller":
				s := "Sedang Menuju"
				req.Status = &s
			case "sampai_gudang", "tiba", "Tiba":
				s := "Tiba"
				req.Status = &s
			case "selesai":
				s := "Selesai"
				req.Status = &s
			default:
				if strings.HasPrefix(*req.Status, "Sedang Menuju") || strings.HasPrefix(*req.Status, "Menuju ") {
					s := "Sedang Menuju"
					req.Status = &s
				} else if strings.HasPrefix(*req.Status, "Tiba di ") ||
					strings.EqualFold(*req.Status, "tiba") {
					// Bug 3: pakai "Tiba" (Title Case) bukan "tiba" (lowercase)
					s := "Tiba"
					req.Status = &s
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var targetRitaseID int64 = req.IDRitase
	if targetRitaseID == 0 {
		_ = h.DB.QueryRow(ctx, `
			SELECT id_ritase 
			FROM ritase 
			WHERE id_driver = $1 OR id_kendaraan = $2 
			ORDER BY id_ritase DESC 
			LIMIT 1
		`, req.IDDriver, req.IDKendaraan).Scan(&targetRitaseID)
	}

	var ritaseID interface{}
	if targetRitaseID != 0 {
		ritaseID = targetRitaseID
	}

	var totalKoli, totalEcer, totalHV int
	if targetRitaseID > 0 {
		_ = h.DB.QueryRow(ctx, `
			SELECT 
				COALESCE(SUM(jumlah_koli), 0),
				COALESCE(SUM(jumlah_ecer), 0),
				COALESCE(SUM(jumlah_high_value), 0)
			FROM ritase_event
			WHERE id_ritase = $1 AND status = 'Bongkar Muat Barang'
		`, targetRitaseID).Scan(&totalKoli, &totalEcer, &totalHV)
	}

	_, err := h.DB.Exec(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, jumlah_koli, jumlah_ecer, jumlah_high_value, nama_lokasi, last_update)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		ON CONFLICT (id_kendaraan) DO UPDATE 
		SET id_ritase = COALESCE(EXCLUDED.id_ritase, armada_tracking.id_ritase),
		    id_driver = EXCLUDED.id_driver,
		    latitude = EXCLUDED.latitude,
		    longitude = EXCLUDED.longitude,
		    kecepatan = EXCLUDED.kecepatan,
		    arah = EXCLUDED.arah,
		    status = COALESCE(NULLIF(EXCLUDED.status, ''), armada_tracking.status),
		    jumlah_koli = $9,
		    jumlah_ecer = $10,
		    jumlah_high_value = $11,
		    nama_lokasi = COALESCE(NULLIF(EXCLUDED.nama_lokasi, ''), armada_tracking.nama_lokasi),
		    last_update = now()
	`, ritaseID, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude, req.Kecepatan, req.Arah, req.Status, totalKoli, totalEcer, totalHV, req.NamaLokasi)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan tracking: "+err.Error())
	}

	h.bus.Publish("force_refresh", "mobile_tracking_update")
	return response.OK(c, "success")
}

// ── JADWAL: helper untuk mencocokkan jam sekarang dengan jendela waktu ritase ──

// hitungWindowMenit menghitung rentang menit [startMin, endMin] (0-1439, endMin bisa > 1440
// kalau window melewati tengah malam) untuk sebuah jadwal ritase, SUDAH TERMASUK toleransi
// mulai 2 jam lebih awal dari jam_mulai resmi.
func hitungWindowMenit(jamMulaiStr, jamSelesaiStr string) (startMin, endMin int, err error) {
	mulai, err := time.Parse("15:04:05", jamMulaiStr)
	if err != nil {
		return 0, 0, err
	}
	selesai, err := time.Parse("15:04:05", jamSelesaiStr)
	if err != nil {
		return 0, 0, err
	}

	mulaiMin := mulai.Hour()*60 + mulai.Minute()
	selesaiMin := selesai.Hour()*60 + selesai.Minute()

	startMin = mulaiMin - 120 // toleransi 2 jam lebih awal
	if startMin < 0 {
		startMin += 1440
	}

	endMin = selesaiMin
	if endMin == 0 {
		// jam_selesai "00:00:00" berarti akhir hari (tengah malam)
		endMin = 1440
	}
	if endMin <= startMin {
		endMin += 1440
	}

	return startMin, endMin, nil
}

// cocokDenganSekarang cek apakah nowMin (0-1439) berada di dalam window [startMin, endMin].
func cocokDenganSekarang(nowMin, startMin, endMin int) bool {
	n := nowMin
	if n < startMin {
		n += 1440
	}
	return n >= startMin && n <= endMin
}

// hitungJamBoleh mengembalikan jam (HH:MM) paling awal driver boleh mulai (2 jam sebelum jam_mulai resmi).
func hitungJamBoleh(jamMulaiStr string) string {
	mulai, err := time.Parse("15:04:05", jamMulaiStr)
	if err != nil {
		return jamMulaiStr
	}
	boleh := mulai.Add(-2 * time.Hour)
	return boleh.Format("15:04")
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
			TO_CHAR(r.jam_mulai, 'HH24:MI:SS'), TO_CHAR(r.jam_selesai, 'HH24:MI:SS')
		FROM ritase r
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.id_driver = $1 AND r.status != 'selesai' AND (r.tanggal = $2 OR r.tanggal IS NULL)
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
			&rc.PlatNomor, &rc.JenisKendaraan, &rc.RitaseKe, &rc.JenisRitase, &rc.JamMulai, &rc.JamSelesai); err == nil {
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

	// 2. Cari kandidat yang jendela waktunya cocok dengan jam sekarang (WIB)
	now := time.Now().In(loc)
	nowMin := now.Hour()*60 + now.Minute()

	var picked *ritaseCandidate
	var isEarly bool
	var nextInfo *ritaseCandidate
	nextStartMin := 999999

	for i := range candidates {
		cand := &candidates[i]

		if cand.JamMulai == nil || cand.JamSelesai == nil {
			if picked == nil {
				picked = cand
				isEarly = false
			}
			continue
		}

		startMin, endMin, errW := hitungWindowMenit(*cand.JamMulai, *cand.JamSelesai)
		if errW != nil {
			continue
		}

		if cocokDenganSekarang(nowMin, startMin, endMin) {
			if picked == nil {
				picked = cand
				n := nowMin
				if n < startMin {
					n += 1440
				}
				mulaiMinAdj := startMin + 120
				isEarly = n < mulaiMinAdj
			}
		} else if picked == nil {
			// Pilih jadwal berikutnya yang paling chronologically dekat (jam_mulai terkecil di masa depan)
			if startMin > nowMin && (nextInfo == nil || startMin < nextStartMin) {
				nextInfo = cand
				nextStartMin = startMin
			}
		}
	}

	if picked == nil {
		resp := map[string]interface{}{
			"has_active_ritase": false,
			"all_completed":     false,
			"schedule_blocked":  true,
			"message":           "Tidak ada jadwal ritase tersisa hari ini.",
		}
		if nextInfo != nil {
			nextSchedule := map[string]interface{}{}
			if nextInfo.JamMulai != nil {
				nextSchedule["jam_mulai"] = *nextInfo.JamMulai
			}
			if nextInfo.JamSelesai != nil {
				nextSchedule["jam_selesai"] = *nextInfo.JamSelesai
			}
			if nextInfo.JenisRitase != nil {
				nextSchedule["jenis_ritase"] = *nextInfo.JenisRitase
			}
			nextSchedule["ritase_ke"] = nextInfo.RitaseKe
			nextSchedule["kode_ritase"] = nextInfo.KodeRitase
			resp["next_schedule"] = nextSchedule
		}
		return response.OK(c, resp)
	}

	idRitase := picked.IDRitase
	assignedKendaraanID := picked.IDKendaraan
	statusRitase := picked.Status
	kodeRitase := picked.KodeRitase
	platNomor := picked.PlatNomor
	jenisKendaraan := picked.JenisKendaraan

	// 3. Ambil Urutan Stop
	rows, err := h.DB.Query(ctx, `
		SELECT 
			rs.id_stop, rs.urutan, rs.jenis_stop, 
			rs.id_seller, rs.id_drop_point, rs.id_gudang, rs.keterangan,
			COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Gudang OG') AS nama_lokasi,
			COALESCE(s.alamat, dp.alamat, g.alamat, 'Gudang Outgoing Utama') AS alamat,
			COALESCE(s.no_hp, '-') AS no_hp,
			COALESCE(s.latitude, g.latitude) AS latitude,
			COALESCE(s.longitude, g.longitude) AS longitude
		FROM ritase_stop rs
		LEFT JOIN seller s ON s.id_seller = rs.id_seller
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
		var idSeller, idDropPoint, idGudang *int64
		var keterangan *string
		var latitude, longitude *float64

		if err := rows.Scan(&idStop, &urutan, &jenisStop, &idSeller, &idDropPoint, &idGudang, &keterangan, &namaLokasi, &alamat, &noHp, &latitude, &longitude); err == nil {
			stop := map[string]interface{}{
				"id_stop":       idStop,
				"urutan":        urutan,
				"jenis_stop":    jenisStop,
				"id_seller":     idSeller,
				"id_drop_point": idDropPoint,
				"id_gudang":     idGudang,
				"keterangan":    keterangan,
				"nama_lokasi":   namaLokasi,
				"alamat":        alamat,
				"no_hp":         noHp,
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

	var countUnfinished int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM ritase
		WHERE id_driver = $1 AND status != 'selesai' AND (tanggal = $2 OR tanggal IS NULL)
	`, idDriver, hariIni).Scan(&countUnfinished)

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
		"is_last_ritase":     countUnfinished <= 1,
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
		"is_early_start":     isEarly,
	}

	if isEarly && picked.JamMulai != nil {
		resp["schedule_warning"] = fmt.Sprintf(
			"Anda memulai Ritase %d lebih awal dari jadwal resmi (jam %s). Diperbolehkan mulai maksimal 2 jam sebelumnya.",
			picked.RitaseKe, (*picked.JamMulai)[0:5],
		)
	}

	return response.OK(c, resp)
}

func (h *APIHandler) StartFreeTrip(c echo.Context) error {
	ctx := c.Request().Context()
	var req struct {
		IdDriver    int64 `json:"id_driver"`
		IdKendaraan int64 `json:"id_kendaraan"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Invalid request payload")
	}

	// Generate kode ritase (e.g. TR-20231015150405-3)
	kodeRitase := fmt.Sprintf("TR-%s-%d", time.Now().Format("20060102150405"), req.IdDriver)

	var idRitase int64
	err := h.DB.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, id_driver, id_kendaraan, status, tanggal, id_drop_point, ritase_ke, created_at)
		VALUES ($1, $2, $3, 'mulai_loading', (now() AT TIME ZONE 'Asia/Jakarta')::date, 1, 1, NOW())
		RETURNING id_ritase
	`, kodeRitase, req.IdDriver, req.IdKendaraan).Scan(&idRitase)

	if err != nil {
		log.Printf("Failed to create free trip: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai perjalanan bebas")
	}

	h.bus.Publish("force_refresh", "mobile_start_free_trip")
	return response.OK(c, map[string]interface{}{
		"id_ritase":   idRitase,
		"kode_ritase": kodeRitase,
	})
}

func (h *APIHandler) AddRitaseStop(c echo.Context) error {
	ctx := c.Request().Context()
	var req struct {
		IdRitase int64 `json:"id_ritase"`
		IdSeller int64 `json:"id_seller"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Invalid request payload")
	}

	// Get max urutan
	var currentMax int
	err := h.DB.QueryRow(ctx, `SELECT COALESCE(MAX(urutan), 0) FROM ritase_stop WHERE id_ritase = $1`, req.IdRitase).Scan(&currentMax)
	if err != nil {
		log.Printf("Failed to get max urutan: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mendapatkan urutan stop")
	}

	newUrutan := currentMax + 1

	var newIdStop int64
	err = h.DB.QueryRow(ctx, `
		INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, id_seller)
		VALUES ($1, $2, 'seller', $3)
		RETURNING id_stop
	`, req.IdRitase, newUrutan, req.IdSeller).Scan(&newIdStop)

	if err != nil {
		log.Printf("Failed to add ritase stop: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menambahkan lokasi")
	}

	h.bus.Publish("force_refresh", "mobile_add_ritase_stop")
	return response.OK(c, map[string]interface{}{
		"id_stop": newIdStop,
		"urutan":  newUrutan,
	})
}

type FinishRitaseReq struct {
	IdRitase int64 `json:"id_ritase"`
}

func (h *APIHandler) FinishRitase(c echo.Context) error {
	var req FinishRitaseReq
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request")
	}

	if req.IdRitase == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase is required")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	_, err := h.DB.Exec(ctx, `UPDATE ritase SET status = 'selesai' WHERE id_ritase = $1`, req.IdRitase)
	if err != nil {
		log.Printf("Failed to update ritase status: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menyelesaikan ritase")
	}

	h.bus.Publish("force_refresh", "mobile_finish_ritase")
	return response.OK(c, "success")
}

type ResetTestRitaseReq struct {
	IdDriver int64 `json:"id_driver"`
}

func (h *APIHandler) ResetTestRitase(c echo.Context) error {
	var req ResetTestRitaseReq
	_ = c.Bind(&req)

	if req.IdDriver == 0 {
		return response.Error(c, http.StatusBadRequest, "id_driver is required")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	_, err := h.DB.Exec(ctx, `UPDATE ritase SET status = 'direncanakan' WHERE id_driver = $1 AND (tanggal = (now() AT TIME ZONE 'Asia/Jakarta')::date OR tanggal IS NULL)`, req.IdDriver)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mereset ritase test")
	}

	h.bus.Publish("force_refresh", "mobile_reset_test_ritase")
	return response.OK(c, "success")
}

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
			COALESCE(EXTRACT(EPOCH FROM (r.jam_selesai - r.jam_mulai))::int, 0),
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
			WHERE status = 'Bongkar Muat Barang'
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
		       COALESCE(s.nama_seller, rs.keterangan, 'Lokasi'),
		       COALESCE(s.alamat, ''),
		       COALESCE(s.no_hp, ''),
		       COALESCE(ev.koli, 0),
		       COALESCE(ev.ecer, 0),
		       COALESCE(ev.hv, 0),
		       COALESCE(rs.foto_manifest_url, ev.photo_url, '')
		FROM ritase_stop rs
		LEFT JOIN seller s ON s.id_seller = rs.id_seller
		LEFT JOIN LATERAL (
			SELECT SUM(jumlah_koli) as koli,
			       SUM(jumlah_ecer) as ecer,
			       SUM(jumlah_high_value) as hv,
			       MAX(foto_manifest_url) as photo_url
			FROM ritase_event re
			WHERE re.id_ritase = rs.id_ritase 
			  AND (re.nama_lokasi = s.nama_seller OR re.status = 'Bongkar Muat Barang')
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

// PostTripStatus mencatat event status perjalanan ke ritase_event dan update armada_tracking.
// Endpoint khusus driver: POST /driver/trip-status
type TripStatusRequest struct {
	IDRitase        int64   `json:"id_ritase"`
	Status          string  `json:"status"`
	NamaLokasi      string  `json:"nama_lokasi"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	JumlahKoli      int     `json:"jumlah_koli"`
	JumlahEcer      int     `json:"jumlah_ecer"`
	JumlahHighValue int     `json:"jumlah_high_value"`
	DurasiDetik     int     `json:"durasi_detik"`
	FotoManifestURL string  `json:"foto_manifest_url"`
}

func (h *APIHandler) PostTripStatus(c echo.Context) error {
	var req TripStatusRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.Status == "" {
		return response.Error(c, http.StatusBadRequest, "status wajib diisi")
	}

	// Terjemahkan status key dari mobile ke teks yang disimpan di DB
	switch req.Status {
	case "mulai_loading":
		req.Status = "Bongkar Muat Barang"
	case "menuju_seller":
		req.Status = "Sedang Menuju"
	case "tiba":
		req.Status = "Tiba"
	case "selesai":
		req.Status = "Selesai"
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var idRitase int64 = req.IDRitase
	if idRitase == 0 {
		if driverID, ok := c.Get(middleware.CtxDriverID).(int64); ok && driverID > 0 {
			_ = h.DB.QueryRow(ctx, `
				SELECT id_ritase FROM ritase
				WHERE id_driver = $1 AND status != 'selesai' AND tanggal = (now() AT TIME ZONE 'Asia/Jakarta')::date
				ORDER BY id_ritase DESC LIMIT 1
			`, driverID).Scan(&idRitase)
		}
	}
	if idRitase == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase tidak ditemukan")
	}

	// Nama lokasi (nullable)
	var namaLokasi interface{}
	if req.NamaLokasi != "" {
		namaLokasi = req.NamaLokasi
	}

	// 0. Durasi stage AUTHORITATIVE dari server — hitung dari selisih created_at
	// event terakhir ke sekarang, JANGAN percaya durasi_detik kiriman mobile
	// (mobile gak bisa memalsukan durasi). Robust juga terhadap layar mati
	// (timer Dart pause di background). Event terakhir = stage yang baru ditutup.
	_, _ = h.DB.Exec(ctx, `
		UPDATE ritase_event
		SET durasi_detik = EXTRACT(EPOCH FROM (now() - created_at))::int
		WHERE id_event = (
			SELECT id_event FROM ritase_event
			WHERE id_ritase = $1
			ORDER BY created_at DESC, id_event DESC
			LIMIT 1
		)
	`, idRitase)

	// 1. Insert ke ritase_event (event baru durasi 0 — dihitung saat stage ditutup)
	var fotoURL interface{}
	if req.FotoManifestURL != "" {
		fotoURL = req.FotoManifestURL
	}
	_, err := h.DB.Exec(ctx, `
		INSERT INTO ritase_event (id_ritase, status, latitude, longitude, nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value, foto_manifest_url)
		VALUES ($1, $2, $3, $4, $5, 0, $6, $7, $8, $9)
	`, idRitase, req.Status, req.Latitude, req.Longitude, namaLokasi, req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue, fotoURL)
	if err != nil {
		log.Printf("[PostTripStatus] Gagal insert ritase_event: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan event: "+err.Error())
	}

	// Update status ritase di tabel ritase ke 'berjalan' jika belum selesai
	if req.Status == "Selesai" {
		_, _ = h.DB.Exec(ctx, `UPDATE ritase SET status = 'selesai' WHERE id_ritase = $1`, idRitase)
	} else {
		_, _ = h.DB.Exec(ctx, `UPDATE ritase SET status = 'berjalan' WHERE id_ritase = $1 AND status != 'selesai'`, idRitase)
	}

	// 2. Hitung total akumulasi muatan yang sedang dibawa di ritase ini (SUM dari semua event Bongkar Muat Barang)
	var totalKoli, totalEcer, totalHV int
	_ = h.DB.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(jumlah_koli), 0),
			COALESCE(SUM(jumlah_ecer), 0),
			COALESCE(SUM(jumlah_high_value), 0)
		FROM ritase_event
		WHERE id_ritase = $1 AND status = 'Bongkar Muat Barang'
	`, idRitase).Scan(&totalKoli, &totalEcer, &totalHV)

	// Bug 10: update total_awb di tabel ritase dari akumulasi muatan event
	// (jumlah_koli dipakai sebagai proxy AWB karena mobile tidak kirim AWB terpisah).
	if req.Status == "Bongkar Muat Barang" {
		_, _ = h.DB.Exec(ctx, `
			UPDATE ritase
			SET total_koli = $1
			WHERE id_ritase = $2 AND status != 'selesai'
		`, totalKoli, idRitase)
	}

	// Update armada_tracking status & nama_lokasi & total muatan akumulasi
	_, _ = h.DB.Exec(ctx, `
		UPDATE armada_tracking
		SET status = $1,
		    nama_lokasi = $2,
		    jumlah_koli = $3,
		    jumlah_ecer = $4,
		    jumlah_high_value = $5,
		    last_update = now()
		WHERE id_ritase = $6
	`, req.Status, namaLokasi, totalKoli, totalEcer, totalHV, idRitase)

	return response.Created(c, map[string]interface{}{
		"id_ritase":   idRitase,
		"status":      req.Status,
		"nama_lokasi": req.NamaLokasi,
	})
}

// UploadManifest menangani upload foto bukti manifest dari driver (multipart form)
// POST /api/v1/driver/upload-manifest
func (h *APIHandler) UploadManifest(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "file foto wajib diunggah")
	}

	src, err := file.Open()
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membaca file foto")
	}
	defer src.Close()

	idRitase, _ := strconv.ParseInt(c.FormValue("id_ritase"), 10, 64)
	namaLokasi := c.FormValue("nama_lokasi")

	uploadDir := "./uploads/manifest"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat direktori upload")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".webp"
	}
	fileName := fmt.Sprintf("manifest_r%d_%d%s", idRitase, time.Now().UnixNano()/1e6, ext)
	dstPath := filepath.Join(uploadDir, fileName)

	dst, err := os.Create(dstPath)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan file foto")
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menulis file foto")
	}

	photoURL := "/uploads/manifest/" + fileName

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	idStop, _ := strconv.ParseInt(c.FormValue("id_stop"), 10, 64)

	// 1. Update ritase_event
	if idRitase > 0 {
		if namaLokasi != "" {
			_, _ = h.DB.Exec(ctx, `
				UPDATE ritase_event
				SET foto_manifest_url = $1
				WHERE id_ritase = $2 AND (nama_lokasi = $3 OR POSITION(LOWER($3) in LOWER(nama_lokasi)) > 0 OR POSITION(LOWER(nama_lokasi) in LOWER($3)) > 0)
			`, photoURL, idRitase, namaLokasi)
		} else {
			_, _ = h.DB.Exec(ctx, `
				UPDATE ritase_event
				SET foto_manifest_url = $1
				WHERE id_event = (
					SELECT id_event FROM ritase_event
					WHERE id_ritase = $2
					ORDER BY created_at DESC, id_event DESC
					LIMIT 1
				)
			`, photoURL, idRitase)
		}
	}

	// 2. Update ritase_stop
	if idStop > 0 {
		_, _ = h.DB.Exec(ctx, `UPDATE ritase_stop SET foto_manifest_url = $1 WHERE id_stop = $2`, photoURL, idStop)
	} else if idRitase > 0 && namaLokasi != "" {
		_, _ = h.DB.Exec(ctx, `
			UPDATE ritase_stop
			SET foto_manifest_url = $1
			WHERE id_stop = (
				SELECT rs.id_stop
				FROM ritase_stop rs
				LEFT JOIN seller s ON s.id_seller = rs.id_seller
				LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
				LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
				WHERE rs.id_ritase = $2
				  AND (
				    COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang) = $3
				    OR POSITION(LOWER(COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER($3)) > 0
				    OR POSITION(LOWER($3) in LOWER(COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0
				  )
				LIMIT 1
			)
		`, photoURL, idRitase, namaLokasi)
	}

	return response.OK(c, map[string]interface{}{
		"message":   "Foto manifest berhasil diunggah",
		"photo_url": photoURL,
	})
}
