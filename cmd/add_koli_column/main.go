package main

import (
	"context"
	"fmt"
	"log"

	"backend/internal/config"
	"backend/internal/database"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Koneksi database gagal: %v", err)
	}
	defer db.Close()

	// Tambahkan kolom jumlah_koli ke tabel armada_tracking jika belum ada
	_, err = db.Exec(ctx, `ALTER TABLE armada_tracking ADD COLUMN IF NOT EXISTS jumlah_koli INT DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Gagal menambahkan kolom jumlah_koli: %v", err)
	}

	fmt.Println("🎉 Selesai! Kolom jumlah_koli berhasil ditambahkan ke tabel armada_tracking!")
}
