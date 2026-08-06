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

	var idRitase int64
	err = db.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, id_driver, id_kendaraan, status, created_at, tanggal)
		VALUES ($1, $2, $3, 'mulai_loading', NOW(), CURRENT_DATE)
		RETURNING id_ritase
	`, "TEST-123", 3, 2).Scan(&idRitase)

	if err != nil {
		log.Fatalf("INSERT ERROR: %v", err)
	}

	fmt.Printf("Inserted id: %d\n", idRitase)
}
