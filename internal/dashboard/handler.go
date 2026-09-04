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

// RegisterRoutes memasang route dashboard di grup yang diberikan (butuh auth).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.GET("/dashboard/summary", h.GetSummary, authMW)
	g.GET("/dashboard/analisis", h.GetAnalisis, authMW)
	g.GET("/dashboard/analytics/trend", h.GetAnalyticsTrend, authMW)
	g.GET("/dashboard/analytics/drivers", h.GetAnalyticsDrivers, authMW)
	g.GET("/dashboard/analytics/sellers", h.GetAnalyticsSellers, authMW)
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
