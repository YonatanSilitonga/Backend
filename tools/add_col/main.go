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
		ALTER TABLE implan_barang_log ADD COLUMN IF NOT EXISTS koli INTEGER DEFAULT 0;
		ALTER TABLE implan_barang_log ADD COLUMN IF NOT EXISTS ecer INTEGER DEFAULT 0;
		ALTER TABLE implan_barang_log ADD COLUMN IF NOT EXISTS high_value INTEGER DEFAULT 0;
	`)
	if err != nil {
		log.Fatalf("Gagal alter table: %v", err)
	}
	fmt.Println("Berhasil menambahkan kolom koli, ecer, dan high_value ke implan_barang_log!")
}
