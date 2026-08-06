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
	// Load .env file
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

	// 1. Tambah kolom jumlah_koli dan jumlah_ecer ke ritase_event
	fmt.Println("Migrasi: Menambahkan kolom jumlah_koli & jumlah_ecer ke ritase_event...")
	_, err = db.Exec(ctx, `ALTER TABLE ritase_event ADD COLUMN IF NOT EXISTS jumlah_koli INT DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Gagal alter tabel ritase_event (jumlah_koli): %v", err)
	}
	_, err = db.Exec(ctx, `ALTER TABLE ritase_event ADD COLUMN IF NOT EXISTS jumlah_ecer INT DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Gagal alter tabel ritase_event (jumlah_ecer): %v", err)
	}

	// 2. Tambah kolom jumlah_ecer ke armada_tracking
	fmt.Println("Migrasi: Menambahkan kolom jumlah_ecer ke armada_tracking...")
	_, err = db.Exec(ctx, `ALTER TABLE armada_tracking ADD COLUMN IF NOT EXISTS jumlah_ecer INT DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Gagal alter tabel armada_tracking (jumlah_ecer): %v", err)
	}

	fmt.Println("🎉 Selesai! Kolom-kolom berhasil ditambahkan!")
}
