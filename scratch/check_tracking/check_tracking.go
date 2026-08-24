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
	godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(ctx, "SELECT id_kendaraan, id_driver, id_ritase, status FROM armada_tracking")
	if err != nil {
		log.Fatalf("Query gagal: %v", err)
	}
	defer rows.Close()

	fmt.Println("Kendaraan | Driver | Ritase | Status")
	fmt.Println("--------------------------------")
	for rows.Next() {
		var idK, idD, idR int64
		var status string
		if err := rows.Scan(&idK, &idD, &idR, &status); err != nil {
			log.Fatal(err)
		}
		
		fmt.Printf("%9d | %6d | %6d | %s\n", idK, idD, idR, status)
	}
}
