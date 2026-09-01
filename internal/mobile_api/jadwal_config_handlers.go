package mobile_api

import (
	"context"
	"net/http"
	"time"

	"backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

// ══════════════════════════════════════════════════════════════
// JADWAL CONFIG CRUD — Konfigurasi jadwal ritase dinamis
// ══════════════════════════════════════════════════════════════

// ── Types ──

type JadwalConfigResponse struct {
	JamRitase      []JamRitaseConfig      `json:"jam_ritase"`
	DriverJenis    []DriverJenisConfig     `json:"driver_jenis"`
	RouteTemplates []RouteTemplateConfig   `json:"route_templates"`
}

type JamRitaseConfig struct {
	ID          int    `json:"id"`
	Jenis       string `json:"jenis"`
	RitaseKe    int    `json:"ritase_ke"`
	JamMulai    string `json:"jam_mulai"`
	JamSelesai  string `json:"jam_selesai"`
}

type DriverJenisConfig struct {
	ID        int    `json:"id"`
	IDDriver  int64  `json:"id_driver"`
	NamaDriver string `json:"nama_driver"`
	RitaseKe  int    `json:"ritase_ke"`
	Jenis     string `json:"jenis"`
}

type RouteTemplateConfig struct {
	ID            int               `json:"id"`
	IDDriver      int64             `json:"id_driver"`
	NamaDriver    string            `json:"nama_driver"`
	IDKendaraan   int64             `json:"id_kendaraan"`
	PlatNomor     string            `json:"plat_nomor"`
	IDDropPoint   int64             `json:"id_drop_point"`
	NamaDropPoint string            `json:"nama_drop_point"`
	RitaseKe      int               `json:"ritase_ke"`
	JenisRitase   string            `json:"jenis_ritase"`
	Aktif         bool              `json:"aktif"`
	Stops         []StopTemplate    `json:"stops"`
}

type StopTemplate struct {
	ID            int    `json:"id"`
	Urutan        int    `json:"urutan"`
	JenisStop     string `json:"jenis_stop"`
	IDLokasi      int64  `json:"id_lokasi"`
	KolomLokasi   string `json:"kolom_lokasi"`
	Keterangan    string `json:"keterangan"`
	NamaLokasi    string `json:"nama_lokasi"`
}

// ── GET /jadwal-config — Ambil semua config ──

func (h *APIHandler) GetJadwalConfig(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	// 1. Jam Ritase
	jamRows, err := h.DB.Query(ctx, "SELECT id, jenis, ritase_ke, jam_mulai::text, jam_selesai::text FROM jadwal_ritase_config ORDER BY jenis, ritase_ke")
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal ambil jam ritase: "+err.Error())
	}
	defer jamRows.Close()

	var jamList []JamRitaseConfig
	for jamRows.Next() {
		var j JamRitaseConfig
		if err := jamRows.Scan(&j.ID, &j.Jenis, &j.RitaseKe, &j.JamMulai, &j.JamSelesai); err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal scan jam ritase: "+err.Error())
		}
		jamList = append(jamList, j)
	}

	// 2. Driver → Jenis
	djRows, err := h.DB.Query(ctx, `
		SELECT drj.id, drj.id_driver, COALESCE(d.nama_driver, 'Driver #' || drj.id_driver), drj.ritase_ke, drj.jenis
		FROM driver_ritase_jenis drj
		LEFT JOIN driver d ON d.id_driver = drj.id_driver
		ORDER BY d.nama_driver, drj.ritase_ke
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal ambil driver jenis: "+err.Error())
	}
	defer djRows.Close()

	var djList []DriverJenisConfig
	for djRows.Next() {
		var dj DriverJenisConfig
		if err := djRows.Scan(&dj.ID, &dj.IDDriver, &dj.NamaDriver, &dj.RitaseKe, &dj.Jenis); err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal scan driver jenis: "+err.Error())
		}
		djList = append(djList, dj)
	}

	// 3. Route Templates
	rtRows, err := h.DB.Query(ctx, `
		SELECT rt.id, rt.id_driver, COALESCE(d.nama_driver, 'Driver #' || rt.id_driver),
		       rt.id_kendaraan, COALESCE(k.plat_nomor, 'Kend #' || rt.id_kendaraan),
		       rt.id_drop_point, COALESCE(dp.nama_drop_point, 'GW' || rt.id_drop_point),
		       rt.ritase_ke, COALESCE(rt.jenis_ritase, 'outgoing'), rt.aktif
		FROM ritase_route_template rt
		LEFT JOIN driver d ON d.id_driver = rt.id_driver
		LEFT JOIN kendaraan k ON k.id_kendaraan = rt.id_kendaraan
		LEFT JOIN drop_point dp ON dp.id_drop_point = rt.id_drop_point
		ORDER BY d.nama_driver, rt.ritase_ke
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal ambil route template: "+err.Error())
	}
	defer rtRows.Close()

	var rtList []RouteTemplateConfig
	for rtRows.Next() {
		var rt RouteTemplateConfig
		if err := rtRows.Scan(&rt.ID, &rt.IDDriver, &rt.NamaDriver, &rt.IDKendaraan, &rt.PlatNomor,
			&rt.IDDropPoint, &rt.NamaDropPoint, &rt.RitaseKe, &rt.JenisRitase, &rt.Aktif); err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal scan route template: "+err.Error())
		}

		// Ambil stops untuk route ini
		stopRows, err := h.DB.Query(ctx, `
			SELECT st.id, st.urutan, st.jenis_stop, st.id_lokasi, st.kolom_lokasi, COALESCE(st.keterangan, ''),
			       COALESCE(
			         CASE
			           WHEN st.kolom_lokasi = 'id_gudang' THEN (SELECT nama_gudang FROM gudang WHERE id_gudang = st.id_lokasi)
			           WHEN st.kolom_lokasi = 'id_seller' THEN (SELECT nama_seller FROM seller WHERE id_seller = st.id_lokasi)
			           WHEN st.kolom_lokasi = 'id_drop_point' THEN (SELECT nama_drop_point FROM drop_point WHERE id_drop_point = st.id_lokasi)
			           ELSE 'Lokasi #' || st.id_lokasi
			         END,
			         'Lokasi Tidak Diketahui'
			       )
			FROM ritase_stop_template st
			WHERE st.id_route_template = $1
			ORDER BY st.urutan
		`, rt.ID)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal ambil stops: "+err.Error())
		}

		for stopRows.Next() {
			var s StopTemplate
			if err := stopRows.Scan(&s.ID, &s.Urutan, &s.JenisStop, &s.IDLokasi, &s.KolomLokasi, &s.Keterangan, &s.NamaLokasi); err != nil {
				stopRows.Close()
				return response.Error(c, http.StatusInternalServerError, "Gagal scan stop: "+err.Error())
			}
			rt.Stops = append(rt.Stops, s)
		}
		stopRows.Close()

		rtList = append(rtList, rt)
	}

	return response.OK(c, JadwalConfigResponse{
		JamRitase:      jamList,
		DriverJenis:    djList,
		RouteTemplates: rtList,
	})
}

// ── POST /jadwal-config/jam — Tambah jam ritase ──

func (h *APIHandler) CreateJamRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	var req struct {
		Jenis      string `json:"jenis"`
		RitaseKe   int    `json:"ritase_ke"`
		JamMulai   string `json:"jam_mulai"`
		JamSelesai string `json:"jam_selesai"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	if req.Jenis == "" || req.RitaseKe <= 0 || req.JamMulai == "" || req.JamSelesai == "" {
		return response.Error(c, http.StatusBadRequest, "jenis, ritase_ke, jam_mulai, jam_selesai wajib diisi")
	}
	if req.Jenis != "outgoing" && req.Jenis != "incoming" {
		return response.Error(c, http.StatusBadRequest, "jenis harus 'outgoing' atau 'incoming'")
	}

	var id int
	err := h.DB.QueryRow(ctx, `
		INSERT INTO jadwal_ritase_config (jenis, ritase_ke, jam_mulai, jam_selesai)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, req.Jenis, req.RitaseKe, req.JamMulai, req.JamSelesai).Scan(&id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal tambah jam ritase: "+err.Error())
	}

	return response.OK(c, map[string]any{"id": id, "message": "Jam ritase berhasil ditambahkan"})
}

// ── PUT /jadwal-config/jam/:id — Update jam ritase ──

func (h *APIHandler) UpdateJamRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	id := c.Param("id")
	var req struct {
		JamMulai   string `json:"jam_mulai"`
		JamSelesai string `json:"jam_selesai"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	if req.JamMulai == "" || req.JamSelesai == "" {
		return response.Error(c, http.StatusBadRequest, "jam_mulai dan jam_selesai wajib diisi")
	}

	tag, err := h.DB.Exec(ctx, `
		UPDATE jadwal_ritase_config SET jam_mulai = $1, jam_selesai = $2 WHERE id = $3
	`, req.JamMulai, req.JamSelesai, id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal update jam ritase: "+err.Error())
	}
	if tag.RowsAffected() == 0 {
		return response.Error(c, http.StatusNotFound, "Jam ritase tidak ditemukan")
	}

	return response.OK(c, map[string]string{"message": "Jam ritase berhasil diupdate"})
}

// ── DELETE /jadwal-config/jam/:id — Hapus jam ritase ──

func (h *APIHandler) DeleteJamRitase(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	id := c.Param("id")
	tag, err := h.DB.Exec(ctx, "DELETE FROM jadwal_ritase_config WHERE id = $1", id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal hapus jam ritase: "+err.Error())
	}
	if tag.RowsAffected() == 0 {
		return response.Error(c, http.StatusNotFound, "Jam ritase tidak ditemukan")
	}

	return response.OK(c, map[string]string{"message": "Jam ritase berhasil dihapus"})
}

// ── POST /jadwal-config/driver-jenis — Tambah mapping driver→jenis ──

func (h *APIHandler) CreateDriverJenis(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	var req struct {
		IDDriver int64  `json:"id_driver"`
		RitaseKe int    `json:"ritase_ke"`
		Jenis    string `json:"jenis"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	if req.IDDriver <= 0 || req.RitaseKe <= 0 || req.Jenis == "" {
		return response.Error(c, http.StatusBadRequest, "id_driver, ritase_ke, jenis wajib diisi")
	}
	if req.Jenis != "outgoing" && req.Jenis != "incoming" {
		return response.Error(c, http.StatusBadRequest, "jenis harus 'outgoing' atau 'incoming'")
	}

	var id int
	err := h.DB.QueryRow(ctx, `
		INSERT INTO driver_ritase_jenis (id_driver, ritase_ke, jenis)
		VALUES ($1, $2, $3)
		ON CONFLICT (id_driver, ritase_ke) DO UPDATE SET jenis = $3
		RETURNING id
	`, req.IDDriver, req.RitaseKe, req.Jenis).Scan(&id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal tambah mapping driver: "+err.Error())
	}

	return response.OK(c, map[string]any{"id": id, "message": "Mapping driver→jenis berhasil disimpan"})
}

// ── DELETE /jadwal-config/driver-jenis/:id — Hapus mapping ──

func (h *APIHandler) DeleteDriverJenis(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	id := c.Param("id")
	tag, err := h.DB.Exec(ctx, "DELETE FROM driver_ritase_jenis WHERE id = $1", id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal hapus mapping: "+err.Error())
	}
	if tag.RowsAffected() == 0 {
		return response.Error(c, http.StatusNotFound, "Mapping tidak ditemukan")
	}

	return response.OK(c, map[string]string{"message": "Mapping driver→jenis berhasil dihapus"})
}

// ── POST /jadwal-config/template — Tambah route template ──

func (h *APIHandler) CreateRouteTemplate(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	var req struct {
		IDDriver    int64         `json:"id_driver"`
		IDKendaraan int64         `json:"id_kendaraan"`
		IDDropPoint int64         `json:"id_drop_point"`
		RitaseKe    int           `json:"ritase_ke"`
		JenisRitase string        `json:"jenis_ritase"`
		Stops       []FixedStop   `json:"stops"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	if req.IDDriver <= 0 || req.IDKendaraan <= 0 || req.IDDropPoint <= 0 || req.RitaseKe <= 0 {
		return response.Error(c, http.StatusBadRequest, "id_driver, id_kendaraan, id_drop_point, ritase_ke wajib diisi")
	}

	// Insert route template
	var id int
	err := h.DB.QueryRow(ctx, `
		INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		RETURNING id
	`, req.IDDriver, req.IDKendaraan, req.IDDropPoint, req.RitaseKe, req.JenisRitase).Scan(&id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal tambah route template: "+err.Error())
	}

	// Insert stops
	for _, stop := range req.Stops {
		if stop.KolomLokasi == "" {
			if stop.Jenis == "gudang" {
				stop.KolomLokasi = "id_gudang"
			} else if stop.Jenis == "seller" {
				stop.KolomLokasi = "id_seller"
			} else {
				stop.KolomLokasi = "id_drop_point"
			}
		}
		_, err := h.DB.Exec(ctx, `
			INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, id, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.KolomLokasi, stop.Keterangan)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal tambah stop: "+err.Error())
		}
	}

	return response.OK(c, map[string]any{"id": id, "message": "Route template berhasil ditambahkan"})
}

// ── PUT /jadwal-config/template/:id — Update route template ──

func (h *APIHandler) UpdateRouteTemplate(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	id := c.Param("id")
	var req struct {
		IDDriver    int64       `json:"id_driver"`
		IDKendaraan int64       `json:"id_kendaraan"`
		IDDropPoint int64       `json:"id_drop_point"`
		RitaseKe    int         `json:"ritase_ke"`
		JenisRitase string      `json:"jenis_ritase"`
		Aktif       bool        `json:"aktif"`
		Stops       []FixedStop `json:"stops"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	// Update route
	tag, err := h.DB.Exec(ctx, `
		UPDATE ritase_route_template
		SET id_driver = $1, id_kendaraan = $2, id_drop_point = $3, ritase_ke = $4, jenis_ritase = $5, aktif = $6
		WHERE id = $7
	`, req.IDDriver, req.IDKendaraan, req.IDDropPoint, req.RitaseKe, req.JenisRitase, req.Aktif, id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal update route template: "+err.Error())
	}
	if tag.RowsAffected() == 0 {
		return response.Error(c, http.StatusNotFound, "Route template tidak ditemukan")
	}

	// Hapus stops lama, insert baru
	_, err = h.DB.Exec(ctx, "DELETE FROM ritase_stop_template WHERE id_route_template = $1", id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal hapus stops lama: "+err.Error())
	}

	for _, stop := range req.Stops {
		if stop.KolomLokasi == "" {
			if stop.Jenis == "gudang" {
				stop.KolomLokasi = "id_gudang"
			} else if stop.Jenis == "seller" {
				stop.KolomLokasi = "id_seller"
			} else {
				stop.KolomLokasi = "id_drop_point"
			}
		}
		_, err := h.DB.Exec(ctx, `
			INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, id, stop.Urutan, stop.Jenis, stop.IDLokasi, stop.KolomLokasi, stop.Keterangan)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "Gagal tambah stop baru: "+err.Error())
		}
	}

	return response.OK(c, map[string]string{"message": "Route template berhasil diupdate"})
}

// ── DELETE /jadwal-config/template/:id — Hapus route template ──

func (h *APIHandler) DeleteRouteTemplate(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	id := c.Param("id")
	tag, err := h.DB.Exec(ctx, "DELETE FROM ritase_route_template WHERE id = $1", id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal hapus route template: "+err.Error())
	}
	if tag.RowsAffected() == 0 {
		return response.Error(c, http.StatusNotFound, "Route template tidak ditemukan")
	}

	return response.OK(c, map[string]string{"message": "Route template berhasil dihapus"})
}
