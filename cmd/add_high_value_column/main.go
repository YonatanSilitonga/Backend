package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("⚠️  Gagal membaca file .env, menggunakan environment variables default.")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL tidak diset!")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal koneksi ke database: %v", err)
	}
	defer db.Close()

	fmt.Println("Migrasi: Menambahkan kolom jumlah_high_value ke ritase_event...")
	_, err = db.Exec(ctx, `ALTER TABLE ritase_event ADD COLUMN IF NOT EXISTS jumlah_high_value INT DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Gagal alter tabel ritase_event (jumlah_high_value): %v", err)
	}

	fmt.Println("Migrasi: Menambahkan kolom jumlah_high_value ke armada_tracking...")
	_, err = db.Exec(ctx, `ALTER TABLE armada_tracking ADD COLUMN IF NOT EXISTS jumlah_high_value INT DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Gagal alter tabel armada_tracking (jumlah_high_value): %v", err)
	}

	fmt.Println("🎉 Selesai! Kolom jumlah_high_value berhasil ditambahkan ke ritase_event dan armada_tracking!")
}
