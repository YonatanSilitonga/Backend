package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := "postgresql://postgres.sxumenxsjbtxphostgfp:MembangunEkonomiDesaUntukIndonesia1@aws-1-ap-northeast-2.pooler.supabase.com:5432/postgres"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(ctx, "SELECT id_user, username, id_driver FROM users")
	if err != nil {
		log.Fatalf("Query gagal: %v", err)
	}
	defer rows.Close()

	fmt.Println("ID User | Username | ID Driver")
	fmt.Println("--------------------------------")
	for rows.Next() {
		var idUser int64
		var username string
		var idDriver *int64
		if err := rows.Scan(&idUser, &username, &idDriver); err != nil {
			log.Fatal(err)
		}
		
		idDrv := "NULL"
		if idDriver != nil {
			idDrv = fmt.Sprintf("%d", *idDriver)
		}
		fmt.Printf("%7d | %-10s | %s\n", idUser, username, idDrv)
	}
}
