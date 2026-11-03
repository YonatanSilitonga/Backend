package mobile_api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)


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
		IdRitase  int64 `json:"id_ritase"`
		IdSeller  int64 `json:"id_seller"`
		IdImplant int64 `json:"id_implant"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Invalid request payload")
	}
	if req.IdRitase <= 0 || (req.IdSeller <= 0 && req.IdImplant <= 0) {
		return response.Error(c, http.StatusBadRequest, "id_ritase dan id_seller/id_implant harus diisi")
	}
	jenisStop := "seller"
	if req.IdImplant > 0 {
		jenisStop = "implant"
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
		INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, id_seller, id_implant)
		VALUES ($1, $2, $3, NULLIF($4, 0), NULLIF($5, 0))
		RETURNING id_stop
	`, req.IdRitase, newUrutan, jenisStop, req.IdSeller, req.IdImplant).Scan(&newIdStop)

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

	_, err := h.DB.Exec(ctx, `UPDATE ritase SET status = 'selesai', updated_at = NOW(), updated_by = (SELECT created_by FROM ritase WHERE id_ritase = $1) WHERE id_ritase = $1`, req.IdRitase)
	if err != nil {
		log.Printf("Failed to update ritase status: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menyelesaikan ritase")
	}

	// Close durasi event terakhir supaya gak 0 detik
	_, _ = h.DB.Exec(ctx, `
		UPDATE ritase_event
		SET durasi_detik = EXTRACT(EPOCH FROM (now() - created_at))::int
		WHERE id_event = (
			SELECT id_event FROM ritase_event
			WHERE id_ritase = $1
			ORDER BY created_at DESC, id_event DESC
			LIMIT 1
		)
	`, req.IdRitase)

	// Pastikan event 'Selesai' tercatat di ritase_event jika belum ada
	var countSelesai int
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM ritase_event WHERE id_ritase = $1 AND status = 'Selesai'`, req.IdRitase).Scan(&countSelesai)
	if countSelesai == 0 {
		var lastLoc string
		var lastLat, lastLng float64
		_ = h.DB.QueryRow(ctx, `
			SELECT COALESCE(nama_lokasi, ''), COALESCE(latitude, 0), COALESCE(longitude, 0)
			FROM ritase_event WHERE id_ritase = $1 ORDER BY id_event DESC LIMIT 1
		`, req.IdRitase).Scan(&lastLoc, &lastLat, &lastLng)

		if lastLoc == "" {
			_ = h.DB.QueryRow(ctx, `
				SELECT COALESCE(i.nama_implant, dp.nama_drop_point, g.nama_gudang, s.nama_seller, 'Selesai')
				FROM ritase_stop rs
				LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
				LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
				LEFT JOIN seller s ON s.id_seller = rs.id_seller
				LEFT JOIN implant i ON i.id_implant = rs.id_implant
				WHERE rs.id_ritase = $1
				ORDER BY rs.urutan DESC LIMIT 1
			`, req.IdRitase).Scan(&lastLoc)
		}

		_, _ = h.DB.Exec(ctx, `
			INSERT INTO ritase_event (id_ritase, status, latitude, longitude, nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value)
			VALUES ($1, 'Selesai', $2, $3, $4, 0, 0, 0, 0)
		`, req.IdRitase, lastLat, lastLng, lastLoc)
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

	_, err := h.DB.Exec(ctx, `UPDATE ritase SET status = 'direncanakan', updated_at = NOW() WHERE id_driver = $1 AND (tanggal = (now() AT TIME ZONE 'Asia/Jakarta')::date OR tanggal IS NULL)`, req.IdDriver)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mereset ritase test")
	}

	h.bus.Publish("force_refresh", "mobile_reset_test_ritase")
	return response.OK(c, "success")
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
	switch strings.ToLower(req.Status) {
	case "mulai_loading", "muat_barang", "muat", "muat barang":
		req.Status = "Muat Barang"
	case "mulai_unloading", "bongkar_barang", "bongkar", "bongkar barang":
		req.Status = "Bongkar Barang"
	case "bongkar muat barang":
		req.Status = "Muat Barang"
	case "menuju_seller", "sedang menuju":
		req.Status = "Sedang Menuju"
	case "kembali_ke_gudang", "kembali_gudang", "kembali ke gudang":
		req.Status = "Kembali ke Gudang"
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

	// Serialize transitions for a trip so saving cargo cannot race the next stage.
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi status")
	}
	defer tx.Rollback(ctx)
	var lockedID int64
	if err = tx.QueryRow(ctx, "SELECT id_ritase FROM ritase WHERE id_ritase = $1 FOR UPDATE", idRitase).Scan(&lockedID); err != nil {
		return response.Error(c, http.StatusBadRequest, "ritase tidak ditemukan")
	}
	merged := false
	if (req.Status == "Muat Barang" || req.Status == "Bongkar Barang") && req.NamaLokasi != "" {
		tag, updateErr := tx.Exec(ctx, `UPDATE ritase_event SET
		 jumlah_koli = GREATEST(COALESCE(jumlah_koli,0), $2),
		 jumlah_ecer = GREATEST(COALESCE(jumlah_ecer,0), $3),
		 jumlah_high_value = GREATEST(COALESCE(jumlah_high_value,0), $4),
		 foto_manifest_url = COALESCE(NULLIF($5,''), foto_manifest_url)
		 WHERE id_event = (SELECT id_event FROM ritase_event WHERE id_ritase=$1 ORDER BY created_at DESC,id_event DESC LIMIT 1)
		 AND status=$6 AND nama_lokasi=$7`, idRitase, req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue, req.FotoManifestURL, req.Status, req.NamaLokasi)
		if updateErr != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal memperbarui muatan")
		}
		merged = tag.RowsAffected() > 0
	}
	if !merged {
		// 0. Durasi stage AUTHORITATIVE dari server — hitung dari selisih created_at
		// event terakhir ke sekarang, JANGAN percaya durasi_detik kiriman mobile
		// (mobile gak bisa memalsukan durasi). Robust juga terhadap layar mati
		// (timer Dart pause di background). Event terakhir = stage yang baru ditutup.
		_, _ = tx.Exec(ctx, `
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

		// Ambil driver_id dari JWT
		var inputByID interface{}
		if did, ok := c.Get(middleware.CtxDriverID).(int64); ok && did > 0 {
			inputByID = did
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO ritase_event (id_ritase, status, latitude, longitude, nama_lokasi, durasi_detik,
			                          koli_jkt, koli_seg, koli_btn, ecer_jkt, ecer_seg, ecer_btn,
			                          koli_hv_jkt, koli_hv_seg, koli_hv_btn, ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
			                          jumlah_koli, jumlah_ecer, jumlah_high_value,
			                          foto_manifest_url, input_by, input_by_id)
			VALUES ($1, $2, $3, $4, $5, 0,
			        0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			        $6, $7, $8, $9, 'driver', $10)
		`, idRitase, req.Status, req.Latitude, req.Longitude, namaLokasi, req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue, fotoURL, inputByID)
		if err != nil {
			log.Printf("[PostTripStatus] Gagal insert ritase_event: %v", err)
			return response.Error(c, http.StatusInternalServerError, "gagal menyimpan event: "+err.Error())
		}
	}

	// Update status ritase di tabel ritase ke 'berjalan' jika belum selesai
	if req.Status == "Selesai" {
		_, _ = tx.Exec(ctx, `UPDATE ritase SET status = 'selesai', updated_at = NOW(), updated_by = (SELECT created_by FROM ritase WHERE id_ritase = $1) WHERE id_ritase = $1`, idRitase)
	} else {
		_, _ = tx.Exec(ctx, `UPDATE ritase SET status = 'berjalan', updated_at = NOW(), updated_by = (SELECT created_by FROM ritase WHERE id_ritase = $1) WHERE id_ritase = $1 AND status != 'selesai'`, idRitase)
	}

	// Isi jam_berangkat otomatis saat status pertama kali berubah dari direncanakan
	_, _ = tx.Exec(ctx, `
		UPDATE ritase SET jam_berangkat = NOW()
		WHERE id_ritase = $1 AND jam_berangkat IS NULL AND status != 'direncanakan'
	`, idRitase)

	// Isi jam_tiba otomatis saat driver tiba atau selesai
	if req.Status == "Tiba" || req.Status == "Selesai" {
		_, _ = tx.Exec(ctx, `
			UPDATE ritase SET jam_tiba = NOW()
			WHERE id_ritase = $1 AND jam_tiba IS NULL
		`, idRitase)
	}

	// 2. Hitung total akumulasi muatan yang sedang dibawa di ritase ini (SUM dari semua event Muat Barang & Bongkar Muat Barang lama)
	var totalKoli, totalEcer, totalHV int
	_ = tx.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(jumlah_koli), 0),
			COALESCE(SUM(jumlah_ecer), 0),
			COALESCE(SUM(jumlah_high_value), 0)
		FROM ritase_event
		WHERE id_ritase = $1 AND status IN ('Bongkar Muat Barang', 'Muat Barang')
	`, idRitase).Scan(&totalKoli, &totalEcer, &totalHV)

	// Bug 10: update total_awb di tabel ritase dari akumulasi muatan event
	// (jumlah_koli dipakai sebagai proxy AWB karena mobile tidak kirim AWB terpisah).
	if req.Status == "Muat Barang" || req.Status == "Bongkar Muat Barang" || req.Status == "Bongkar Barang" {
		_, _ = tx.Exec(ctx, `
			UPDATE ritase
			SET total_koli = $1
			WHERE id_ritase = $2 AND status != 'selesai'
		`, totalKoli, idRitase)
	}

	// Cari id_kendaraan dan id_driver dari ritase ini
	var idKendaraan, idDriver int64
	_ = tx.QueryRow(ctx, `SELECT id_kendaraan, id_driver FROM ritase WHERE id_ritase = $1`, idRitase).Scan(&idKendaraan, &idDriver)

	// Update armada_tracking status & nama_lokasi & total muatan akumulasi
	_, _ = tx.Exec(ctx, `
		UPDATE armada_tracking
		SET id_ritase = $1,
		    status = $2,
		    nama_lokasi = $3,
		    jumlah_koli = $4,
		    jumlah_ecer = $5,
		    jumlah_high_value = $6,
		    last_update = now()
		WHERE id_ritase = $1 OR (id_kendaraan = $7 AND $7 > 0) OR (id_driver = $8 AND $8 > 0)
	`, idRitase, req.Status, namaLokasi, totalKoli, totalEcer, totalHV, idKendaraan, idDriver)

	if err = tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan transaksi status")
	}
	return response.Created(c, map[string]interface{}{
		"id_ritase":   idRitase,
		"status":      req.Status,
		"nama_lokasi": req.NamaLokasi,
	})
}