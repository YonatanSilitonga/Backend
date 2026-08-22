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

	var idRitase int64
	var statusRitase, kodeRitase string
	err = db.QueryRow(ctx, `
		SELECT id_ritase, status, kode_ritase
		FROM ritase
		WHERE status != 'selesai' AND (tanggal = (now() AT TIME ZONE 'Asia/Jakarta')::date OR tanggal IS NULL)
		ORDER BY id_ritase ASC
		LIMIT 1
	`).Scan(&idRitase, &statusRitase, &kodeRitase)

	if err != nil {
		log.Fatalf("Gagal query ritase: %v", err)
	}
	fmt.Printf("Aktif Ritase ID: %d, Status: %s, Kode: %s\n", idRitase, statusRitase, kodeRitase)

	rows, err := db.Query(ctx, `
		SELECT 
			rs.id_stop, rs.urutan, rs.jenis_stop, 
			rs.id_seller, rs.id_drop_point, rs.id_gudang, rs.keterangan,
			COALESCE(s.nama_seller, dp.nama_drop_point, g.nama_gudang, 'Gudang OG') AS nama_lokasi,
			COALESCE(s.alamat, dp.alamat, g.alamat, 'Gudang Outgoing Utama') AS alamat,
			COALESCE(s.no_hp, '-') AS no_hp,
			COALESCE(s.latitude, g.latitude) AS latitude,
			COALESCE(s.longitude, g.longitude) AS longitude
		FROM ritase_stop rs
		LEFT JOIN seller s ON s.id_seller = rs.id_seller
		LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
		LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
		WHERE rs.id_ritase = $1
		ORDER BY rs.urutan ASC
	`, idRitase)

	if err != nil {
		log.Fatalf("Gagal query ritase stops: %v", err)
	}
	defer rows.Close()

	fmt.Println("Query stops berhasil!")
}
