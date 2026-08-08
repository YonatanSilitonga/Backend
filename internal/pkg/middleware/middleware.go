package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Setup memasang middleware dasar untuk aplikasi Echo:
// Logger, Recover, dan CORS.
func Setup(e *echo.Echo) {
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		// "*" biar web lokal (localhost:3000) & web via ngrok/Vercel semua bisa.
		// Endpoint web dilindungi JWT, jadi aman untuk MVP.
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			echo.GET, echo.POST, echo.PATCH, echo.PUT, echo.DELETE, echo.OPTIONS,
		},
		AllowHeaders: []string{
			echo.HeaderAuthorization,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderOrigin,
			"ngrok-skip-browser-warning", // wajib buat request browser lewat ngrok-free
		},
	}))
}
