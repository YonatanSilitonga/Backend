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
	_ = godotenv.Load("../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL tidak diset!")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal db: %v", err)
	}
	defer db.Close()

	// 1. Hapus ritase_event dari ritase hari ini yang berstatus 'direncanakan' atau duplikat
	res1, err := db.Exec(ctx, `
		DELETE FROM ritase_event 
		WHERE id_ritase IN (
			SELECT id_ritase FROM ritase 
			WHERE tanggal = CURRENT_DATE AND (status = 'direncanakan' OR kode_ritase LIKE '%-%-%')
		)
	`)
	if err != nil {
		log.Printf("Err delete ritase_event: %v", err)
	} else {
		fmt.Printf("Terhapus %d ritase_event duplikat/direncanakan.\n", res1.RowsAffected())
	}

	// 2. Hapus ritase_stop dari ritase hari ini yang berstatus 'direncanakan' atau duplikat
	res2, err := db.Exec(ctx, `
		DELETE FROM ritase_stop 
		WHERE id_ritase IN (
			SELECT id_ritase FROM ritase 
			WHERE tanggal = CURRENT_DATE AND (status = 'direncanakan' OR kode_ritase LIKE '%-%-%')
		)
	`)
	if err != nil {
		log.Printf("Err delete ritase_stop: %v", err)
	} else {
		fmt.Printf("Terhapus %d ritase_stop duplikat/direncanakan.\n", res2.RowsAffected())
	}

	// 3. Hapus armada_tracking dari ritase hari ini yang berstatus 'direncanakan' atau duplikat
	res3, err := db.Exec(ctx, `
		DELETE FROM armada_tracking 
		WHERE id_ritase IN (
			SELECT id_ritase FROM ritase 
			WHERE tanggal = CURRENT_DATE AND (status = 'direncanakan' OR kode_ritase LIKE '%-%-%')
		)
	`)
	if err != nil {
		log.Printf("Err delete armada_tracking: %v", err)
	} else {
		fmt.Printf("Terhapus %d armada_tracking duplikat/direncanakan.\n", res3.RowsAffected())
	}

	// 4. Hapus ritase duplikat / direncanakan hari ini
	res4, err := db.Exec(ctx, `
		DELETE FROM ritase 
		WHERE tanggal = CURRENT_DATE AND (status = 'direncanakan' OR kode_ritase LIKE '%-%-%')
	`)
	if err != nil {
		log.Fatalf("Err delete ritase: %v", err)
	} else {
		fmt.Printf("🎉 Terhapus %d ritase duplikat/direncanakan hari ini!\n", res4.RowsAffected())
	}
}
