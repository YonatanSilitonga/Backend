package mobile_api

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	if err != nil && dbPassword != req.Password {
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
	IDRitase    int64   `json:"id_ritase"`
	IDKendaraan int64   `json:"id_kendaraan"`
	IDDriver    int64   `json:"id_driver"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Kecepatan   *int    `json:"kecepatan"`
	Arah        *int    `json:"arah"`
	Status      *string `json:"status"`
	JumlahKoli      int     `json:"jumlah_koli"`
	JumlahEcer      int     `json:"jumlah_ecer"`
	JumlahHighValue int     `json:"jumlah_high_value"`
	DurasiDetik     *int    `json:"durasi_detik"`
	NamaLokasi  *string `json:"nama_lokasi"`
	// Offline = sinyal "app berhenti" (onDestroy). Backend langsung cap kendaraan
	// offline (last_update di-stamp basi) TANPA mengubah posisi terakhir.
	Offline *bool `json:"offline"`
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
			WHERE id_kendaraan = $1 AND id_driver = $2
		`, req.IDKendaraan, req.IDDriver)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal menandai offline: "+err.Error())
		}
		return response.OK(c, "ok")
	}

	if req.Status != nil {
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
		case "sampai_gudang", "tiba":
			s := "tiba"
			req.Status = &s
		case "selesai":
			s := "Selesai"
			req.Status = &s
		default:
			if strings.HasPrefix(*req.Status, "Sedang Menuju") || strings.HasPrefix(*req.Status, "Menuju ") {
				s := "Sedang Menuju"
				req.Status = &s
			} else if strings.HasPrefix(*req.Status, "Tiba di ") || *req.Status == "tiba" {
				s := "tiba"
				req.Status = &s
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

	_, err := h.DB.Exec(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, jumlah_koli, jumlah_ecer, jumlah_high_value, nama_lokasi, last_update)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		ON CONFLICT (id_kendaraan) DO UPDATE 
		SET id_ritase = EXCLUDED.id_ritase,
		    id_driver = EXCLUDED.id_driver,
		    latitude = EXCLUDED.latitude,
		    longitude = EXCLUDED.longitude,
		    kecepatan = EXCLUDED.kecepatan,
		    arah = EXCLUDED.arah,
		    status = EXCLUDED.status,
		    jumlah_koli = EXCLUDED.jumlah_koli,
		    jumlah_ecer = EXCLUDED.jumlah_ecer,
		    jumlah_high_value = EXCLUDED.jumlah_high_value,
		    nama_lokasi = EXCLUDED.nama_lokasi,
		    last_update = now()
	`, ritaseID, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude, req.Kecepatan, req.Arah, req.Status, req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue, req.NamaLokasi)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan tracking: "+err.Error())
	}

	h.bus.Publish("force_refresh", "mobile_tracking_update")
	return response.OK(c, "success")
}

func (h *APIHandler) GetActiveRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	idDriverParam := c.QueryParam("id_driver")
	idKendaraanParam := c.QueryParam("id_kendaraan")

	idDriver, _ := strconv.ParseInt(idDriverParam, 10, 64)
	idKendaraan, _ := strconv.ParseInt(idKendaraanParam, 10, 64)

	if idDriver == 0 || idKendaraan == 0 {
		return response.Error(c, http.StatusBadRequest, "id_driver dan id_kendaraan harus diisi")
	}

	// 1. Cari Ritase Aktif
	var idRitase int64
	var statusRitase, kodeRitase string
	err := h.DB.QueryRow(ctx, `
		SELECT id_ritase, status, kode_ritase
		FROM ritase
		WHERE id_driver = $1 AND id_kendaraan = $2 AND status != 'selesai' AND tanggal = CURRENT_DATE
		ORDER BY id_ritase ASC
		LIMIT 1
	`, idDriver, idKendaraan).Scan(&idRitase, &statusRitase, &kodeRitase)

	if err != nil {
		var countFinishedToday int
		_ = h.DB.QueryRow(ctx, `
			SELECT COUNT(*) FROM ritase
			WHERE id_driver = $1 AND id_kendaraan = $2 AND status = 'selesai' AND tanggal = CURRENT_DATE
		`, idDriver, idKendaraan).Scan(&countFinishedToday)

		// Jika tidak ada ritase, kembalikan response sukses tapi kosong (menandakan tidak ada rute)
		return response.OK(c, map[string]interface{}{
			"has_active_ritase": false,
			"all_completed":     countFinishedToday > 0,
		})
	}

	// 2. Ambil Urutan Stop (termasuk latitude/longitude per stop untuk ETA)
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
		} else {
			log.Printf("Scan error: %v", err)
		}
	}

	var countUnfinished int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM ritase
		WHERE id_driver = $1 AND id_kendaraan = $2 AND status != 'selesai' AND tanggal = CURRENT_DATE
	`, idDriver, idKendaraan).Scan(&countUnfinished)

	// Baseline waktu stage aktif: created_at event status terakhir ritase ini.
	// Dipakai mobile utk merekonstruksi durasi stage saat app dibuka lagi
	// (jangan mulai dari 0 tiap kali buka app). Null kalau belum ada event.
	var stageStartedAt *time.Time
	_ = h.DB.QueryRow(ctx, `
		SELECT MAX(created_at) FROM ritase_event WHERE id_ritase = $1
	`, idRitase).Scan(&stageStartedAt)

	// Progress resume (anti-manipulasi): jumlah stop yang sudah 'Tiba' = index
	// stop yang sedang dikerjakan, + status stage terakhir. Dipakai mobile supaya
	// app yang di-kill & dibuka lagi LANJUT dari stop yang sama (bukan mulai ulang).
	var currentStopIndex int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM ritase_event WHERE id_ritase = $1 AND status = 'Tiba'
	`, idRitase).Scan(&currentStopIndex)

	var lastStatus *string
	_ = h.DB.QueryRow(ctx, `
		SELECT status FROM ritase_event
		WHERE id_ritase = $1
		ORDER BY created_at DESC, id_event DESC
		LIMIT 1
	`, idRitase).Scan(&lastStatus)

	return response.OK(c, map[string]interface{}{
		"has_active_ritase":  true,
		"id_ritase":          idRitase,
		"kode_ritase":        kodeRitase,
		"status":             statusRitase,
		"is_last_ritase":     countUnfinished <= 1,
		"stops":              stops,
		"stage_started_at":   stageStartedAt,
		"current_stop_index": currentStopIndex,
		"last_status":        lastStatus,
	})
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
		VALUES ($1, $2, $3, 'mulai_loading', CURRENT_DATE, 1, 1, NOW())
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

	_, err := h.DB.Exec(ctx, `UPDATE ritase SET status = 'direncanakan' WHERE id_driver = $1 AND (tanggal = CURRENT_DATE OR tanggal IS NULL)`, req.IdDriver)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mereset ritase test")
	}

	h.bus.Publish("force_refresh", "mobile_reset_test_ritase")
	return response.OK(c, "success")
}

// PostTripStatus mencatat event status perjalanan ke ritase_event dan update armada_tracking.
// Endpoint khusus driver: POST /driver/trip-status
type TripStatusRequest struct {
	IDRitase    int64   `json:"id_ritase"`
	Status      string  `json:"status"`
	NamaLokasi  string  `json:"nama_lokasi"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	JumlahKoli      int     `json:"jumlah_koli"`
	JumlahEcer      int     `json:"jumlah_ecer"`
	JumlahHighValue int     `json:"jumlah_high_value"`
	DurasiDetik     int     `json:"durasi_detik"`
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
				WHERE id_driver = $1 AND status != 'selesai' AND tanggal = CURRENT_DATE
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
	_, err := h.DB.Exec(ctx, `
		INSERT INTO ritase_event (id_ritase, status, latitude, longitude, nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value)
		VALUES ($1, $2, $3, $4, $5, 0, $6, $7, $8)
	`, idRitase, req.Status, req.Latitude, req.Longitude, namaLokasi, req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue)
	if err != nil {
		log.Printf("[PostTripStatus] Gagal insert ritase_event: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan event: "+err.Error())
	}

	// 2. Update armada_tracking status & nama_lokasi
	_, _ = h.DB.Exec(ctx, `
		UPDATE armada_tracking
		SET status = $1, nama_lokasi = $2
		WHERE id_ritase = $3
	`, req.Status, namaLokasi, idRitase)

	return response.Created(c, map[string]interface{}{
		"id_ritase":   idRitase,
		"status":      req.Status,
		"nama_lokasi": req.NamaLokasi,
	})
}
