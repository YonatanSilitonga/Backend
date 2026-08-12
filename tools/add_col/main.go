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

	// Alter table
	_, err = db.Exec(ctx, `
		ALTER TABLE ritase_event ADD COLUMN IF NOT EXISTS nama_lokasi VARCHAR(255);
		ALTER TABLE armada_tracking ADD COLUMN IF NOT EXISTS nama_lokasi VARCHAR(255);
	`)
	if err != nil {
		log.Fatalf("Gagal alter table: %v", err)
	}
	fmt.Println("Berhasil menambahkan kolom nama_lokasi ke ritase_event dan armada_tracking!")
}
