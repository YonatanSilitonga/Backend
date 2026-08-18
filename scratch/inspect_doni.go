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
	_ = godotenv.Load("../.env")
	dbURL := os.Getenv("DATABASE_URL")
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Gagal connect: %v", err)
	}
	defer conn.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find Doni's active ritase today
	var idRitase int64
	var kode, status string
	err = conn.QueryRow(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.status
		FROM ritase r
		JOIN driver d ON r.id_driver = d.id_driver
		WHERE d.nama_driver ILIKE '%DONI%' AND r.tanggal = CURRENT_DATE
		ORDER BY r.id_ritase DESC LIMIT 1
	`).Scan(&idRitase, &kode, &status)

	if err != nil {
		log.Fatalf("Ritase Doni tidak ditemukan: %v", err)
	}

	fmt.Printf("=== RITASE DONI (ID: %d, Kode: %s, Status: %s) ===\n", idRitase, kode, status)

	// List stops
	rows, err := conn.Query(ctx, `
		SELECT urutan, jenis_stop, id_gudang, id_seller, id_drop_point, keterangan
		FROM ritase_stop WHERE id_ritase = $1 ORDER BY urutan
	`, idRitase)
	if err == nil {
		fmt.Println("\n--- STOPS ---")
		for rows.Next() {
			var u int
			var j, ket string
			var g, s, dp *int64
			_ = rows.Scan(&u, &j, &g, &s, &dp, &ket)
			fmt.Printf("Stop %d | Jenis: %-10s | Gudang: %v | Seller: %v | DP: %v | Ket: %s\n", u, j, g, s, dp, ket)
		}
		rows.Close()
	}

	// List events
	rows2, err := conn.Query(ctx, `
		SELECT id_event, status, latitude, longitude, created_at
		FROM ritase_event WHERE id_ritase = $1 ORDER BY created_at DESC
	`, idRitase)
	if err == nil {
		fmt.Println("\n--- EVENTS ---")
		for rows2.Next() {
			var id int64
			var st string
			var lat, lng *float64
			var at time.Time
			_ = rows2.Scan(&id, &st, &lat, &lng, &at)
			fmt.Printf("Event %d | Status: %-25s | Lat/Lng: %v,%v | Time: %s\n", id, st, lat, lng, at.Format("15:04:05"))
		}
		rows2.Close()
	}
}
