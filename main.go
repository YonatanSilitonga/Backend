package main

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/driver"
	"backend/internal/kendaraan"
	"backend/internal/seller"
	"backend/internal/tracking"
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

	// modul auth (web): login JWT + me + logout
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, jwtManager)
	authH := auth.NewHandler(authSvc)
	authMW := appMiddleware.Auth(jwtManager)

	// modul mobile: seller, driver, kendaraan, tracking
	sellerRepo := seller.NewRepository(db)
	sellerSvc := seller.NewService(sellerRepo)
	sellerH := seller.NewHandler(sellerSvc)

	driverRepo := driver.NewRepository(db)
	driverSvc := driver.NewService(driverRepo)
	driverH := driver.NewHandler(driverSvc)

	kendaraanRepo := kendaraan.NewRepository(db)
	kendaraanSvc := kendaraan.NewService(kendaraanRepo)
	kendaraanH := kendaraan.NewHandler(kendaraanSvc)

	trackingRepo := tracking.NewRepository(db)
	trackingSvc := tracking.NewService(trackingRepo)
	trackingH := tracking.NewHandler(trackingSvc)
	_ = trackingH

	e := echo.New()
	appMiddleware.Setup(e)

	// health check: bukti server hidup & database nyambung
	e.GET("/health", func(c echo.Context) error {
		if err := db.Ping(c.Request().Context()); err != nil {
			return response.Error(c, 503, "database tidak bisa dijangkau")
		}
		return response.OK(c, map[string]string{
			"status":  "up",
			"version":  "0.1.0",
		})
	})

	v1 := e.Group("/api/v1")

	// ── ROUTE MOBILE (dijaga utuh — jangan diubah) ──
	v1.GET("/sellers", sellerH.ListSeller)
	v1.GET("/drivers", driverH.ListDriver)
	v1.GET("/vehicles", kendaraanH.ListKendaraan)
	v1.POST("/driver/tracking", handler.PostTracking)

	// ── ROUTE AUTH WEB (login JWT + me + logout) ──
	authH.RegisterRoutes(v1, authMW)
	//   POST /auth/login  (public; terima username ATAU email; balikin JWT)
	//   GET  /auth/me     (butuh token)
	//   POST /auth/logout (butuh token)

	log.Printf("server jalan di :%s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
