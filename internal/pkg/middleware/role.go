package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

// RequireRoles membatasi akses hanya untuk role tertentu (diambil dari JWT).
// Contoh: RequireRoles("direktur", "tower_control") → role lain dapat 403.
func RequireRoles(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get(CtxRole).(string)
			if !allowed[role] {
				return response.Error(c, http.StatusForbidden, "akses ditolak: peran tidak diizinkan")
			}
			return next(c)
		}
	}
}
