package dashboard

import (
	"net/http"

	"github.com/labstack/echo/v4"

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

// RegisterRoutes memasang route dashboard di grup yang diberikan (butuh auth).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.GET("/dashboard/summary", h.GetSummary, authMW)
	g.GET("/dashboard/analisis", h.GetAnalisis, authMW)
}
