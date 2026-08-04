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

	// Ubah kolom id_ritase agar boleh NULL (opsional) di tabel armada_tracking
	_, err = db.Exec(ctx, `ALTER TABLE armada_tracking ALTER COLUMN id_ritase DROP NOT NULL;`)
	if err != nil {
		log.Fatalf("Gagal alter tabel armada_tracking: %v", err)
	}

	fmt.Println("🎉 Selesai! Kolom id_ritase pada tabel armada_tracking sekarang opsional (NULLABLE)!")
}
