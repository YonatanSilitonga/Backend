package mobile_api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// TrackerGPSRequest payload dari perangkat GPS tracker (independen dari HP driver).
// Tracker kirim posisi via HTTP POST — tanpa JWT (device tidak login), tapi wajib
// header X-Tracker-Key yang cocok dengan TRACKER_API_KEY di server.
type TrackerGPSRequest struct {
	IMEI      string   `json:"imei"`  // serial/IMEI perangkat tracker
	Latitude  float64  `json:"lat"`   // latitude
	Longitude float64  `json:"lng"`   // longitude
	Kecepatan *float64 `json:"kecepatan"` // km/jam (opsional)
	Arah      *int     `json:"arah"`      // derajat heading 0-359 (opsional)
}

// PostTrackerGPS menerima posisi dari GPS tracker hardware.
// Alur: validasi key → lookup id_kendaraan dari IMEI → upsert armada_tracking
// (last_update fresh) → publish force_refresh biar dashboard langsung update.
//
// Efek: selama tracker hidup, armada tetap ONLINE meski HP driver mati,
// karena ambang offline (TRACKING_OFFLINE_MIN) dihitung dari last_update.
func (h *APIHandler) PostTrackerGPS(c echo.Context) error {
	// 1. Validasi secret bersama — tracker tidak pakai JWT.
	if h.trackerKey == "" {
		return response.Error(c, http.StatusServiceUnavailable, "tracker belum dikonfigurasi (TRACKER_API_KEY kosong)")
	}
	key := c.Request().Header.Get("X-Tracker-Key")
	if key == "" || key != h.trackerKey {
		return response.Error(c, http.StatusUnauthorized, "X-Tracker-Key tidak valid")
	}

	// 2. Bind payload.
	var req TrackerGPSRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid: "+err.Error())
	}
	req.IMEI = strings.TrimSpace(req.IMEI)
	if req.IMEI == "" {
		return response.Error(c, http.StatusBadRequest, "imei wajib diisi")
	}
	if req.Latitude == 0 && req.Longitude == 0 {
		return response.Error(c, http.StatusBadRequest, "lat/lng wajib diisi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// 3. Lookup kendaraan dari IMEI tracker.
	var idKendaraan int64
	err := h.DB.QueryRow(ctx, `
		SELECT id_kendaraan
		FROM kendaraan_tracker
		WHERE imei = $1 AND aktif = TRUE
	`, req.IMEI).Scan(&idKendaraan)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "IMEI tracker tidak terdaftar atau nonaktif")
	}

	// 4. Cari ritase aktif untuk kendaraan ini (opsional, kalau ada yang sedang jalan).
	var targetRitaseID int64
	_ = h.DB.QueryRow(ctx, `
		SELECT id_ritase
		FROM ritase
		WHERE id_kendaraan = $1 AND status != 'selesai'
		ORDER BY id_ritase DESC
		LIMIT 1
	`, idKendaraan).Scan(&targetRitaseID)

	var ritaseID interface{}
	if targetRitaseID != 0 {
		ritaseID = targetRitaseID
	}

	// 5. Upsert posisi — pola sama dengan PostTracking mobile, TAPI id_driver tidak
	//    di-overwrite (biar identitas driver dari HP tetap terjaga saat HP hidup).
	_, err = h.DB.Exec(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update)
		VALUES ($1, $2, NULL, $3, $4, $5, $6, NULL, now())
		ON CONFLICT (id_kendaraan) DO UPDATE
		SET id_ritase = EXCLUDED.id_ritase,
		    latitude = EXCLUDED.latitude,
		    longitude = EXCLUDED.longitude,
		    kecepatan = EXCLUDED.kecepatan,
		    arah = EXCLUDED.arah,
		    last_update = now()
	`, ritaseID, idKendaraan, req.Latitude, req.Longitude, req.Kecepatan, req.Arah)
	if err != nil {
		log.Printf("tracker gps: gagal menyimpan posisi kendaraan %d: %v", idKendaraan, err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan posisi tracker")
	}

	// 6. Broadcast realtime.
	h.bus.Publish("force_refresh", "tracker_gps_update")
	return response.OK(c, "ok")
}
