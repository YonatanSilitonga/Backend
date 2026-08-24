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
		dbURL = "postgres://postgres:postgres@localhost:5432/tower_control?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.tanggal, r.id_driver, d.nama_driver, r.ritase_ke, r.status
		FROM ritase r
		LEFT JOIN driver d ON r.id_driver = d.id_driver
		WHERE r.tanggal = CURRENT_DATE
		ORDER BY r.id_driver, r.id_ritase
	`)
	if err != nil {
		log.Fatalf("Query err: %v", err)
	}
	defer rows.Close()

	fmt.Println("--- LIST RITASE HARI INI ---")
	for rows.Next() {
		var id int64
		var kode, status string
		var tgl time.Time
		var idDriver int64
		var namaDriver *string
		var ritaseKe int
		_ = rows.Scan(&id, &kode, &tgl, &idDriver, &namaDriver, &ritaseKe, &status)
		driver := "Unknown"
		if namaDriver != nil {
			driver = *namaDriver
		}
		fmt.Printf("ID: %d | Kode: %-25s | Driver: %-15s (ID: %d) | RitaseKe: %d | Status: %s\n", id, kode, driver, idDriver, ritaseKe, status)
	}
}
