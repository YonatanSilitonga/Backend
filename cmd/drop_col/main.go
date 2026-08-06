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

	query := `ALTER TABLE ritase DROP COLUMN IF EXISTS id_seller;`
	
	_, err = db.Exec(ctx, query)
	if err != nil {
		log.Fatalf("Failed to drop column: %v", err)
	}

	fmt.Println("Successfully dropped 'id_seller' column from 'ritase' table.")
}
