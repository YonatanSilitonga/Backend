package driver

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk driver.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListDriver menangani GET /drivers.
func (h *Handler) ListDriver(c echo.Context) error {
	data, err := h.svc.ListDriver(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data driver")
	}
	return response.OK(c, data)
}
