package main

import (
	"context"
	"fmt"
	"log"

	"backend/internal/config"
	"backend/internal/database"
)

type DriverData struct {
	Nama     string
	NoHP     string
	NoSIM    string
	JenisSIM string
	Status   string
}

func main() {
	ctx := context.Background()
	cfg := config.Load()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Koneksi database gagal: %v", err)
	}
	defer db.Close()

	drivers := []DriverData{
		{Nama: "Budi Santoso", NoHP: "0812-3456-7890", NoSIM: "810112345678", JenisSIM: "B1 Umum", Status: "bertugas"},
		{Nama: "Agus Wijaya", NoHP: "0813-9876-5432", NoSIM: "810298765432", JenisSIM: "B1 Umum", Status: "bertugas"},
		{Nama: "Slamet Riyadi", NoHP: "0857-3333-4444", NoSIM: "810498765433", JenisSIM: "B1 Umum", Status: "bertugas"},
		{Nama: "Dedi Kurniawan", NoHP: "0811-5555-6666", NoSIM: "810512345680", JenisSIM: "B1 Umum", Status: "libur"},
		{Nama: "Hendra Gunawan", NoHP: "0822-7777-8888", NoSIM: "810698765434", JenisSIM: "B2", Status: "libur"},
		{Nama: "Rudi Hartono", NoHP: "0821-1111-2222", NoSIM: "810312345679", JenisSIM: "B1 Umum", Status: "bertugas"},
	}

	for _, d := range drivers {
		_, err := db.Exec(ctx, `
			INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, d.Nama, d.NoHP, d.NoSIM, d.JenisSIM, d.Status)

		if err != nil {
			log.Printf("Gagal insert driver %s: %v", d.Nama, err)
		} else {
			fmt.Printf("✓ Driver dikembalikan: %s (%s)\n", d.Nama, d.Status)
		}
	}

	fmt.Println("\n🎉 Selesai! Semua data driver bawaan telah dikembalikan secara utuh!")
}
