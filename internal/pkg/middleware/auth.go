package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/jwt"
	"backend/internal/pkg/response"
)

const (
	// CtxUserID adalah key di Echo context untuk user ID dari token.
	CtxUserID = "auth_user_id"
	// CtxRole adalah key di Echo context untuk role dari token.
	CtxRole = "auth_role"
)

// Auth adalah middleware untuk memvalidasi JWT dari header Authorization.
func Auth(jwtManager *jwt.Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get(echo.HeaderAuthorization)
			if header == "" {
				return response.Error(c, http.StatusUnauthorized, "token tidak ditemukan")
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return response.Error(c, http.StatusUnauthorized, "format token tidak valid")
			}

			claims, err := jwtManager.Parse(parts[1])
			if err != nil {
				return response.Error(c, http.StatusUnauthorized, "token tidak valid atau kedaluwarsa")
			}

			c.Set(CtxUserID, claims.UserID)
			c.Set(CtxRole, claims.Role)

			return next(c)
		}
	}
}
