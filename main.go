package main

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"

	mobileAPI "backend/internal/api"
	"backend/internal/armada"
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/dashboard"
	"backend/internal/database"
	appJWT "backend/internal/pkg/jwt"
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

	// JWT manager (secret dari env, TTL 24 jam)
	jwtManager := appJWT.NewManager(cfg.JWTSecret, 24*time.Hour)

	// dependency injection per modul
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, jwtManager)
	authH := auth.NewHandler(authSvc)

	armadaRepo := armada.NewRepository(db)
	armadaSvc := armada.NewService(armadaRepo)
	armadaH := armada.NewHandler(armadaSvc)

	dashRepo := dashboard.NewRepository(db)
	dashSvc := dashboard.NewService(dashRepo)
	dashH := dashboard.NewHandler(dashSvc)

	e := echo.New()
	appMiddleware.Setup(e)

	// grup API v1
	api := e.Group("/api/v1")
	authMW := appMiddleware.Auth(jwtManager)

	// health check (tanpa auth)
	api.GET("/health", func(c echo.Context) error {
		if err := db.Ping(c.Request().Context()); err != nil {
			return response.Error(c, 503, "database tidak bisa dijangkau")
		}
		return response.OK(c, map[string]string{
			"status":  "up",
			"version": "0.2.0",
		})
	})

	// handler mobile (milik Whisnu) — endpoint public yang dipakai app driver
	apiH := mobileAPI.NewAPIHandler(db)
	api.GET("/sellers", apiH.GetSellers)
	api.GET("/drivers", apiH.GetDrivers)
	api.GET("/vehicles", apiH.GetVehicles)

	// route per modul (web + shared)
	authH.RegisterRoutes(api, authMW)
	armadaH.RegisterRoutes(api, authMW)
	dashH.RegisterRoutes(api, authMW)

	log.Printf("server jalan di :%s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
