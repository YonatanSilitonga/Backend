package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config menyimpan semua konfigurasi aplikasi yang dibaca dari environment.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	// Ambang offline armada (menit tanpa GPS terbaru). Default 3 menit —
	// konsisten dengan threshold frontend (OFFLINE_MINUTES = 3).
	TrackingOfflineMin int
	// Ambang session online (jam sejak login tanpa aktivitas). Default 12 jam —
	// anti "hantu online" kalau driver force-stop/off tanpa logout.
	SessionOfflineHours int
	// Wajib session login aktif buat status LIVE/online (offline = GPS basi ATAU
	// gak login). Default false — nyalain (SESSION_REQUIRED=true) SETELAH app
	// mobile dipastikan sudah pakai alur login (/auth/login), biar tidak semua
	// armada jadi offline mendadak saat rollout.
	SessionRequired bool
	// Interval broadcast SSE realtime (ms). Default 3000 — push ke web dashboard
	// tiap N detik (summary + analisis + posisi armada).
	RealtimeIntervalMs int
	// TrackerAPIKey adalah secret bersama untuk endpoint GPS tracker
	// (POST /api/v1/tracker/gps). Tracker tidak login JWT, jadi wajib
	// kirim header X-Tracker-Key yang sama dengan nilai ini.
	TrackerAPIKey string
}

// Load membaca file .env (jika ada) lalu mengumpulkan konfigurasi dari environment.
func Load() *Config {
	_ = godotenv.Load() // abaikan error, env sistem tetap bisa dipakai

	return &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		TrackingOfflineMin:  getEnvInt("TRACKING_OFFLINE_MIN", 3),
		SessionOfflineHours: getEnvInt("SESSION_OFFLINE_HOURS", 12),
		SessionRequired:     getEnvBool("SESSION_REQUIRED", false),
		RealtimeIntervalMs:  getEnvInt("REALTIME_INTERVAL_MS", 3000),
		TrackerAPIKey:       getEnv("TRACKER_API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
