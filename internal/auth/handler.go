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

// Logout menangani POST /auth/logout (hapus penanda session online).
func (h *Handler) Logout(c echo.Context) error {
	userID := c.Get(appMiddleware.CtxUserID).(int64)
	_ = h.svc.Logout(c.Request().Context(), userID)
	return response.OK(c, map[string]string{"status": "logged_out"})
}

// OpenApp menangani POST /driver/open — catat kapan app mobile dibuka.
func (h *Handler) OpenApp(c echo.Context) error {
	userID := c.Get(appMiddleware.CtxUserID).(int64)
	if err := h.svc.OpenApp(c.Request().Context(), userID); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mencatat aktivitas app")
	}
	return response.OK(c, map[string]string{"status": "ok"})
}

// ChangePassword menangani POST /auth/change-password (butuh JWT).
func (h *Handler) ChangePassword(c echo.Context) error {
	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	userID := c.Get(appMiddleware.CtxUserID).(int64)
	if err := h.svc.ChangePassword(c.Request().Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}
	return response.OK(c, map[string]string{"message": "Password berhasil diubah"})
}

// ResetPassword menangani POST /auth/reset-password (lupa password, tanpa OTP).
func (h *Handler) ResetPassword(c echo.Context) error {
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if err := h.svc.ResetPassword(c.Request().Context(), req.Username, req.NoHP, req.NewPassword); err != nil {
		return response.Error(c, http.StatusBadRequest, err.Error())
	}
	return response.OK(c, map[string]string{"message": "Password berhasil direset. Silakan login ulang."})
}

// RegisterRoutes memasang route auth di grup yang diberikan.
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	g.POST("/auth/login", h.Login)
	g.GET("/auth/me", h.Me, authMW)
	g.POST("/auth/logout", h.Logout, authMW)
	g.POST("/driver/open", h.OpenApp, authMW) // telemetry app dibuka (mobile)
	g.POST("/auth/change-password", h.ChangePassword, authMW)
	g.POST("/auth/reset-password", h.ResetPassword) // lupa password tanpa OTP (mobile)
}
