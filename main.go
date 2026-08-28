package main

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/armada"
	adminpkg "backend/internal/admin"
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/dashboard"
	"backend/internal/database"
	"backend/internal/driver"
	"backend/internal/eventbus"
	"backend/internal/kendaraan"
	"backend/internal/mobile_api"
	appJWT "backend/internal/pkg/jwt"
	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
	"backend/internal/realtime"
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

	// Ensure database schema columns exist
	_, _ = db.Exec(ctx, `
		ALTER TABLE ritase_event ADD COLUMN IF NOT EXISTS nama_lokasi VARCHAR(255);
		ALTER TABLE armada_tracking ADD COLUMN IF NOT EXISTS nama_lokasi VARCHAR(255);
	`)

	// Auto-recalculate armada_tracking muatan dari ritase_event (sync data)
	if _, err := db.Exec(ctx, `
		UPDATE armada_tracking at SET
			jumlah_koli = COALESCE(sub.koli, 0),
			jumlah_ecer = COALESCE(sub.ecer, 0),
			jumlah_high_value = COALESCE(sub.hv, 0)
		FROM (
			SELECT id_ritase,
			       sum(jumlah_koli) AS koli,
			       sum(jumlah_ecer) AS ecer,
			       sum(jumlah_high_value) AS hv
			FROM ritase_event
			WHERE status = 'Bongkar Muat Barang'
			GROUP BY id_ritase
		) sub
		WHERE at.id_ritase = sub.id_ritase
	`); err != nil {
		log.Printf("[STARTUP] warning: gagal sync muatan armada_tracking: %v", err)
	} else {
		log.Println("[STARTUP] armada_tracking muatan tersync dari ritase_event")
	}

	// Event bus untuk komunikasi instan antar modul
	eventBus := eventbus.New()

	// JWT manager (secret dari env, TTL 24 jam)
	jwtManager := appJWT.NewManager(cfg.JWTSecret, 24*time.Hour)

	// modul auth (web): login JWT + me + logout
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, jwtManager)
	authH := auth.NewHandler(authSvc, eventBus)
	authMW := appMiddleware.Auth(jwtManager)

	// modul web: armada + dashboard
	armadaRepo := armada.NewRepository(db)
	armadaSvc := armada.NewService(armadaRepo, cfg.TrackingOfflineMin, cfg.SessionOfflineHours, cfg.SessionRequired)
	armadaH := armada.NewHandler(armadaSvc, eventBus)

	dashRepo := dashboard.NewRepository(db, cfg.TrackingOfflineMin, cfg.SessionOfflineHours, cfg.SessionRequired)
	dashSvc := dashboard.NewService(dashRepo)
	dashH := dashboard.NewHandler(dashSvc)

	// ── REALTIME SSE: gabung summary + analisis + map jadi 1 snapshot ──
	rtProvider := &liveProvider{dash: dashSvc, arm: armadaSvc}
	rtHub := realtime.NewHub(rtProvider, time.Duration(cfg.RealtimeIntervalMs)*time.Millisecond, eventBus)
	rtH := realtime.NewHandler(rtHub)

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
	handler := mobile_api.NewAPIHandler(db, eventBus, cfg.TrackerAPIKey)

	v1 := e.Group("/api/v1")
	v1.GET("/sellers", handler.GetSellers)
	v1.GET("/drivers", handler.GetDrivers)
	v1.GET("/vehicles", handler.GetVehicles)
	v1.GET("/app/version", handler.GetAppVersion)

	// Serve static uploads (manifest photos, etc.)
	e.Static("/uploads", "./uploads")

	// Endpoint driver mobile — WAJIB JWT (Authorization: Bearer). Tanpa token → 401.
	// Ini menutup celah "ghost GPS": siapa pun tidak bisa mengirim posisi palsu.
	// Catatan: app mobile harus login via /auth/login dulu untuk dapat JWT.
	v1.GET("/driver/active-ritase", handler.GetActiveRitase, authMW)
	v1.POST("/driver/tracking", handler.PostTracking, authMW)
	v1.POST("/driver/start-free-trip", handler.StartFreeTrip, authMW)
	v1.POST("/driver/add-stop", handler.AddRitaseStop, authMW)
	v1.POST("/driver/finish-ritase", handler.FinishRitase, authMW)
	v1.POST("/driver/trip-status", handler.PostTripStatus, authMW)
	v1.POST("/driver/upload-manifest", handler.UploadManifest, authMW)
	v1.POST("/driver/reset-test-ritase", handler.ResetTestRitase, authMW)
	v1.GET("/driver/history-ritase", handler.GetDriverHistoryRitase, authMW)
	v1.GET("/driver/history-ritase/:id", handler.GetDriverHistoryDetail, authMW)

	// GPS tracker hardware — sumber posisi cadangan saat HP mati.
	// Tanpa JWT (device tidak login), dilindungi header X-Tracker-Key.
	v1.POST("/tracker/gps", handler.PostTrackerGPS)

	// Admin Ritase Endpoints (Tower Control Web) — butuh JWT + role admin
	adminMW := []echo.MiddlewareFunc{authMW, appMiddleware.RequireRoles("admin")}

	// modul admin: CRUD master data + user management
	adminRepo := adminpkg.NewRepository(db)
	adminSvc := adminpkg.NewService(adminRepo)
	adminH := adminpkg.NewHandler(adminSvc)

	// ── READ-ONLY RITASE ENDPOINTS (semua role web bisa baca) ──
	v1.GET("/ritases", handler.AdminGetRitases, authMW)
	v1.GET("/master-options", handler.AdminGetMasterOptions, authMW)
	v1.GET("/manifest-photos", handler.AdminGetManifestPhotos, authMW)
	v1.GET("/ritase/generate/preview", handler.AdminPreviewGenerateDailyRitase, authMW)

	// ── GROUP ADMIN (tulis hapus ritase + CRUD baru) ──
	admin := v1.Group("/admin", adminMW...)
	admin.POST("/ritase/generate", handler.AdminGenerateDailyRitase)
	admin.POST("/ritase", handler.AdminCreateRitase)
	admin.PUT("/ritase/:id", handler.AdminUpdateRitase)
	admin.DELETE("/ritase/:id", handler.AdminDeleteRitase)

	// Admin CRUD master data + user management
	adminH.RegisterRoutes(admin)

	// ── ROUTE AUTH WEB (login JWT + me + logout) ──
	authH.RegisterRoutes(v1, authMW)

	// ── ROUTE WEB: ARMADA + DASHBOARD (butuh token JWT) ──
	armadaH.RegisterRoutes(v1, authMW)
	dashH.RegisterRoutes(v1, authMW)

	// ── REALTIME SSE (butuh token JWT) ──
	v1.GET("/realtime/live", rtH.Stream, authMW)
	go rtHub.Run(ctx)

	log.Printf("server jalan di :%s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}

// liveProvider menggabungkan snapshot dashboard + armada untuk SSE.
type liveProvider struct {
	dash *dashboard.Service
	arm  *armada.Service
}

func (p *liveProvider) GetSnapshot(ctx context.Context) (map[string]any, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	summary, err := p.dash.GetSummary(queryCtx)
	if err != nil {
		return nil, err
	}

	analisis, err := p.dash.GetAnalisis(queryCtx)
	if err != nil {
		return nil, err
	}

	tracking, err := p.arm.GetTrackingMap(queryCtx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"summary":  summary,
		"analisis": analisis,
		"map":      tracking,
	}, nil
}
