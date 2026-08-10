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
	Stops       []FixedStop `json:"stops"`
}

// Fixed Route Template Definitions for 1-Click Auto-Generate
var defaultFixedRoutes = []FixedRoute{
	{
		IDDriver: 3, IDKendaraan: 2, IDDropPoint: 2, RitaseKe: 1,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Mulai dari gudang origin"},
			{2, "seller", 3, "id_seller", "Ambil paket di Seller 3"},
			{3, "seller", 1, "id_seller", "Ambil paket di Seller 1"},
			{4, "drop_point", 2, "id_drop_point", "Tujuan akhir Drop Point 2"},
		},
	},
	{
		IDDriver: 3, IDKendaraan: 2, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 3, "id_seller", "Seller 3"},
			{3, "gudang", 1, "id_gudang", "Gudang 1"},
			{4, "drop_point", 2, "id_drop_point", "Drop Point 2"},
		},
	},
	{
		IDDriver: 2, IDKendaraan: 6, IDDropPoint: 2, RitaseKe: 1,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 2, "id_seller", "Seller 2"},
			{3, "gudang", 2, "id_gudang", "Gudang 2"},
			{4, "drop_point", 2, "id_drop_point", "Drop Point 2"},
		},
	},
	{
		IDDriver: 2, IDKendaraan: 6, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "drop_point", 2, "id_drop_point", "Drop Point 2"},
			{2, "seller", 2, "id_seller", "Seller 2"},
			{3, "gudang", 2, "id_gudang", "Gudang 2"},
			{4, "drop_point", 2, "id_drop_point", "Drop Point 2"},
		},
	},
	{
		IDDriver: 1, IDKendaraan: 11, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "drop_point", 2, "id_drop_point", "Drop Point 2"},
			{2, "seller", 4, "id_seller", "Seller 4"},
			{3, "seller", 1, "id_seller", "Seller 1"},
			{4, "gudang", 1, "id_gudang", "Gudang 1"},
			{5, "drop_point", 2, "id_drop_point", "Drop Point 2"},
		},
	},
	{
		IDDriver: 4, IDKendaraan: 15, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 7, "id_seller", "PGA2 Seller 7"},
			{3, "drop_point", 2, "id_drop_point", "Drop Point 2"},
		},
	},
	{
		IDDriver: 15, IDKendaraan: 3, IDDropPoint: 2, RitaseKe: 3,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "drop_point", 2, "id_drop_point", "Drop Point 2"},
			{3, "drop_point", 2, "id_drop_point", "Drop Point 2"},
		},
	},
	{
		IDDriver: 11, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "drop_point", 3, "id_drop_point", "Drop Point 3"},
		},
	},
	{
		IDDriver: 11, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 3,
		Stops: []FixedStop{
			{1, "gudang", 3, "id_gudang", "Gudang 3"},
			{2, "gudang", 2, "id_gudang", "Gudang 2"},
			{3, "drop_point", 3, "id_drop_point", "Drop Point 3"},
		},
	},
	{
		IDDriver: 10, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 1,
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "drop_point", 3, "id_drop_point", "Drop Point 3"},
		},
	},
	{
		IDDriver: 10, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 4,
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "drop_point", 3, "id_drop_point", "Drop Point 3"},
		},
	},
}

// AdminGenerateDailyRitase Handler 1-Klik Generate Rute Harian
func (h *APIHandler) AdminGenerateDailyRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	// 1. Clear current date's uncompleted ritase and stops to cleanly overwrite/replace
	_, _ = h.DB.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase IN (SELECT id_ritase FROM ritase WHERE tanggal = CURRENT_DATE AND status != 'selesai')")
	_, _ = h.DB.Exec(ctx, "DELETE FROM ritase WHERE tanggal = CURRENT_DATE AND status != 'selesai'")

	countGenerated := 0
	todayStr := time.Now().Format("20060102")

	for _, route := range defaultFixedRoutes {
		kodeRitase := fmt.Sprintf("TR-%s-D%d-R%d", todayStr, route.IDDriver, route.RitaseKe)

		var idRitase int64
		err := h.DB.QueryRow(ctx, `
			INSERT INTO ritase (
				kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status
			) VALUES (
				$1, CURRENT_DATE, $2, $3, $4, $5, 'direncanakan'
			) RETURNING id_ritase
		`, kodeRitase, route.IDDriver, route.IDKendaraan, route.IDDropPoint, route.RitaseKe).Scan(&idRitase)

		if err != nil {
			log.Printf("Err generate ritase D%d: %v", route.IDDriver, err)
			continue
		}

		for _, stop := range route.Stops {
			query := fmt.Sprintf(`
				INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
				VALUES ($1, $2, $3, $4, $5)
			`, stop.KolomLokasi)

			_, _ = h.DB.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan)
		}
		countGenerated++
	}

	return response.OK(c, map[string]interface{}{
		"total_generated": countGenerated,
		"message":         fmt.Sprintf("Berhasil menimpa & meng-generate %d ritase harian!", countGenerated),
	})
}

// AdminGetRitases Ambil daftar ritase untuk tanggal tertentu (default hari ini)
func (h *APIHandler) AdminGetRitases(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tanggalParam := c.QueryParam("tanggal")
	if tanggalParam == "" {
		tanggalParam = time.Now().Format("2006-01-02")
	}

	rows, err := h.DB.Query(ctx, `
		SELECT 
			r.id_ritase, r.kode_ritase, TO_CHAR(r.tanggal, 'YYYY-MM-DD') AS tanggal,
			r.id_driver, COALESCE(d.nama_driver, 'Driver #' || r.id_driver) AS nama_driver,
			COALESCE(d.jabatan, 'TRANSPORTER') AS jabatan_driver,
			r.id_kendaraan, COALESCE(k.plat_nomor, 'KD-' || r.id_kendaraan) AS nopol,
			r.id_drop_point, COALESCE(dp.nama_drop_point, 'Drop Point #' || r.id_drop_point) AS nama_drop_point,
			r.ritase_ke, r.status
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
		var kodeRitase, tanggal, namaDriver, jabatanDriver, nopol, namaDropPoint, status string
		var ritaseKe int

		if err := rows.Scan(&idRitase, &kodeRitase, &tanggal, &idDriver, &namaDriver, &jabatanDriver, &idKendaraan, &nopol, &idDropPoint, &namaDropPoint, &ritaseKe, &status); err != nil {
			continue
		}

		// Ambil stops untuk ritase ini (pastikan selalu slice kosong, bukan nil)
		stops := make([]map[string]interface{}, 0)
		stopRows, _ := h.DB.Query(ctx, `
			SELECT 
				rs.id_stop, rs.urutan, rs.jenis_stop,
				rs.id_seller, rs.id_drop_point, rs.id_gudang, rs.keterangan,
				COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Lokasi') AS nama_lokasi
			FROM ritase_stop rs
			LEFT JOIN seller s ON s.id_seller = rs.id_seller
			LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
			LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
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

				if err := stopRows.Scan(&idStop, &urutan, &jenisStop, &idSeller, &idDP, &idGudang, &ket, &namaLokasi); err == nil {
					stops = append(stops, map[string]interface{}{
						"id_stop":       idStop,
						"urutan":        urutan,
						"jenis_stop":    jenisStop,
						"id_seller":     idSeller,
						"id_drop_point": idDP,
						"id_gudang":     idGudang,
						"keterangan":    ket,
						"nama_lokasi":   namaLokasi,
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

	_, _ = h.DB.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase = $1", idRitase)
	_, err = h.DB.Exec(ctx, "DELETE FROM ritase WHERE id_ritase = $1", idRitase)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal menghapus ritase: "+err.Error())
	}

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

	// Update Header Ritase
	_, err = h.DB.Exec(ctx, `
		UPDATE ritase
		SET id_driver = COALESCE(NULLIF($1, 0), id_driver),
		    id_kendaraan = COALESCE(NULLIF($2, 0), id_kendaraan),
		    id_drop_point = COALESCE(NULLIF($3, 0), id_drop_point),
		    ritase_ke = COALESCE(NULLIF($4, 0), ritase_ke),
		    status = COALESCE(NULLIF($5, ''), status)
		WHERE id_ritase = $6
	`, req.IDDriver, req.IDKendaraan, req.IDDropPoint, req.RitaseKe, req.Status, idRitase)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengupdate ritase: "+err.Error())
	}

	// Jika ada stops baru, perbarui ritase_stop
	if len(req.Stops) > 0 {
		_, _ = h.DB.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase = $1", idRitase)

		for _, stop := range req.Stops {
			kolom := "id_seller"
			if stop.Jenis == "gudang" {
				kolom = "id_gudang"
			} else if stop.Jenis == "drop_point" {
				kolom = "id_drop_point"
			}

			query := fmt.Sprintf(`
				INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
				VALUES ($1, $2, $3, $4, $5)
			`, kolom)

			_, _ = h.DB.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan)
		}
	}

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
	Stops       []FixedStop `json:"stops"`
}

// AdminCreateRitase Handler untuk membuat ritase manual baru
func (h *APIHandler) AdminCreateRitase(c echo.Context) error {
	var req CreateRitaseRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload tidak valid")
	}

	if req.IDDriver == 0 || req.IDKendaraan == 0 || req.IDDropPoint == 0 {
		return response.Error(c, http.StatusBadRequest, "Driver, Kendaraan, dan Drop Point wajib dipilih")
	}

	if req.Tanggal == "" {
		req.Tanggal = time.Now().Format("2006-01-02")
	}
	if req.RitaseKe == 0 {
		req.RitaseKe = 1
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	todayClean := strings.ReplaceAll(req.Tanggal, "-", "")
	kodeRitase := fmt.Sprintf("TR-%s-D%d-R%d", todayClean, req.IDDriver, req.RitaseKe)

	var idRitase int64
	err := h.DB.QueryRow(ctx, `
		INSERT INTO ritase (
			kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status
		) VALUES (
			$1, $2::date, $3, $4, $5, $6, 'direncanakan'
		) RETURNING id_ritase
	`, kodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, req.IDDropPoint, req.RitaseKe).Scan(&idRitase)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal membuat ritase: "+err.Error())
	}

	for _, stop := range req.Stops {
		kolom := "id_seller"
		if stop.Jenis == "gudang" {
			kolom = "id_gudang"
		} else if stop.Jenis == "drop_point" {
			kolom = "id_drop_point"
		}

		query := fmt.Sprintf(`
			INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, %s, keterangan)
			VALUES ($1, $2, $3, $4, $5)
		`, kolom)

		_, _ = h.DB.Exec(ctx, query, idRitase, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.Keterangan)
	}

	return response.Created(c, map[string]interface{}{
		"id_ritase": idRitase,
		"message":   "Jadwal ritase baru berhasil dibuat!",
	})
}

