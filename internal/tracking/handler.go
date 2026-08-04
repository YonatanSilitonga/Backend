package tracking

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk tracking.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// PostTracking menangani POST /driver/tracking.
func (h *Handler) PostTracking(c echo.Context) error {
	var req CreateTrackingRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.IDRitase == 0 || req.IDKendaraan == 0 || req.IDDriver == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase, id_kendaraan, id_driver wajib diisi")
	}

	data, err := h.svc.CreateTracking(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan tracking")
	}
	return response.OK(c, data)
}
