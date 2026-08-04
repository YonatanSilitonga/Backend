package main

import (
	"context"
	"fmt"
	"log"

	"backend/internal/config"
	"backend/internal/database"
)

type SellerData struct {
	Kode      string
	Nama      string
	Alamat    string
	Kota      string
	PIC       string
	NoHP      string
	Status    string
	Latitude  float64
	Longitude float64
}

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Koneksi database gagal: %v", err)
	}
	defer db.Close()

	// Pastikan kolom latitude & longitude ada di tabel seller
	_, err = db.Exec(ctx, `
		ALTER TABLE seller ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
		ALTER TABLE seller ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;
	`)
	if err != nil {
		log.Printf("Peringatan saat penambahan kolom: %v", err)
	}

	// Data 7 Seller Lengkap dari Excel
	sellers := []SellerData{
		{
			Kode:      "SLR-001",
			Nama:      "TITIP AJA",
			Alamat:    "RMM9+49Q, RT.002/RW.003, Poris Plawad, Kec. Batuceper, Kota Tangerang, Banten 15141",
			Kota:      "Tangerang",
			PIC:       "Jarot",
			NoHP:      "+62 899-2279-170",
			Status:    "aktif",
			Latitude:  -6.152972,
			Longitude: 106.603056,
		},
		{
			Kode:      "SLR-002",
			Nama:      "SOMETHING",
			Alamat:    "QJQG+5H7 Cikokol, Kota Tangerang, Banten",
			Kota:      "Tangerang",
			PIC:       "Deni",
			NoHP:      "+62 895-3281-77533",
			Status:    "aktif",
			Latitude:  -6.102222,
			Longitude: 106.685694,
		},
		{
			Kode:      "SLR-003",
			Nama:      "SKI",
			Alamat:    "Jl. Pajajaran XIV No.62, RT.005/RW.005, Gandasari, Kec. Jatiuwung, Kota Tangerang, Banten 15810",
			Kota:      "Tangerang",
			PIC:       "Mun",
			NoHP:      "+62 856-0834-9714",
			Status:    "aktif",
			Latitude:  -6.231611,
			Longitude: 106.720278,
		},
		{
			Kode:      "SLR-004",
			Nama:      "CILUPBA",
			Alamat:    "VMXP+477 Benda, Kota Tangerang, Banten",
			Kota:      "Tangerang",
			PIC:       "Eko",
			NoHP:      "+62 851-7348-9193",
			Status:    "aktif",
			Latitude:  -6.214750,
			Longitude: 106.680556,
		},
		{
			Kode:      "SLR-005",
			Nama:      "PAYUTRUS KACAMATA",
			Alamat:    "QMPJ+463 Pinang, Kota Tangerang, Banten",
			Kota:      "Tangerang",
			PIC:       "gopur",
			NoHP:      "+62 895-3953-20446",
			Status:    "aktif",
			Latitude:  -6.220694,
			Longitude: 106.585472,
		},
		{
			Kode:      "SLR-006",
			Nama:      "BAYUR",
			Alamat:    "RJW3+R63 Periuk Jaya, Kota Tangerang, Banten",
			Kota:      "Tangerang",
			PIC:       "siregar",
			NoHP:      "+62 853-8119-6599",
			Status:    "aktif",
			Latitude:  -6.212083,
			Longitude: 106.626444,
		},
		{
			Kode:      "SLR-007",
			Nama:      "LARANGAN CIPADU",
			Alamat:    "QP9C+943 Sudimara Tim., Kota Tangerang, Banten",
			Kota:      "Tangerang",
			PIC:       "Junaedi",
			NoHP:      "+62 858-9471-8860",
			Status:    "aktif",
			Latitude:  -6.167750,
			Longitude: 106.668778,
		},
	}

	// Hapus data lama di tabel seller untuk digantikan data baru yang presisi
	_, err = db.Exec(ctx, "DELETE FROM seller")
	if err != nil {
		log.Fatalf("Gagal mengosongkan tabel seller: %v", err)
	}

	// Insert ke-7 data seller baru
	for _, s := range sellers {
		_, err := db.Exec(ctx, `
			INSERT INTO seller (kode_seller, nama_seller, alamat, kota, pic, no_hp, status, latitude, longitude)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, s.Kode, s.Nama, s.Alamat, s.Kota, s.PIC, s.NoHP, s.Status, s.Latitude, s.Longitude)

		if err != nil {
			log.Fatalf("Gagal insert seller %s: %v", s.Nama, err)
		}
		fmt.Printf("✓ Berhasil menyimpan seller: %s (%f, %f)\n", s.Nama, s.Latitude, s.Longitude)
	}

	fmt.Println("\n🎉 Selesai! Semua 7 data seller lengkap dengan Alamat, PIC, No HP, Latitude & Longitude telah tersimpan ke Supabase!")
}
