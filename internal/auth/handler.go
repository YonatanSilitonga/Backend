package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

// Handler menyediakan endpoint HTTP untuk auth.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Login menangani POST /auth/login.
func (h *Handler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}

	res, err := h.svc.Login(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusUnauthorized, err.Error())
	}

	return response.OK(c, res)
}

// Me menangani GET /auth/me (butuh token).
func (h *Handler) Me(c echo.Context) error {
	userID := c.Get(appMiddleware.CtxUserID).(int64)

	user, err := h.svc.Me(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, http.StatusNotFound, err.Error())
	}

	return response.OK(c, user)
}

// Logout menangani POST /auth/logout (token stateless, client cukup hapus lokal).
func (h *Handler) Logout(c echo.Context) error {
	return response.OK(c, map[string]string{"status": "logged_out"})
}

// RegisterRoutes memasang route auth di grup yang diberikan.
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.POST("/auth/login", h.Login)
	g.GET("/auth/me", h.Me, authMW)
	g.POST("/auth/logout", h.Logout, authMW)
}
