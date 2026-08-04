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

	rows, err := db.Query(ctx, `SELECT id_kendaraan, plat_nomor FROM kendaraan LIMIT 5`)
	if err != nil {
		log.Fatalf("Gagal select: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== KENDARAAN ===")
	for rows.Next() {
		var id int
		var plat string
		if err := rows.Scan(&id, &plat); err == nil {
			fmt.Printf("ID: %d | Plat: %s\n", id, plat)
		}
	}
}
