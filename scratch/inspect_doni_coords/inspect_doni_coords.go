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

	rows2, err := conn.Query(ctx, `
		SELECT e.id_event, e.status, e.latitude, e.longitude, e.created_at
		FROM ritase_event e
		JOIN ritase r ON e.id_ritase = r.id_ritase
		JOIN driver d ON r.id_driver = d.id_driver
		WHERE d.nama_driver ILIKE '%DONI%' AND r.tanggal = CURRENT_DATE
		ORDER BY e.created_at DESC
	`)
	if err == nil {
		fmt.Println("--- DONI RITASE 87 EXACT EVENT COORDS ---")
		for rows2.Next() {
			var id int64
			var st string
			var lat, lng *float64
			var at time.Time
			_ = rows2.Scan(&id, &st, &lat, &lng, &at)
			vLat, vLng := 0.0, 0.0
			if lat != nil {
				vLat = *lat
			}
			if lng != nil {
				vLng = *lng
			}
			fmt.Printf("Event %d | Status: %-25s | Lat/Lng: %f, %f | Time: %s\n", id, st, vLat, vLng, at.Format("15:04:05"))
		}
		rows2.Close()
	}
}
