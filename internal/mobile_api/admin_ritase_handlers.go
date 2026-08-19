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
			{4, "gateway", 2, "id_drop_point", "Tujuan akhir Gateway 2"},
		},
	},
	{
		IDDriver: 3, IDKendaraan: 2, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 3, "id_seller", "Seller 3"},
			{3, "gudang", 1, "id_gudang", "Gudang 1"},
			{4, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 2, IDKendaraan: 6, IDDropPoint: 2, RitaseKe: 1,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 2, "id_seller", "Seller 2"},
			{3, "gudang", 2, "id_gudang", "Gudang 2"},
			{4, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 2, IDKendaraan: 6, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gateway", 2, "id_drop_point", "Gateway 2"},
			{2, "seller", 2, "id_seller", "Seller 2"},
			{3, "gudang", 2, "id_gudang", "Gudang 2"},
			{4, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 1, IDKendaraan: 11, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gateway", 2, "id_drop_point", "Gateway 2"},
			{2, "seller", 4, "id_seller", "Seller 4"},
			{3, "seller", 1, "id_seller", "Seller 1"},
			{4, "gudang", 1, "id_gudang", "Gudang 1"},
			{5, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 4, IDKendaraan: 15, IDDropPoint: 2, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "seller", 7, "id_seller", "PGA2 Seller 7"},
			{3, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 15, IDKendaraan: 3, IDDropPoint: 2, RitaseKe: 3,
		Stops: []FixedStop{
			{1, "gudang", 1, "id_gudang", "Gudang 1"},
			{2, "gateway", 2, "id_drop_point", "Gateway 2"},
			{3, "gateway", 2, "id_drop_point", "Gateway 2"},
		},
	},
	{
		IDDriver: 11, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 2,
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
	{
		IDDriver: 11, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 3,
		Stops: []FixedStop{
			{1, "gudang", 3, "id_gudang", "Gudang 3"},
			{2, "gudang", 2, "id_gudang", "Gudang 2"},
			{3, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
	{
		IDDriver: 10, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 1,
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
	{
		IDDriver: 10, IDKendaraan: 9, IDDropPoint: 3, RitaseKe: 4,
		Stops: []FixedStop{
			{1, "gudang", 2, "id_gudang", "Gudang 2"},
			{2, "gateway", 3, "id_drop_point", "Gateway 3"},
		},
	},
}

type PreviewRoute struct {
	IDDriver    int64         `json:"id_driver"`
	NamaDriver  string        `json:"nama_driver"`
	IDKendaraan int64         `json:"id_kendaraan"`
	PlatNomor   string        `json:"plat_nomor"`
	RitaseKe    int           `json:"ritase_ke"`
	Stops       []PreviewStop `json:"stops"`
}

type PreviewStop struct {
	Urutan     int    `json:"urutan"`
	JenisStop  string `json:"jenis_stop"`
	IDLokasi   int64  `json:"id_lokasi"`
	NamaLokasi string `json:"nama_lokasi"`
	Keterangan string `json:"keterangan"`
}

// AdminPreviewGenerateDailyRitase mengembalikan data rute yang akan digenerate beserta nama lokasinya
func (h *APIHandler) AdminPreviewGenerateDailyRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

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

	// 2. Determine Routes to Use (Latest generated or fallback to default)
	var routesToUse []FixedRoute
	var latestDate *time.Time
	_ = h.DB.QueryRow(ctx, "SELECT MAX(tanggal) FROM ritase").Scan(&latestDate)

	if latestDate != nil {
		rowsR, errR := h.DB.Query(ctx, "SELECT id_ritase, id_driver, id_kendaraan, COALESCE(id_drop_point, 0), ritase_ke FROM ritase WHERE tanggal = $1 ORDER BY id_driver, ritase_ke", *latestDate)
		if errR == nil {
			var ritaseMap = make(map[int64]*FixedRoute)
			var orderedRitaseIDs []int64

			for rowsR.Next() {
				var idRitase, idDriver, idKendaraan int64
				var idDropPoint int64
				var ritaseKe int
				if err := rowsR.Scan(&idRitase, &idDriver, &idKendaraan, &idDropPoint, &ritaseKe); err == nil {
					ritaseMap[idRitase] = &FixedRoute{
						IDDriver:    idDriver,
						IDKendaraan: idKendaraan,
						IDDropPoint: idDropPoint,
						RitaseKe:    ritaseKe,
						Stops:       []FixedStop{},
					}
					orderedRitaseIDs = append(orderedRitaseIDs, idRitase)
				}
			}
			rowsR.Close()

			if len(orderedRitaseIDs) > 0 {
				rowsS, errS := h.DB.Query(ctx, `
					SELECT rs.id_ritase, rs.urutan, rs.jenis_stop,
						COALESCE(rs.id_gudang, rs.id_seller, rs.id_drop_point) as id_lokasi,
						rs.keterangan
					FROM ritase_stop rs
					JOIN ritase r ON r.id_ritase = rs.id_ritase
					WHERE r.tanggal = $1
					ORDER BY rs.id_ritase, rs.urutan
				`, *latestDate)

				if errS == nil {
					for rowsS.Next() {
						var idRitase int64
						var stop FixedStop
						var idLokasi *int64
						var ket *string

						if err := rowsS.Scan(&idRitase, &stop.Urutan, &stop.Jenis, &idLokasi, &ket); err == nil {
							if idLokasi != nil {
								stop.IDLokasi = *idLokasi
							}
							if ket != nil {
								stop.Keterangan = *ket
							}
							if r, ok := ritaseMap[idRitase]; ok {
								r.Stops = append(r.Stops, stop)
							}
						}
					}
					rowsS.Close()
				}

				for _, idRitase := range orderedRitaseIDs {
					routesToUse = append(routesToUse, *ritaseMap[idRitase])
				}
			}
		}
	}

	if len(routesToUse) == 0 {
		routesToUse = defaultFixedRoutes
	}

	// 3. Map Routes to Preview Format
	var previewRoutes []PreviewRoute
	for _, fr := range routesToUse {
		driverName := fmt.Sprintf("Driver #%d", fr.IDDriver)
		if name, ok := drivers[fr.IDDriver]; ok {
			driverName = name
		}

		plat := fmt.Sprintf("Kendaraan #%d", fr.IDKendaraan)
		if p, ok := vehicles[fr.IDKendaraan]; ok {
			plat = p
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
				Keterangan: fs.Keterangan,
			})
		}

		previewRoutes = append(previewRoutes, PreviewRoute{
			IDDriver:    fr.IDDriver,
			NamaDriver:  driverName,
			IDKendaraan: fr.IDKendaraan,
			PlatNomor:   plat,
			RitaseKe:    fr.RitaseKe,
			Stops:       previewStops,
		})
	}

	return response.OK(c, map[string]interface{}{
		"total_preview": len(previewRoutes),
		"routes":        previewRoutes,
	})
}

// AdminGenerateDailyRitase Handler 1-Klik Generate Rute Harian
func (h *APIHandler) AdminGenerateDailyRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	var req struct {
		Routes []FixedRoute `json:"routes"`
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

	// 1. Clear current date's uncompleted ritase_event, ritase_stop, armada_tracking, and ritase to cleanly overwrite/replace
	if _, err := tx.Exec(ctx, "DELETE FROM ritase_event WHERE id_ritase IN (SELECT id_ritase FROM ritase WHERE tanggal = CURRENT_DATE AND status != 'selesai')"); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal membersihkan event lama: "+err.Error())
	}
	if _, err := tx.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase IN (SELECT id_ritase FROM ritase WHERE tanggal = CURRENT_DATE AND status != 'selesai')"); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal membersihkan ritase_stop lama: "+err.Error())
	}
	if _, err := tx.Exec(ctx, "DELETE FROM armada_tracking WHERE id_ritase IN (SELECT id_ritase FROM ritase WHERE tanggal = CURRENT_DATE AND status != 'selesai')"); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal membersihkan tracking lama: "+err.Error())
	}
	if _, err := tx.Exec(ctx, "DELETE FROM ritase WHERE tanggal = CURRENT_DATE AND status != 'selesai'"); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal membersihkan ritase lama: "+err.Error())
	}

	countGenerated := 0
	todayStr := time.Now().Format("20060102")

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

		var idRitase int64
		err := tx.QueryRow(ctx, `
			INSERT INTO ritase (
				kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status
			) VALUES (
				$1, CURRENT_DATE, $2, $3, $4, $5, 'direncanakan'
			) RETURNING id_ritase
		`, kodeRitase, route.IDDriver, route.IDKendaraan, finalDropPointID, route.RitaseKe).Scan(&idRitase)

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
			r.id_drop_point, COALESCE(dp.nama_drop_point, 'Gateway #' || r.id_drop_point) AS nama_drop_point,
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

	// Update Header Ritase
	tag, err := tx.Exec(ctx, `
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
		req.Tanggal = time.Now().Format("2006-01-02")
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

	var idRitase int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase (
			kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, status
		) VALUES (
			$1, $2::date, $3, $4, $5, $6, 'direncanakan'
		) RETURNING id_ritase
	`, kodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, finalDropPointID, req.RitaseKe).Scan(&idRitase)

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

