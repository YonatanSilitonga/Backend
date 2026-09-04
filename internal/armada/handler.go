package armada

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"backend/internal/eventbus"
	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk modul armada.
type Handler struct {
	svc *Service
	bus *eventbus.Bus
}

func NewHandler(svc *Service, bus *eventbus.Bus) *Handler {
	return &Handler{svc: svc, bus: bus}
}

/* ---------- Master data ---------- */

// ListKendaraan menangani GET /armada/kendaraan.
func (h *Handler) ListKendaraan(c echo.Context) error {
	data, err := h.svc.ListKendaraan(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data kendaraan")
	}
	return response.OK(c, data)
}

// ListDriver menangani GET /armada/driver.
func (h *Handler) ListDriver(c echo.Context) error {
	data, err := h.svc.ListDriver(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data driver")
	}
	return response.OK(c, data)
}

/* ---------- Ritase / Penugasan ---------- */

// ListRitase menangani GET /armada/ritase?driver_id=&tanggal=
// Driver (role=driver) hanya melihat ritase miliknya sendiri (scoping dari JWT).
func (h *Handler) ListRitase(c echo.Context) error {
	driverID, _ := strconv.ParseInt(c.QueryParam("driver_id"), 10, 64)
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	tanggal := c.QueryParam("tanggal")
	if tanggal != "" && startDate == "" {
		startDate = tanggal // fallback for old frontend calls
	}

	if role, ok := c.Get(appMiddleware.CtxRole).(string); ok && role == "driver" {
		if did, ok := c.Get(appMiddleware.CtxDriverID).(int64); ok && did > 0 {
			driverID = did
		}
	}

	data, err := h.svc.ListRitase(c.Request().Context(), driverID, startDate, endDate)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data ritase")
	}
	return response.OK(c, data)
}

// GetRitase menangani GET /armada/ritase/:id (detail + timeline).
func (h *Handler) GetRitase(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id ritase tidak valid")
	}

	data, err := h.svc.GetRitase(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "ritase tidak ditemukan")
	}
	return response.OK(c, data)
}

// CreateRitase menangani POST /armada/ritase (penugasan oleh tower_control).
func (h *Handler) CreateRitase(c echo.Context) error {
	var req CreateRitaseRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.KodeRitase == "" || req.IDDriver == 0 || req.IDKendaraan == 0 {
		return response.Error(c, http.StatusBadRequest, "kode_ritase, id_driver, id_kendaraan wajib diisi")
	}

	createdBy, _ := c.Get(appMiddleware.CtxUserID).(int64)
	data, err := h.svc.CreateRitase(c.Request().Context(), req, createdBy)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat ritase")
	}
	return response.Created(c, data)
}

// UpdateStatus menangani POST /armada/ritase/:id/status (tombol 10 status driver).
func (h *Handler) UpdateStatus(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id ritase tidak valid")
	}

	var req UpdateStatusRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.Status == "" {
		return response.Error(c, http.StatusBadRequest, "status wajib diisi")
	}

	switch req.Status {
	case "mulai_loading":
		req.Status = "Bongkar Muat Barang"
	case "berangkat_gudang":
		req.Status = "Keluar Gudang"
	case "menuju_seller":
		req.Status = "Sedang Menuju"
	case "sampai_gudang", "tiba":
		req.Status = "tiba"
	case "selesai":
		req.Status = "Selesai"
	default:
		if strings.HasPrefix(req.Status, "Sedang Menuju") || strings.HasPrefix(req.Status, "Menuju ") {
			req.Status = "Sedang Menuju"
		} else if strings.HasPrefix(req.Status, "Tiba di ") || req.Status == "tiba" {
			req.Status = "tiba"
		}
	}

	data, err := h.svc.UpdateStatus(c.Request().Context(), id, req)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "ritase tidak ditemukan")
	}
	h.bus.Publish("force_refresh", "ritase_status_update")
	return response.Created(c, data)
}

// UpdateMuatan menangani PATCH /armada/ritase/:id/muatan (input AWB/Koli/tertinggal).
func (h *Handler) UpdateMuatan(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id ritase tidak valid")
	}

	var req UpdateMuatanRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}

	data, err := h.svc.UpdateMuatan(c.Request().Context(), id, req)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "ritase tidak ditemukan")
	}
	h.bus.Publish("force_refresh", "ritase_muatan_update")
	return response.OK(c, data)
}

/* ---------- Tracking ---------- */

// ListTracking menangani GET /armada/tracking.
func (h *Handler) ListTracking(c echo.Context) error {
	data, err := h.svc.ListTracking(c.Request().Context(), 50)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data tracking")
	}
	return response.OK(c, data)
}

// CreateTracking menangani POST /armada/tracking (kirim posisi GPS dari driver).
func (h *Handler) CreateTracking(c echo.Context) error {
	var req CreateTrackingRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.IDRitase == 0 || req.IDKendaraan == 0 || req.IDDriver == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase, id_kendaraan, id_driver wajib diisi")
	}

	data, err := h.svc.CreateTracking(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan posisi")
	}
	h.bus.Publish("force_refresh", "tracking_update")
	return response.Created(c, data)
}

// GetTrackingMap menangani GET /armada/tracking/map (posisi live + lokasi seller).
func (h *Handler) GetTrackingMap(c echo.Context) error {
	data, err := h.svc.GetTrackingMap(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data peta tracking")
	}
	return response.OK(c, data)
}

// GetTrackingHistory menangani GET /armada/tracking/history?kendaraan_id=&driver_id=&tanggal=
func (h *Handler) GetTrackingHistory(c echo.Context) error {
	idKendaraan, _ := strconv.ParseInt(c.QueryParam("kendaraan_id"), 10, 64)
	idDriver, _ := strconv.ParseInt(c.QueryParam("driver_id"), 10, 64)
	if idKendaraan <= 0 && idDriver <= 0 {
		return response.Error(c, http.StatusBadRequest, "kendaraan_id atau driver_id wajib diisi")
	}
	tanggal := c.QueryParam("tanggal")

	data, err := h.svc.GetTrackingHistory(c.Request().Context(), idKendaraan, idDriver, tanggal)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat tracking")
	}
	return response.OK(c, data)
}

// GetGpsHistory menangani GET /armada/ritase/:id/gps-history — titik GPS history per ritase.
func (h *Handler) GetGpsHistory(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id ritase tidak valid")
	}
	data, err := h.svc.GetGpsHistory(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil GPS history")
	}
	return response.OK(c, data)
}

// RegisterRoutes memasang route armada di grup yang diberikan (butuh auth).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.GET("/armada/kendaraan", h.ListKendaraan, authMW)
	g.GET("/armada/driver", h.ListDriver, authMW)

	g.GET("/armada/ritase", h.ListRitase, authMW)
	g.GET("/armada/ritase/:id", h.GetRitase, authMW)
	g.GET("/armada/ritase/:id/gps-history", h.GetGpsHistory, authMW)
	g.POST("/armada/ritase", h.CreateRitase, authMW)
	g.POST("/armada/ritase/:id/status", h.UpdateStatus, authMW)
	g.PATCH("/armada/ritase/:id/muatan", h.UpdateMuatan, authMW)

	g.GET("/armada/tracking", h.ListTracking, authMW)
	g.POST("/armada/tracking", h.CreateTracking, authMW)
	g.GET("/armada/tracking/map", h.GetTrackingMap, authMW)
	g.GET("/armada/tracking/history", h.GetTrackingHistory, authMW)
}
