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

	rows, err := db.Query(ctx, `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'ritase'
	`)
	if err != nil {
		log.Fatalf("Gagal select schema: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== SCHEMA ritase ===")
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err == nil {
			fmt.Printf("Column: %s | Type: %s\n", colName, dataType)
		}
	}
}
