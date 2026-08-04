package main

import (
	"context"
	"log"

	"github.com/labstack/echo/v4"

	"backend/internal/config"
	"backend/internal/driver"
	"backend/internal/database"
	"backend/internal/mobile_api"
	"backend/internal/kendaraan"
	"backend/internal/seller"
	"backend/internal/tracking"
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

	handler := mobile_api.NewAPIHandler(db)

	v1 := e.Group("/api/v1")
	v1.POST("/auth/login", handler.Login)
	v1.GET("/sellers", sellerH.ListSeller)
	v1.GET("/drivers", driverH.ListDriver)
	v1.GET("/vehicles", kendaraanH.ListKendaraan)
	v1.POST("/driver/tracking", trackingH.PostTracking)

	log.Printf("server jalan di :%s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
