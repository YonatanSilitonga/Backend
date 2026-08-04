package main

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"

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

	// 1. Insert ke tabel driver (jika belum ada)
	var idDriver int64
	err = db.QueryRow(ctx, `
		INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver)
		VALUES ('AWALUDIN', '0812-9999-8888', '810199998888', 'B1 Umum', 'bertugas')
		ON CONFLICT DO NOTHING
		RETURNING id_driver
	`).Scan(&idDriver)
	if err != nil {
		// Ambil ID jika sudah ada
		_ = db.QueryRow(ctx, "SELECT id_driver FROM driver WHERE nama_driver = 'AWALUDIN' LIMIT 1").Scan(&idDriver)
	}
	fmt.Printf("✓ Driver AWALUDIN diproses (ID Driver: %d)\n", idDriver)

	// 2. Hash password 'password' menggunakan bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Gagal hash password: %v", err)
	}

	// 3. Insert / Update ke tabel users
	_, err = db.Exec(ctx, `
		INSERT INTO users (username, password, role)
		VALUES ('AWALUDIN', $1, 'driver')
		ON CONFLICT (username) DO UPDATE SET password = $1, role = 'driver'
	`, string(hashedPassword))

	if err != nil {
		log.Fatalf("Gagal insert user AWALUDIN: %v", err)
	}

	fmt.Println("🎉 Selesai! User AWALUDIN (Password: password) telah berhasil tersimpan di tabel users & driver Supabase!")
}
