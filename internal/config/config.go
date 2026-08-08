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
	// Ambang offline armada (menit tanpa GPS terbaru). Default 15 menit —
	// menoleransi heartbeat hemat baterai (3 mnt) + retry jaringan.
	TrackingOfflineMin int
}

// Load membaca file .env (jika ada) lalu mengumpulkan konfigurasi dari environment.
func Load() *Config {
	_ = godotenv.Load() // abaikan error, env sistem tetap bisa dipakai

	return &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		TrackingOfflineMin: getEnvInt("TRACKING_OFFLINE_MIN", 15),
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
