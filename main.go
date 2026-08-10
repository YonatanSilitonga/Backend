package main

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/armada"
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/dashboard"
	"backend/internal/database"
	"backend/internal/driver"
	"backend/internal/kendaraan"
	"backend/internal/mobile_api"
	appJWT "backend/internal/pkg/jwt"
	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
	"backend/internal/seller"
	"backend/internal/tracking"
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

	// modul web: armada + dashboard
	armadaRepo := armada.NewRepository(db)
	armadaSvc := armada.NewService(armadaRepo, cfg.TrackingOfflineMin, cfg.SessionOfflineHours)
	armadaH := armada.NewHandler(armadaSvc)

	dashRepo := dashboard.NewRepository(db, cfg.TrackingOfflineMin)
	dashSvc := dashboard.NewService(dashRepo)
	dashH := dashboard.NewHandler(dashSvc)

	// modul mobile: seller, driver, kendaraan, tracking
	sellerRepo := seller.NewRepository(db)
	sellerSvc := seller.NewService(sellerRepo)
	_ = sellerSvc

	driverRepo := driver.NewRepository(db)
	driverSvc := driver.NewService(driverRepo)
	_ = driverSvc

	kendaraanRepo := kendaraan.NewRepository(db)
	kendaraanSvc := kendaraan.NewService(kendaraanRepo)
	_ = kendaraanSvc

	trackingRepo := tracking.NewRepository(db)
	trackingSvc := tracking.NewService(trackingRepo)
	_ = trackingSvc

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

	// Inisialisasi API Handler Mobile milik Whisnu
	handler := mobile_api.NewAPIHandler(db)

	v1 := e.Group("/api/v1")
	v1.GET("/sellers", handler.GetSellers)
	v1.GET("/drivers", handler.GetDrivers)
	v1.GET("/vehicles", handler.GetVehicles)
	v1.GET("/driver/active-ritase", handler.GetActiveRitase)
	v1.POST("/driver/tracking", handler.PostTracking)
	v1.POST("/driver/start-free-trip", handler.StartFreeTrip)
	v1.POST("/driver/add-stop", handler.AddRitaseStop)

	// Admin Ritase Endpoints (Tower Control Web)
	v1.GET("/admin/ritases", handler.AdminGetRitases)
	v1.POST("/admin/ritase/generate", handler.AdminGenerateDailyRitase)
	v1.PUT("/admin/ritase/:id", handler.AdminUpdateRitase)
	v1.DELETE("/admin/ritase/:id", handler.AdminDeleteRitase)

	// ── ROUTE AUTH WEB (login JWT + me + logout) ──
	authH.RegisterRoutes(v1, authMW)

	// ── ROUTE WEB: ARMADA + DASHBOARD (butuh token JWT) ──
	armadaH.RegisterRoutes(v1, authMW)
	dashH.RegisterRoutes(v1, authMW)

	log.Printf("server jalan di :%s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
