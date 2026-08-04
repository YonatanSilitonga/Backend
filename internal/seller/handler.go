package seller

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk seller.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListSeller menangani GET /sellers.
func (h *Handler) ListSeller(c echo.Context) error {
	data, err := h.svc.ListSeller(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data seller")
	}
	return response.OK(c, data)
}
