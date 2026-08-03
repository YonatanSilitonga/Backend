package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config menyimpan semua konfigurasi aplikasi yang dibaca dari environment.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
}

// Load membaca file .env (jika ada) lalu mengumpulkan konfigurasi dari environment.
func Load() *Config {
	_ = godotenv.Load() // abaikan error, env sistem tetap bisa dipakai

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", "change-me-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
