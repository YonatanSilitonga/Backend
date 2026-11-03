package mobile_api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
)

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
		WITH muatan AS (
			SELECT id_ritase,
			       sum(jumlah_koli) AS koli,
			       sum(jumlah_ecer) AS ecer,
			       sum(jumlah_high_value) AS hv
			FROM ritase_event
			WHERE status IN ('Bongkar Muat Barang', 'Muat Barang')
			GROUP BY id_ritase
		)
		SELECT 
			r.id_ritase, r.kode_ritase, TO_CHAR(r.tanggal, 'YYYY-MM-DD') AS tanggal,
			r.id_driver, COALESCE(d.nama_driver, 'Driver #' || r.id_driver) AS nama_driver,
			COALESCE(d.jabatan, 'TRANSPORTER') AS jabatan_driver,
			r.id_kendaraan, COALESCE(k.plat_nomor, 'KD-' || r.id_kendaraan) AS nopol,
			r.id_drop_point, COALESCE(dp.nama_drop_point, 'Gateway #' || r.id_drop_point) AS nama_drop_point,
			r.ritase_ke, r.status, COALESCE(r.jenis_ritase, ''),
			TO_CHAR(r.jam_mulai, 'HH24:MI'), TO_CHAR(r.jam_selesai, 'HH24:MI'),
			COALESCE(TO_CHAR((SELECT MIN(e.created_at) + interval '7 hours' FROM ritase_event e WHERE e.id_ritase = r.id_ritase), 'HH24:MI'), ''),
			COALESCE(TO_CHAR((SELECT MAX(e.created_at) + interval '7 hours' FROM ritase_event e WHERE e.id_ritase = r.id_ritase AND e.status = 'Selesai'), 'HH24:MI'), ''),
			COALESCE(m.koli, 0), COALESCE(m.ecer, 0), COALESCE(m.hv, 0),
			COALESCE(r.created_at::text, ''), COALESCE(r.updated_at::text, ''),
			COALESCE(u1.username, '') AS created_by_name,
			COALESCE(u2.username, '') AS updated_by_name
		FROM ritase r
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
		LEFT JOIN users u1 ON u1.id_user = r.created_by
		LEFT JOIN users u2 ON u2.id_user = r.updated_by
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
		var jamBerangkat, jamTiba *string
		var totalKoli, totalEcer, totalHV int
		var createdAt, updatedAt *string
		var createdByName, updatedByName string

		if err := rows.Scan(&idRitase, &kodeRitase, &tanggal, &idDriver, &namaDriver, &jabatanDriver, &idKendaraan, &nopol, &idDropPoint, &namaDropPoint, &ritaseKe, &status, &jenisRitase, &jamMulai, &jamSelesai, &jamBerangkat, &jamTiba, &totalKoli, &totalEcer, &totalHV, &createdAt, &updatedAt, &createdByName, &updatedByName); err != nil {
			continue
		}

		// Ambil stops untuk ritase ini (pastikan selalu slice kosong, bukan nil)
		stops := make([]map[string]interface{}, 0)
		stopRows, _ := h.DB.Query(ctx, `
			SELECT 
				rs.id_stop, rs.urutan,
				CASE WHEN rs.id_implant IS NOT NULL THEN 'implant' ELSE rs.jenis_stop END AS jenis_stop,
				rs.id_seller, rs.id_implant, rs.id_drop_point, rs.id_gudang, rs.keterangan,
				COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Lokasi') AS nama_lokasi,
				re.jumlah_koli,
				re.jumlah_ecer,
				re.jumlah_high_value,
				re.durasi_detik,
				COALESCE(re.foto_manifest_url, rs.foto_manifest_url) AS foto_manifest_url
			FROM ritase_stop rs
			LEFT JOIN seller s ON s.id_seller = rs.id_seller
			LEFT JOIN implant i ON i.id_implant = rs.id_implant
			LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
			LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
			LEFT JOIN LATERAL (
				SELECT 
					COALESCE(MAX(ev.jumlah_koli), 0) AS jumlah_koli,
					COALESCE(MAX(ev.jumlah_ecer), 0) AS jumlah_ecer,
					COALESCE(MAX(ev.jumlah_high_value), 0) AS jumlah_high_value,
					COALESCE((
						SELECT SUM(ev2.durasi_detik)
						FROM ritase_event ev2 
						WHERE ev2.id_ritase = rs.id_ritase 
						AND ev2.status IN ('Tiba', 'Bongkar Muat Barang', 'Muat Barang', 'Bongkar Barang')
						AND (
							ev2.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
							OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev2.nama_lokasi) IN LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
							OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) IN LOWER(ev2.nama_lokasi)) > 0)
						)
					), 0) AS durasi_detik,
					(SELECT ev2.foto_manifest_url FROM ritase_event ev2
					 WHERE ev2.id_ritase = rs.id_ritase
					   AND ev2.foto_manifest_url IS NOT NULL AND ev2.foto_manifest_url != ''
					   AND (
					     ev2.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
					     OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev2.nama_lokasi) in LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
					     OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER(ev2.nama_lokasi)) > 0)
					   )
					 LIMIT 1
					) AS foto_manifest_url
				FROM ritase_event ev
				WHERE ev.id_ritase = rs.id_ritase
				  AND (
				    ev.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
				    OR (ev.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev.nama_lokasi) in LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
				    OR (ev.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER(ev.nama_lokasi)) > 0)
				  )
			) re ON true
			WHERE rs.id_ritase = $1
			ORDER BY rs.urutan ASC
		`, idRitase)

		if stopRows != nil {
			for stopRows.Next() {
				var idStop int64
				var urutan int
				var jenisStop, namaLokasi string
				var idSeller, idImplant, idDP, idGudang *int64
				var ket *string
				var koli, ecer, highValue, durasiDetik *int
				var fotoManifestURL *string

				if err := stopRows.Scan(&idStop, &urutan, &jenisStop, &idSeller, &idImplant, &idDP, &idGudang, &ket, &namaLokasi, &koli, &ecer, &highValue, &durasiDetik, &fotoManifestURL); err == nil {
					stops = append(stops, map[string]interface{}{
						"id_stop":           idStop,
						"urutan":            urutan,
						"jenis_stop":        jenisStop,
						"id_seller":         idSeller,
						"id_implant":        idImplant,
						"id_drop_point":     idDP,
						"id_gudang":         idGudang,
						"keterangan":        ket,
						"nama_lokasi":       namaLokasi,
						"jumlah_koli":       koli,
						"jumlah_ecer":       ecer,
						"jumlah_high_value": highValue,
						"durasi_detik":      durasiDetik,
						"foto_manifest_url": fotoManifestURL,
					})
				}
			}
			stopRows.Close()
		}

		result = append(result, map[string]interface{}{
			"id_ritase":        idRitase,
			"kode_ritase":      kodeRitase,
			"tanggal":          tanggal,
			"id_driver":        idDriver,
			"nama_driver":      namaDriver,
			"jabatan_driver":   jabatanDriver,
			"id_kendaraan":     idKendaraan,
			"nopol":            nopol,
			"id_drop_point":    idDropPoint,
			"nama_drop_point":  namaDropPoint,
			"ritase_ke":        ritaseKe,
			"status":           status,
			"jenis_ritase":     jenisRitase,
			"jam_mulai":        jamMulai,
			"jam_selesai":      jamSelesai,
			"jam_berangkat":    jamBerangkat,
			"jam_tiba":         jamTiba,
			"total_koli":       totalKoli,
			"total_eceran":     totalEcer,
			"total_high_value": totalHV,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
			"created_by_name":  createdByName,
			"updated_by_name":  updatedByName,
			"stops":            stops,
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

	// ── Cek status ritase ──
	var delStatus string
	err = h.DB.QueryRow(ctx, "SELECT status FROM ritase WHERE id_ritase = $1", idRitase).Scan(&delStatus)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Ritase tidak ditemukan")
	}

	// Cek apakah user adalah whisnu (hak eksklusif khusus akun whisnu)
	userID, _ := c.Get(middleware.CtxUserID).(int64)
	var currentUsername string
	_ = h.DB.QueryRow(ctx, "SELECT username FROM users WHERE id_user = $1", userID).Scan(&currentUsername)
	isWhisnu := strings.ToLower(currentUsername) == "whisnu"

	if !isWhisnu {
		if delStatus == "berjalan" {
			return response.Error(c, http.StatusBadRequest, "Ritase sedang berjalan. Tidak bisa dihapus.")
		}
		if delStatus == "selesai" {
			return response.Error(c, http.StatusBadRequest, "Ritase sudah selesai. Tidak bisa dihapus.")
		}
	}

	// ── Opsi A (preserve data): tolak hapus jika masih ada data anak ──
	// input_kapten.id_ritase terisi lewat konfirmasi manual kapten dan dijaga
	// FK RESTRICT (input_kapten_id_ritase_fkey). Jangan CASCADE agar bukti
	// muatan kapten tidak ikut hilang.
	var nInputKapten int
	if err := h.DB.QueryRow(ctx, "SELECT COUNT(*) FROM input_kapten WHERE id_ritase = $1", idRitase).Scan(&nInputKapten); err != nil {
		log.Printf("[AdminDeleteRitase] pre-check input_kapten gagal id=%d: %v", idRitase, err)
	} else if nInputKapten > 0 {
		return response.Error(c, http.StatusBadRequest,
			fmt.Sprintf("Ritase tidak bisa dihapus karena sudah ada %d input kapten yang terhubung. Data kapten dipertahankan agar tidak hilang. Batalkan penautan / hubungi kapten terlebih dahulu.", nInputKapten))
	}

	var nKonfirmasi int
	if err := h.DB.QueryRow(ctx, "SELECT COUNT(*) FROM konfirmasi_penjemputan WHERE id_ritase = $1", idRitase).Scan(&nKonfirmasi); err != nil {
		log.Printf("[AdminDeleteRitase] pre-check konfirmasi_penjemputan gagal id=%d: %v", idRitase, err)
	} else if nKonfirmasi > 0 {
		return response.Error(c, http.StatusBadRequest,
			fmt.Sprintf("Ritase tidak bisa dihapus karena sudah ada %d konfirmasi penjemputan yang terhubung. Batalkan penautan terlebih dahulu.", nKonfirmasi))
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		log.Printf("[AdminDeleteRitase] begin tx gagal id=%d: %v", idRitase, err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menghapus ritase. Coba lagi.")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, "UPDATE armada_tracking SET id_ritase = NULL WHERE id_ritase = $1", idRitase); err != nil {
		log.Printf("[AdminDeleteRitase] unlink armada_tracking gagal id=%d: %v", idRitase, err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menghapus ritase. Coba lagi.")
	}
	_, _ = tx.Exec(ctx, "DELETE FROM ritase_event WHERE id_ritase = $1", idRitase)
	_, _ = tx.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase = $1", idRitase)
	if _, err = tx.Exec(ctx, "DELETE FROM ritase WHERE id_ritase = $1", idRitase); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			log.Printf("[AdminDeleteRitase] FK block id=%d: %v", idRitase, err)
			return response.Error(c, http.StatusBadRequest,
				"Ritase tidak bisa dihapus karena masih ada data kapten / penjemputan yang terhubung. Data dipertahankan agar tidak hilang. Batalkan penautan terlebih dahulu.")
		}
		log.Printf("[AdminDeleteRitase] delete ritase gagal id=%d: %v", idRitase, err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menghapus ritase. Coba lagi.")
	}
	if err = tx.Commit(ctx); err != nil {
		log.Printf("[AdminDeleteRitase] commit gagal id=%d: %v", idRitase, err)
		return response.Error(c, http.StatusInternalServerError, "Gagal menghapus ritase. Coba lagi.")
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
	JamMulai    string      `json:"jam_mulai"`
	JamSelesai  string      `json:"jam_selesai"`
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

	// ── Cek status ritase ──
	var currentStatus string
	err = h.DB.QueryRow(ctx, "SELECT status FROM ritase WHERE id_ritase = $1", idRitase).Scan(&currentStatus)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Ritase tidak ditemukan")
	}

	// Cek apakah user adalah whisnu (hak eksklusif khusus akun whisnu)
	userID, _ := c.Get(middleware.CtxUserID).(int64)
	var updateUsername string
	_ = h.DB.QueryRow(ctx, "SELECT username FROM users WHERE id_user = $1", userID).Scan(&updateUsername)
	isWhisnuUpdate := strings.ToLower(updateUsername) == "whisnu"

	if !isWhisnuUpdate {
		if currentStatus == "selesai" {
			return response.Error(c, http.StatusBadRequest, "Ritase sudah selesai dan tidak bisa diubah")
		}
		if currentStatus == "berjalan" {
			// Saat berjalan: hanya boleh update stops, driver/kendaraan/ritase_ke/status tidak boleh diubah
			if req.IDDriver != 0 || req.IDKendaraan != 0 || req.RitaseKe != 0 || req.Status != "" || req.JenisRitase != "" {
				return response.Error(c, http.StatusBadRequest, "Ritase sedang berjalan. Hanya stops yang bisa diubah.")
			}
		}
	}

	// Jika status diubah kembali ke 'direncanakan', bersihkan riwayat event agar fresh
	if req.Status == "direncanakan" && currentStatus != "direncanakan" {
		_, _ = h.DB.Exec(ctx, "DELETE FROM ritase_event WHERE id_ritase = $1", idRitase)
		_, _ = h.DB.Exec(ctx, "UPDATE armada_tracking SET id_ritase = NULL WHERE id_ritase = $1", idRitase)
	}

	// ── Validasi status driver harus aktif (jika driver diganti) ──
	if req.IDDriver != 0 {
		var driverStatus string
		err := h.DB.QueryRow(ctx, "SELECT COALESCE(status_driver, 'on_duty') FROM driver WHERE id_driver = $1", req.IDDriver).Scan(&driverStatus)
		if err != nil {
			return response.Error(c, http.StatusBadRequest, "Driver tidak ditemukan")
		}
		if driverStatus != "on_duty" && driverStatus != "aktif" && driverStatus != "bertugas" {
			return response.Error(c, http.StatusBadRequest, fmt.Sprintf("Driver sedang tidak aktif (status: %s). Hanya driver aktif yang bisa ditugaskan.", driverStatus))
		}
	}

	// ── Validasi status kendaraan harus tersedia (jika kendaraan diganti) ──
	if req.IDKendaraan != 0 {
		var vehicleStatus string
		err := h.DB.QueryRow(ctx, "SELECT COALESCE(status_kendaraan, 'available') FROM kendaraan WHERE id_kendaraan = $1", req.IDKendaraan).Scan(&vehicleStatus)
		if err != nil {
			return response.Error(c, http.StatusBadRequest, "Kendaraan tidak ditemukan")
		}
		if vehicleStatus != "available" && vehicleStatus != "in_transit" && vehicleStatus != "tersedia" && vehicleStatus != "berjalan" && vehicleStatus != "aktif" {
			return response.Error(c, http.StatusBadRequest, fmt.Sprintf("Kendaraan sedang tidak tersedia (status: %s). Hanya kendaraan available yang bisa ditugaskan.", vehicleStatus))
		}
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	// Prioritas: jam_mulai/jam_selesai explicit > auto dari ambilJadwal > keep existing
	var jamMulai, jamSelesai interface{}
	if req.JamMulai != "" && req.JamSelesai != "" {
		// Frontend kirim jam manual → pakai langsung
		jamMulai = req.JamMulai
		jamSelesai = req.JamSelesai
	} else if req.JenisRitase != "" && req.RitaseKe != 0 {
		// Frontend gak kirim jam tapi kirim jenis_ritase+ritase_ke → hitung otomatis
		jamMulai, jamSelesai = h.ambilJadwalDB(ctx, req.JenisRitase, req.RitaseKe)
	}
	// else: both nil → COALESCE di SQL tetap pertahankan nilai lama

	// Extract user ID for audit
	userID, _ = c.Get(middleware.CtxUserID).(int64)

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
		    jam_selesai = COALESCE($8, jam_selesai),
		    updated_at = NOW(),
		    updated_by = $10
		WHERE id_ritase = $9
	`, req.IDDriver, req.IDKendaraan, req.IDDropPoint, req.RitaseKe, req.Status, req.JenisRitase, jamMulai, jamSelesai, idRitase, userID)
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
			// Validasi FK: pastikan ID lokasi benar-benar ada di tabel terkait
			if err := validasiStopID(ctx, tx, stop.Jenis, stop.IDLokasi); err != nil {
				return response.Error(c, http.StatusBadRequest, err.Error())
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
	JamMulai    string      `json:"jam_mulai"`
	JamSelesai  string      `json:"jam_selesai"`
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

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	// ── Validasi status driver harus aktif ──
	var driverStatus string
	err := h.DB.QueryRow(ctx, "SELECT COALESCE(status_driver, 'on_duty') FROM driver WHERE id_driver = $1", req.IDDriver).Scan(&driverStatus)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "Driver tidak ditemukan")
	}
	if driverStatus != "on_duty" && driverStatus != "aktif" && driverStatus != "bertugas" {
		return response.Error(c, http.StatusBadRequest, fmt.Sprintf("Driver sedang tidak aktif (status: %s). Hanya driver aktif yang bisa ditugaskan.", driverStatus))
	}

	// ── Validasi status kendaraan harus tersedia ──
	var vehicleStatus string
	err = h.DB.QueryRow(ctx, "SELECT COALESCE(status_kendaraan, 'available') FROM kendaraan WHERE id_kendaraan = $1", req.IDKendaraan).Scan(&vehicleStatus)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "Kendaraan tidak ditemukan")
	}
	if vehicleStatus != "available" && vehicleStatus != "in_transit" && vehicleStatus != "tersedia" && vehicleStatus != "berjalan" && vehicleStatus != "aktif" {
		return response.Error(c, http.StatusBadRequest, fmt.Sprintf("Kendaraan sedang tidak tersedia (status: %s). Hanya kendaraan available yang bisa ditugaskan.", vehicleStatus))
	}

	// ── Validasi duplikat: driver tidak boleh punya ritase ke-X di tanggal yang sama ──
	var existingCount int
	_ = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM ritase
		WHERE id_driver = $1 AND tanggal = $2::date AND ritase_ke = $3
	`, req.IDDriver, req.Tanggal, req.RitaseKe).Scan(&existingCount)
	if existingCount > 0 {
		return response.Error(c, http.StatusConflict,
			fmt.Sprintf("Driver sudah memiliki ritase ke-%d pada tanggal %s. Pilih driver lain atau ritase ke berbeda.", req.RitaseKe, req.Tanggal))
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

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	// Extract user ID for audit
	userID, _ := c.Get(middleware.CtxUserID).(int64)

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

	// Auto-tentukan jenis_ritase kalau frontend gak kirim
	jenisRitase := req.JenisRitase
	if jenisRitase == "" {
		jenisRitase = h.tentukanJenisRitaseDB(ctx, req.IDDriver, req.RitaseKe)
	}

	// Prioritas: jam_mulai/jam_selesai explicit > auto dari ambilJadwal
	var jamMulai, jamSelesai interface{}
	if req.JamMulai != "" && req.JamSelesai != "" {
		jamMulai = req.JamMulai
		jamSelesai = req.JamSelesai
	} else {
		jamMulai, jamSelesai = h.ambilJadwalDB(ctx, jenisRitase, req.RitaseKe)
	}
	var jenisVal interface{}
	if jenisRitase != "" {
		jenisVal = jenisRitase
	}

	var idRitase int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase (
			kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status,
			jenis_ritase, jam_mulai, jam_selesai, created_by
		) VALUES (
			$1, $2::date, $3, $4, $5, $6, 'direncanakan', $7, $8, $9, $10
		) RETURNING id_ritase
	`, kodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, finalDropPointID, req.RitaseKe, jenisVal, jamMulai, jamSelesai, userID).Scan(&idRitase)

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
		// Validasi FK: pastikan ID lokasi benar-benar ada di tabel terkait
		if err := validasiStopID(ctx, tx, stop.Jenis, stop.IDLokasi); err != nil {
			return response.Error(c, http.StatusBadRequest, err.Error())
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
	case "id_seller", "id_implant", "id_drop_point", "id_gudang":
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
	case "implant":
		return "id_implant", nil
	}
	return "", fmt.Errorf("jenis_stop tidak dikenal: %q", jenis)
}

// validasiStopID memastikan ID lokasi pada stop benar-benar ada di tabel terkait sebelum INSERT.
func validasiStopID(ctx context.Context, tx pgx.Tx, jenis string, idLokasi int64) error {
	var exists bool
	var table, column string
	switch jenis {
	case "gudang":
		table, column = "gudang", "id_gudang"
	case "seller":
		table, column = "seller", "id_seller"
	case "implant":
		// Rute hanya boleh berhenti di titik penjemputan (host/self), bukan
		// implant sumber yang sudah digabung ke titik lain.
		query := `
			SELECT EXISTS(
				SELECT 1 FROM implant i
				WHERE i.id_implant = $1 AND i.status = 'aktif'
				  AND i.latitude IS NOT NULL AND i.longitude IS NOT NULL
				  AND (
					EXISTS (SELECT 1 FROM kapten_implant_map kim
					        WHERE COALESCE(kim.id_titik_implant, kim.id_implant) = i.id_implant)
					OR NOT EXISTS (SELECT 1 FROM kapten_implant_map kim
					               WHERE kim.id_implant = i.id_implant)
				  )
			)`
		if err := tx.QueryRow(ctx, query, idLokasi).Scan(&exists); err != nil {
			return fmt.Errorf("gagal validasi implant ID %d: %v", idLokasi, err)
		}
		if !exists {
			return fmt.Errorf("implant ID %d bukan titik penjemputan aktif / belum punya koordinat", idLokasi)
		}
		return nil
	case "gateway", "drop_point":
		table, column = "drop_point", "id_drop_point"
	default:
		return fmt.Errorf("jenis_stop tidak dikenal: %q", jenis)
	}
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = $1)", table, column)
	if err := tx.QueryRow(ctx, query, idLokasi).Scan(&exists); err != nil {
		return fmt.Errorf("gagal validasi %s ID %d: %v", jenis, idLokasi, err)
	}
	if !exists {
		return fmt.Errorf("%s ID %d tidak ditemukan di database", jenis, idLokasi)
	}
	return nil
}

// AdminGetMasterOptions Ambil opsi master data (drivers, kendaraan, drop_points, sellers, gudangs)
func (h *APIHandler) AdminGetMasterOptions(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	// 1. Drivers (hanya aktif — filter nonaktif di backend)
	drivers := make([]map[string]interface{}, 0)
	dRows, _ := h.DB.Query(ctx, "SELECT id_driver, nama_driver, COALESCE(jabatan, 'TRANSPORTER'), COALESCE(status_driver, 'on_duty') FROM driver WHERE LOWER(COALESCE(status_driver, 'on_duty')) != 'nonaktif' ORDER BY id_driver ASC")
	if dRows != nil {
		for dRows.Next() {
			var id int64
			var nama, jabatan, status string
			if err := dRows.Scan(&id, &nama, &jabatan, &status); err == nil {
				drivers = append(drivers, map[string]interface{}{"id_driver": id, "nama_driver": nama, "jabatan": jabatan, "status_driver": status})
			}
		}
		dRows.Close()
	}

	// 2. Kendaraan (termasuk status untuk filter frontend)
	kendaraans := make([]map[string]interface{}, 0)
	kRows, _ := h.DB.Query(ctx, "SELECT id_kendaraan, plat_nomor, jenis_kendaraan, COALESCE(status_kendaraan, 'available') FROM kendaraan ORDER BY id_kendaraan ASC")
	if kRows != nil {
		for kRows.Next() {
			var id int64
			var plat, jenis, status string
			if err := kRows.Scan(&id, &plat, &jenis, &status); err == nil {
				kendaraans = append(kendaraans, map[string]interface{}{"id_kendaraan": id, "plat_nomor": plat, "jenis_kendaraan": jenis, "status_kendaraan": status})
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

	// 4. Sellers (hanya aktif — seller pindahan implant sudah nonaktif/dihapus)
	sellers := make([]map[string]interface{}, 0)
	sRows, _ := h.DB.Query(ctx, "SELECT id_seller, nama_seller, kode_seller FROM seller WHERE status = 'aktif' ORDER BY id_seller ASC")
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

	// 4b. Implants (hanya aktif)
	implants := make([]map[string]interface{}, 0)
	iRows, _ := h.DB.Query(ctx, "SELECT id_implant, nama_implant, kode_implant FROM implant WHERE status = 'aktif' ORDER BY id_implant ASC")
	if iRows != nil {
		for iRows.Next() {
			var id int64
			var nama, kode string
			if err := iRows.Scan(&id, &nama, &kode); err == nil {
				implants = append(implants, map[string]interface{}{"id_implant": id, "nama_implant": nama, "kode_implant": kode})
			}
		}
		iRows.Close()
	}

	// 4c. Titik penjemputan implant (host/self saja)
	implantPoints := make([]map[string]interface{}, 0)
	ipRows, _ := h.DB.Query(ctx, `
		SELECT i.id_implant, i.nama_implant, COALESCE(i.kode_implant, ''),
		       (i.latitude IS NOT NULL AND i.longitude IS NOT NULL) AS has_coordinate
		FROM implant i
		WHERE i.status = 'aktif'
		  AND (
			EXISTS (SELECT 1 FROM kapten_implant_map kim
			        WHERE COALESCE(kim.id_titik_implant, kim.id_implant) = i.id_implant)
			OR NOT EXISTS (SELECT 1 FROM kapten_implant_map kim
			               WHERE kim.id_implant = i.id_implant)
		  )
		ORDER BY i.nama_implant ASC`)
	if ipRows != nil {
		for ipRows.Next() {
			var id int64
			var nama, kode string
			var hasCoordinate bool
			if err := ipRows.Scan(&id, &nama, &kode, &hasCoordinate); err == nil {
				implantPoints = append(implantPoints, map[string]interface{}{
					"id_implant": id, "nama_implant": nama, "kode_implant": kode,
					"has_coordinate": hasCoordinate,
				})
			}
		}
		ipRows.Close()
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

	// 6. Driver Jenis (mapping driver → jenis ritase per ritase_ke)
	driverJenis := make([]map[string]interface{}, 0)
	djRows, _ := h.DB.Query(ctx, "SELECT id_driver, ritase_ke, jenis FROM driver_ritase_jenis ORDER BY id_driver, ritase_ke")
	if djRows != nil {
		for djRows.Next() {
			var idDriver, ritaseKe int64
			var jenis string
			if err := djRows.Scan(&idDriver, &ritaseKe, &jenis); err == nil {
				driverJenis = append(driverJenis, map[string]interface{}{"id_driver": idDriver, "ritase_ke": ritaseKe, "jenis": jenis})
			}
		}
		djRows.Close()
	}

	// 7. Jam Ritase Config (jam mulai/selesai per jenis + ritase_ke)
	jamRitase := make([]map[string]interface{}, 0)
	jrRows, _ := h.DB.Query(ctx, "SELECT jenis, ritase_ke, jam_mulai::text, jam_selesai::text FROM jadwal_ritase_config ORDER BY jenis, ritase_ke")
	if jrRows != nil {
		for jrRows.Next() {
			var jenis string
			var ritaseKe int64
			var jamMulai, jamSelesai string
			if err := jrRows.Scan(&jenis, &ritaseKe, &jamMulai, &jamSelesai); err == nil {
				jamRitase = append(jamRitase, map[string]interface{}{"jenis": jenis, "ritase_ke": ritaseKe, "jam_mulai": jamMulai, "jam_selesai": jamSelesai})
			}
		}
		jrRows.Close()
	}

	return response.OK(c, map[string]interface{}{
		"drivers":        drivers,
		"kendaraan":      kendaraans,
		"drop_points":    dropPoints,
		"sellers":        sellers,
		"implants":       implants,
		"implant_points": implantPoints,
		"gudangs":        gudangs,
		"driver_jenis":   driverJenis,
		"jam_ritase":     jamRitase,
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
