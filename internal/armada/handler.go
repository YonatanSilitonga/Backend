package armada

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk modul armada.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
	tanggal := c.QueryParam("tanggal")

	if role, ok := c.Get(appMiddleware.CtxRole).(string); ok && role == "driver" {
		if did, ok := c.Get(appMiddleware.CtxDriverID).(int64); ok && did > 0 {
			driverID = did
		}
	}

	data, err := h.svc.ListRitase(c.Request().Context(), driverID, tanggal)
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

// CreateRitase menangani POST /armada/ritase (penugasan oleh kapten).
func (h *Handler) CreateRitase(c echo.Context) error {
	var req CreateRitaseRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.KodeRitase == "" || req.IDDriver == 0 || req.IDKendaraan == 0 || req.IDSeller == 0 {
		return response.Error(c, http.StatusBadRequest, "kode_ritase, id_driver, id_kendaraan, id_seller wajib diisi")
	}

	data, err := h.svc.CreateRitase(c.Request().Context(), req)
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

	data, err := h.svc.UpdateStatus(c.Request().Context(), id, req)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "ritase tidak ditemukan")
	}
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

// GetTrackingHistory menangani GET /armada/tracking/history?kendaraan_id=
func (h *Handler) GetTrackingHistory(c echo.Context) error {
	idKendaraan, _ := strconv.ParseInt(c.QueryParam("kendaraan_id"), 10, 64)
	if idKendaraan <= 0 {
		return response.Error(c, http.StatusBadRequest, "kendaraan_id wajib diisi")
	}

	data, err := h.svc.GetTrackingHistory(c.Request().Context(), idKendaraan)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat tracking")
	}
	return response.OK(c, data)
}

// RegisterRoutes memasang route armada di grup yang diberikan (butuh auth).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.GET("/armada/kendaraan", h.ListKendaraan, authMW)
	g.GET("/armada/driver", h.ListDriver, authMW)

	g.GET("/armada/ritase", h.ListRitase, authMW)
	g.GET("/armada/ritase/:id", h.GetRitase, authMW)
	g.POST("/armada/ritase", h.CreateRitase, authMW)
	g.POST("/armada/ritase/:id/status", h.UpdateStatus, authMW)
	g.PATCH("/armada/ritase/:id/muatan", h.UpdateMuatan, authMW)

	g.GET("/armada/tracking", h.ListTracking, authMW)
	g.POST("/armada/tracking", h.CreateTracking, authMW)
	g.GET("/armada/tracking/map", h.GetTrackingMap, authMW)
	g.GET("/armada/tracking/history", h.GetTrackingHistory, authMW)
}
