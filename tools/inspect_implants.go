package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("d:/Magang/Backend/.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://tower_user:MembangunEkonomiDesaUntukIndonesia1@127.0.0.1:5432/tower_control?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Koneksi gagal: %v", err)
	}
	defer conn.Close(ctx)

	// Cek tabel implant
	fmt.Println("=== TABEL IMPLANT ===")
	rows, err := conn.Query(ctx, `
		SELECT id_implant, COALESCE(kode_implant, '-'), COALESCE(nama_implant, '-'), COALESCE(kota, '-'), COALESCE(alamat, '-')
		FROM implant
		ORDER BY id_implant
	`)
	if err != nil {
		log.Fatalf("Query implant gagal: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var kode, nama, kota, alamat string
		_ = rows.Scan(&id, &kode, &nama, &kota, &alamat)
		fmt.Printf("ID: %3d | Kode: %-12s | Nama: %-30s | Kota: %-15s | Alamat: %s\n", id, kode, nama, kota, alamat)
	}
}

