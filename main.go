package main

import (
	"context"
	"log"

	"github.com/labstack/echo/v4"

	"backend/internal/api"
	"backend/internal/config"
	"backend/internal/database"
	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	// koneksi ke Supabase (PostgreSQL)
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("koneksi database gagal: %v", err)
	}
	defer db.Close()
	log.Println("berhasil konek ke database")

	e := echo.New()
	appMiddleware.Setup(e)

	// health check: bukti server hidup & database nyambung
	e.GET("/health", func(c echo.Context) error {
		if err := db.Ping(c.Request().Context()); err != nil {
			return response.Error(c, 503, "database tidak bisa dijangkau")
		}
		return response.OK(c, map[string]string{
			"status":  "up",
			"version": "0.1.0",
		})
	})

	handler := api.NewAPIHandler(db)

	v1 := e.Group("/api/v1")
	v1.POST("/auth/login", handler.Login)
	v1.GET("/sellers", handler.GetSellers)
	v1.GET("/drivers", handler.GetDrivers)
	v1.GET("/vehicles", handler.GetVehicles)

	log.Printf("server jalan di :%s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
