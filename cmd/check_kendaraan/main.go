package main

import (
	"context"
	"fmt"
	"log"

	"backend/internal/config"
	"backend/internal/database"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Koneksi database gagal: %v", err)
	}
	defer db.Close()

	// Query stops for ritase 7
	rows, err := db.Query(ctx, `
		SELECT 
			rs.id_stop, rs.urutan, rs.jenis_stop, 
			rs.id_seller, rs.id_drop_point, rs.id_gudang, rs.keterangan,
			COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Gudang OG') AS nama_lokasi
		FROM ritase_stop rs
		LEFT JOIN seller s ON s.id_seller = rs.id_seller
		LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
		LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
		WHERE rs.id_ritase = 7
		ORDER BY rs.urutan ASC
	`)
	if err != nil {
		log.Fatalf("Query error: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== STOPS FOR RITASE 7 ===")
	for rows.Next() {
		var idStop int64
		var urutan int
		var jenisStop, namaLokasi string
		var idSeller, idDropPoint, idGudang *int64
		var keterangan *string

		if err := rows.Scan(&idStop, &urutan, &jenisStop, &idSeller, &idDropPoint, &idGudang, &keterangan, &namaLokasi); err == nil {
			fmt.Printf("Urutan %d: %s (%s, seller_id: %v)\n", urutan, namaLokasi, jenisStop, idSeller)
		}
	}
}
