package kendaraan

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk kendaraan.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListKendaraan menangani GET /vehicles.
func (h *Handler) ListKendaraan(c echo.Context) error {
	data, err := h.svc.ListKendaraan(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data kendaraan")
	}
	return response.OK(c, data)
}
