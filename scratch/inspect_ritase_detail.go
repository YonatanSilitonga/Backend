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

	rows, err := conn.Query(ctx, `
		SELECT rs.id_stop, rs.urutan, rs.jenis_stop, rs.id_gudang, rs.id_seller, rs.id_drop_point,
		       g.nama_gudang, s.nama_seller, dp.nama_drop_point,
		       COALESCE(g.latitude, s.latitude, dp.latitude) as lat,
		       COALESCE(g.longitude, s.longitude, dp.longitude) as lng
		FROM ritase_stop rs
		LEFT JOIN gudang g ON rs.id_gudang = g.id_gudang
		LEFT JOIN seller s ON rs.id_seller = s.id_seller
		LEFT JOIN drop_point dp ON rs.id_drop_point = dp.id_drop_point
		WHERE rs.id_ritase = 87
		ORDER BY rs.urutan
	`)
	if err != nil {
		log.Fatalf("Query err: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== DETAIL STOPS UNTUK RITASE DONI (ID 87) ===")
	for rows.Next() {
		var idStop, urutan int
		var jenis string
		var idG, idS, idDp *int64
		var namaG, namaS, namaDp *string
		var lat, lng *float64
		_ = rows.Scan(&idStop, &urutan, &jenis, &idG, &idS, &idDp, &namaG, &namaS, &namaDp, &lat, &lng)
		nama := ""
		if namaG != nil {
			nama = *namaG
		} else if namaS != nil {
			nama = *namaS
		} else if namaDp != nil {
			nama = *namaDp
		}
		fmt.Printf("Stop %d | Jenis: %-10s | Nama: %-25s | Lat/Lng: %v,%v\n", urutan, jenis, nama, lat, lng)
	}
}
