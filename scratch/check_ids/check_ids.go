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

	rows, err := conn.Query(ctx, "SELECT id_gudang, nama_gudang FROM gudang")
	if err != nil {
		log.Fatalf("Query gudang err: %v", err)
	}
	fmt.Println("=== ID GUDANG VALID DI DATABASE SUPABASE ===")
	for rows.Next() {
		var id int64
		var nama string
		_ = rows.Scan(&id, &nama)
		fmt.Printf("ID Gudang: %d -> Nama: %s\n", id, nama)
	}
	rows.Close()

	rows2, err := conn.Query(ctx, "SELECT id_drop_point, nama_drop_point FROM drop_point")
	if err == nil {
		fmt.Println("\n=== ID DROP POINT / GATEWAY VALID ===")
		for rows2.Next() {
			var id int64
			var nama string
			_ = rows2.Scan(&id, &nama)
			fmt.Printf("ID DropPoint: %d -> Nama: %s\n", id, nama)
		}
		rows2.Close()
	}
}
