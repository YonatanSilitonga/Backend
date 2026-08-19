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

	rows, err := conn.Query(ctx, "SELECT id_seller, nama_seller, latitude, longitude FROM seller WHERE id_seller = 7 OR nama_seller ILIKE '%Cipadu%'")
	if err != nil {
		log.Fatalf("Query err: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== QUERY SELLER 7 / CIPADU IN DATABASE ===")
	for rows.Next() {
		var id int64
		var nama string
		var lat, lng float64
		_ = rows.Scan(&id, &nama, &lat, &lng)
		fmt.Printf("ID Seller: %d | Nama: %-30s | Lat/Lng: %f, %f\n", id, nama, lat, lng)
	}
}
