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

	rows, err := db.Query(ctx, "SELECT id_driver, nama_driver, COALESCE(no_hp, ''), COALESCE(status_driver, '') FROM driver ORDER BY id_driver ASC")
	if err != nil {
		log.Fatalf("Gagal query tabel driver: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== DAFTAR DATA DRIVER DI DATABASE SUPABASE ===")
	count := 0
	for rows.Next() {
		var id int64
		var nama, hp, status string
		if err := rows.Scan(&id, &nama, &hp, &status); err == nil {
			count++
			fmt.Printf("%d. ID: %d | Nama: %s | HP: %s | Status: %s\n", count, id, nama, hp, status)
		}
	}
	fmt.Printf("\nTotal Data Driver Saat Ini: %d baris\n", count)
}
