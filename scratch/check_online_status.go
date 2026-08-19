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
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL tidak ada")
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Gagal connect DB: %v", err)
	}
	defer conn.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("=== CEK TIMESTAMPS ARMADA TRACKING ===")
	rows, err := conn.Query(ctx, `
		SELECT t.id_tracking, k.plat_nomor, d.nama_driver, t.last_update,
		       NOW() - t.last_update AS age,
		       (t.last_update < NOW() - INTERVAL '3 minutes') AS is_offline_3m
		FROM armada_tracking t
		LEFT JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
		LEFT JOIN driver d ON d.id_driver = t.id_driver
		ORDER BY t.last_update DESC
	`)
	if err == nil {
		for rows.Next() {
			var idTrk int
			var plat, drv string
			var lu time.Time
			var age time.Duration
			var isOffline bool
			_ = rows.Scan(&idTrk, &plat, &drv, &lu, &age, &isOffline)
			fmt.Printf("Veh=%s | Driver=%s | LastUpdate=%s | UmurPing=%v | IsOffline(>3m)=%v\n",
				plat, drv, lu.Format("15:04:05"), age.Round(time.Second), isOffline)
		}
		rows.Close()
	}
}
