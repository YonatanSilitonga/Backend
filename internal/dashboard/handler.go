package dashboard

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk dashboard.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetSummary menangani GET /dashboard/summary (KPI ringkasan).
func (h *Handler) GetSummary(c echo.Context) error {
	data, err := h.svc.GetSummary(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil ringkasan dashboard")
	}
	return response.OK(c, data)
}

// GetAnalisis menangani GET /dashboard/analisis (durasi + bottleneck + alert).
func (h *Handler) GetAnalisis(c echo.Context) error {
	data, err := h.svc.GetAnalisis(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil analisis dashboard")
	}
	return response.OK(c, data)
}

// GetAnalyticsTrend menangani GET /dashboard/analytics/trend?from=&to=.
func (h *Handler) GetAnalyticsTrend(c echo.Context) error {
	data, err := h.svc.GetAnalyticsTrend(c.Request().Context(), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil trend analitik")
	}
	return response.OK(c, data)
}

// GetAnalyticsDrivers menangani GET /dashboard/analytics/drivers?from=&to=.
func (h *Handler) GetAnalyticsDrivers(c echo.Context) error {
	data, err := h.svc.GetAnalyticsDrivers(c.Request().Context(), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil performa driver")
	}
	return response.OK(c, data)
}

// GetAnalyticsSellers menangani GET /dashboard/analytics/sellers?from=&to=.
func (h *Handler) GetAnalyticsSellers(c echo.Context) error {
	data, err := h.svc.GetAnalyticsSellers(c.Request().Context(), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil analitik seller")
	}
	return response.OK(c, data)
}

// GetAnalyticsImplants menangani GET /dashboard/analytics/implants?from=&to=.
func (h *Handler) GetAnalyticsImplants(c echo.Context) error {
	data, err := h.svc.GetAnalyticsImplants(c.Request().Context(), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil analitik implant")
	}
	return response.OK(c, data)
}

// GetImplanDilayani menangani GET /dashboard/implan-dilayani?tanggal=.
func (h *Handler) GetImplanDilayani(c echo.Context) error {
	data, err := h.svc.GetImplanDilayani(c.Request().Context(), c.QueryParam("tanggal"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data implan dilayani")
	}
	return response.OK(c, data)
}

// GetImplanDilayaniDetail menangani GET /dashboard/implan-dilayani/:id?tanggal=.
func (h *Handler) GetImplanDilayaniDetail(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id implant tidak valid")
	}
	data, err := h.svc.GetImplanDilayaniDetail(c.Request().Context(), id, c.QueryParam("tanggal"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil detail implant")
	}
	return response.OK(c, data)
}

// GetSellerDilayani menangani GET /dashboard/seller-dilayani?tanggal=&from=&to=.
func (h *Handler) GetSellerDilayani(c echo.Context) error {
	data, err := h.svc.GetSellerDilayani(
		c.Request().Context(),
		c.QueryParam("tanggal"),
		c.QueryParam("from"),
		c.QueryParam("to"),
	)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data seller dilayani")
	}
	return response.OK(c, data)
}

// GetDropperDilayani menangani GET /dashboard/dropper-dilayani?tanggal=.
func (h *Handler) GetDropperDilayani(c echo.Context) error {
	data, err := h.svc.GetDropperDilayani(c.Request().Context(), c.QueryParam("tanggal"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data muatan dropper")
	}
	return response.OK(c, data)
}

// GetSellerDilayaniDetail menangani GET /dashboard/seller-dilayani/detail?key=&tanggal=.
func (h *Handler) GetSellerDilayaniDetail(c echo.Context) error {
	key := c.QueryParam("key")
	if key == "" {
		return response.Error(c, http.StatusBadRequest, "key seller wajib diisi")
	}
	data, err := h.svc.GetSellerDilayaniDetail(c.Request().Context(), key, c.QueryParam("tanggal"))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil detail seller")
	}
	return response.OK(c, data)
}

// RegisterRoutes memasang route dashboard di grup yang diberikan (butuh auth).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.GET("/dashboard/summary", h.GetSummary, authMW)
	g.GET("/dashboard/analisis", h.GetAnalisis, authMW)
	g.GET("/dashboard/analytics/trend", h.GetAnalyticsTrend, authMW)
	g.GET("/dashboard/analytics/drivers", h.GetAnalyticsDrivers, authMW)
	g.GET("/dashboard/analytics/sellers", h.GetAnalyticsSellers, authMW)
	g.GET("/dashboard/analytics/implants", h.GetAnalyticsImplants, authMW)
	g.GET("/dashboard/implan-dilayani", h.GetImplanDilayani, authMW)
	g.GET("/dashboard/implan-dilayani/:id", h.GetImplanDilayaniDetail, authMW)
	g.GET("/dashboard/seller-dilayani", h.GetSellerDilayani, authMW)
	g.GET("/dashboard/seller-dilayani/detail", h.GetSellerDilayaniDetail, authMW)
	g.GET("/dashboard/dropper-dilayani", h.GetDropperDilayani, authMW)
	g.PATCH("/dashboard/alerts/:id/resolve", h.ResolveAlert, authMW)
	g.GET("/dashboard/alerts/ritase/:id", h.GetAlertsByRitase, authMW)
}

// ResolveAlert menangani PATCH /dashboard/alerts/:id/resolve — menandai alert sebagai ditangani.
func (h *Handler) ResolveAlert(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id alert tidak valid")
	}
	userID, _ := c.Get(middleware.CtxUserID).(int64)
	if err := h.svc.ResolveAlert(c.Request().Context(), id, userID); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal resolve alert")
	}
	return response.OK(c, map[string]string{"message": "alert berhasil ditandai normal"})
}

// GetAlertsByRitase menangani GET /dashboard/alerts/ritase/:id — riwayat alert per ritase.
func (h *Handler) GetAlertsByRitase(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id ritase tidak valid")
	}
	data, err := h.svc.GetAlertsByRitase(c.Request().Context(), id)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat alert")
	}
	return response.OK(c, data)
}
