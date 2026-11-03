package mobile_api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

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

	// id_driver dari token JWT (authMW) dengan fallback lookup/auto-create untuk driver pickup
	if driverID, ok := c.Get(middleware.CtxDriverID).(int64); ok && driverID > 0 {
		req.IDDriver = driverID
	} else if userID, ok := c.Get(middleware.CtxUserID).(int64); ok && userID > 0 {
		var foundDriverID *int64
		var uname string
		_ = h.DB.QueryRow(c.Request().Context(), `SELECT id_driver, username FROM users WHERE id_user = $1`, userID).Scan(&foundDriverID, &uname)
		if foundDriverID != nil && *foundDriverID > 0 {
			req.IDDriver = *foundDriverID
		} else {
			var newDrvID int64
			err := h.DB.QueryRow(c.Request().Context(), `
				INSERT INTO driver (nama_driver, no_hp, status_driver) 
				VALUES ($1, '081200000000', 'aktif') 
				RETURNING id_driver
			`, uname).Scan(&newDrvID)
			if err == nil && newDrvID > 0 {
				_, _ = h.DB.Exec(c.Request().Context(), `UPDATE users SET id_driver = $1 WHERE id_user = $2`, newDrvID, userID)
				req.IDDriver = newDrvID
			} else {
				return response.Error(c, http.StatusUnauthorized, "token tidak memuat id_driver yang valid")
			}
		}
	} else {
		return response.Error(c, http.StatusUnauthorized, "token tidak memuat id_driver yang valid")
	}

	if req.IDKendaraan == 0 {
		req.IDKendaraan = 18 // Fallback default armada pickup
	}

	// Sinyal "app berhenti" (onDestroy service) → cap kendaraan langsung OFFLINE
	// tanpa mengubah posisi terakhir. last_update di-stamp basi (1 jam lalu)
	// biar query offline langsung membaca true, tanpa nunggu ambang menit.
	if req.Offline != nil && *req.Offline {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		_, err := h.DB.Exec(ctx, `
			UPDATE armada_tracking
			SET last_update = now() - interval '2 hours',
			    status = 'Selesai'
			WHERE id_kendaraan = $1 OR id_driver = $2
		`, req.IDKendaraan, req.IDDriver)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal menandai offline: "+err.Error())
		}
		// Hapus juga baris armada_tracking untuk driver pickup (tanpa id_ritase) agar langsung bersih dari peta Fadel
		_, _ = h.DB.Exec(ctx, `
			DELETE FROM armada_tracking
			WHERE (id_kendaraan = $1 OR id_driver = $2) AND (id_ritase IS NULL OR id_ritase = 0)
		`, req.IDKendaraan, req.IDDriver)

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
			case "mulai_loading", "muat_barang", "muat":
				s := "Muat Barang"
				req.Status = &s
			case "mulai_unloading", "bongkar_barang", "bongkar":
				s := "Bongkar Barang"
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
			case "kembali_ke_gudang", "kembali_gudang", "kembali ke gudang":
				s := "Kembali ke Gudang"
				req.Status = &s
			case "selesai":
				s := "Selesai"
				req.Status = &s
			default:
				if strings.EqualFold(*req.Status, "muat barang") || strings.HasPrefix(*req.Status, "Muat") {
					s := "Muat Barang"
					req.Status = &s
				} else if strings.EqualFold(*req.Status, "bongkar barang") || strings.HasPrefix(*req.Status, "Bongkar") {
					s := "Bongkar Barang"
					req.Status = &s
				} else if strings.HasPrefix(*req.Status, "Bongkar Muat") {
					s := "Muat Barang"
					req.Status = &s
				} else if strings.EqualFold(*req.Status, "kembali ke gudang") || strings.HasPrefix(*req.Status, "Kembali") {
					s := "Kembali ke Gudang"
					req.Status = &s
				} else if strings.HasPrefix(*req.Status, "Sedang Menuju") || strings.HasPrefix(*req.Status, "Menuju ") {
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
			WHERE id_ritase = $1 AND status IN ('Bongkar Muat Barang', 'Muat Barang')
		`, targetRitaseID).Scan(&totalKoli, &totalEcer, &totalHV)
	} else {
		if req.JumlahKoli > 0 || req.JumlahEcer > 0 || req.JumlahHighValue > 0 {
			totalKoli = req.JumlahKoli
			totalEcer = req.JumlahEcer
			totalHV = req.JumlahHighValue
		} else {
			_ = h.DB.QueryRow(ctx, `
				WITH last_selesai AS (
					SELECT COALESCE(MAX(id_log), 0) AS max_selesai_id
					FROM driver_pickup_log
					WHERE id_user = (SELECT id_user FROM users WHERE id_driver = $1 LIMIT 1)
					  AND status = 'selesai'
					  AND (tanggal = CURRENT_DATE OR tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				)
				SELECT 
					COALESCE(SUM(dpl.jumlah_barang), 0)::int,
					COALESCE(SUM(dpl.ecer), 0)::int,
					COALESCE(SUM(dpl.high_value), 0)::int
				FROM driver_pickup_log dpl, last_selesai ls
				WHERE dpl.id_user = (SELECT id_user FROM users WHERE id_driver = $1 LIMIT 1)
				  AND (dpl.tanggal = CURRENT_DATE OR dpl.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				  AND dpl.id_log > ls.max_selesai_id
			`, req.IDDriver).Scan(&totalKoli, &totalEcer, &totalHV)
		}
	}

	var namaLokasiVal interface{}
	if req.NamaLokasi != nil && *req.NamaLokasi != "" {
		namaLokasiVal = *req.NamaLokasi
	} else if targetRitaseID > 0 {
		var lastEventLoc string
		_ = h.DB.QueryRow(ctx, `
			SELECT nama_lokasi FROM ritase_event
			WHERE id_ritase = $1 AND nama_lokasi IS NOT NULL AND nama_lokasi != ''
			ORDER BY id_event DESC LIMIT 1
		`, targetRitaseID).Scan(&lastEventLoc)
		if lastEventLoc != "" {
			namaLokasiVal = lastEventLoc
		}
	}

	_, err := h.DB.Exec(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, jumlah_koli, jumlah_ecer, jumlah_high_value, nama_lokasi, stopped_since, last_update)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		        CASE WHEN $6::int IS NULL OR $6::int <= 5 THEN now() ELSE NULL END,
		        now())
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
		    nama_lokasi = COALESCE($12, armada_tracking.nama_lokasi),
		    stopped_since = CASE
		        WHEN $6::int IS NULL OR $6::int <= 5 THEN COALESCE(armada_tracking.stopped_since, now())
		        ELSE NULL
		    END,
		    last_update = now()
	`, ritaseID, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude, req.Kecepatan, req.Arah, req.Status, totalKoli, totalEcer, totalHV, namaLokasiVal)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan tracking: "+err.Error())
	}

	// Jika driver berganti kendaraan (misal dari kendaraan default fallback ke kendaraan terpilih),
	// bersihkan baris armada_tracking kendaraan lain untuk driver ini agar tidak muncul ganda (ghost vehicle).
	// Fix: kondisi sebelumnya hanya hapus kalau id_ritase NULL/0 — kendaraan lama yang ritasenya sudah
	// 'selesai' tetap lolos. Sekarang hapus juga kalau ritase sudah selesai.
	if req.IDDriver > 0 && req.IDKendaraan > 0 {
		_, _ = h.DB.Exec(ctx, `
			DELETE FROM armada_tracking at_ghost
			WHERE at_ghost.id_driver = $1
			  AND at_ghost.id_kendaraan != $2
			  AND (
			      at_ghost.id_ritase IS NULL
			      OR at_ghost.id_ritase = 0
			      OR EXISTS (
			          SELECT 1 FROM ritase r
			          WHERE r.id_ritase = at_ghost.id_ritase
			            AND r.status = 'selesai'
			      )
			  )
		`, req.IDDriver, req.IDKendaraan)
	}

	// Simpan GPS history — setiap titik GPS yang valid dicatat untuk rute di peta.
	if targetRitaseID > 0 && req.Latitude != 0 && req.Longitude != 0 {
		_, _ = h.DB.Exec(ctx, `
			INSERT INTO armada_gps_history (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, targetRitaseID, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude, req.Kecepatan)
	}

	// Update last_open driver agar tidak dianggap "offline" oleh dashboard
	// selama GPS masih dikirim — meskipun last_login sudah NULL/expired.
	// Penting untuk driver_pickup yang background tracking tetap jalan walau app di-minimize.
	if req.IDDriver > 0 {
		_, _ = h.DB.Exec(ctx, `
			UPDATE users SET last_open = now()
			WHERE id_driver = $1
		`, req.IDDriver)
	}

	h.bus.Publish("force_refresh", "mobile_tracking_update")
	return response.OK(c, "success")
}
