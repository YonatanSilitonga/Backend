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

	var dpCount int
	err = db.QueryRow(ctx, "SELECT count(*) FROM drop_point WHERE id_drop_point = 1").Scan(&dpCount)
	if err != nil {
		log.Fatal(err)
	}

	var kCount int
	err = db.QueryRow(ctx, "SELECT count(*) FROM kendaraan WHERE id_kendaraan = 2").Scan(&kCount)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Drop Point 1: %d\n", dpCount)
	fmt.Printf("Kendaraan 2: %d\n", kCount)
}
