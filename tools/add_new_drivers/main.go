package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type NewDriver struct {
	Username string
	Nama     string
	NoHP     string
}

func main() {
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL tidak ditemukan di .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal konek database: %v", err)
	}
	defer conn.Close(ctx)

	newDrivers := []NewDriver{
		{Username: "mamet", Nama: "Mamet", NoHP: "081200000001"},
		{Username: "kenken", Nama: "Kenken", NoHP: "081200000002"},
		{Username: "lalang", Nama: "Lalang", NoHP: "081200000003"},
		{Username: "catur", Nama: "Catur", NoHP: "081200000004"},
		{Username: "april", Nama: "April", NoHP: "081200000005"},
	}

	passwordPlain := "driver123"
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Gagal generate bcrypt hash: %v", err)
	}
	passwordHash := string(hashBytes)

	// Realign sequence users jika perlu
	_, _ = conn.Exec(ctx, `SELECT setval('users_id_user_seq', GREATEST((SELECT COALESCE(MAX(id_user),1) FROM users), 1))`)

	fmt.Println("=== PROSES PENAMBAHAN DRIVER & AKUN BARU ===")
	for _, d := range newDrivers {
		// 1. Cek / Insert ke tabel driver
		var idDriver int64
		err := conn.QueryRow(ctx, "SELECT id_driver FROM driver WHERE LOWER(nama_driver) = LOWER($1) LIMIT 1", d.Nama).Scan(&idDriver)
		if errors.Is(err, pgx.ErrNoRows) {
			err = conn.QueryRow(ctx, `
				INSERT INTO driver (nama_driver, no_hp, status_driver, jenis_driver)
				VALUES ($1, $2, 'bertugas', 'tetap')
				RETURNING id_driver
			`, d.Nama, d.NoHP).Scan(&idDriver)
			if err != nil {
				log.Printf("❌ Gagal insert driver %s: %v", d.Nama, err)
				continue
			}
			fmt.Printf("✓ Profil Driver dibuat: %s (id_driver: %d)\n", d.Nama, idDriver)
		} else if err != nil {
			log.Printf("❌ Gagal cek driver %s: %v", d.Nama, err)
			continue
		} else {
			fmt.Printf("ℹ Profil Driver sudah ada: %s (id_driver: %d)\n", d.Nama, idDriver)
		}

		// 2. Cek / Insert / Update ke tabel users
		var idUser int64
		usernameLower := strings.ToLower(d.Username)
		err = conn.QueryRow(ctx, `
			INSERT INTO users (username, password, role, id_driver, status)
			VALUES ($1, $2, 'driver', $3, 'aktif')
			ON CONFLICT (username) DO UPDATE 
			SET password = EXCLUDED.password,
			    id_driver = EXCLUDED.id_driver,
			    status = 'aktif'
			RETURNING id_user
		`, usernameLower, passwordHash, idDriver).Scan(&idUser)

		if err != nil {
			log.Printf("❌ Gagal insert/update user %s: %v", usernameLower, err)
		} else {
			fmt.Printf("✅ Akun User Login Berhasil: username=%s | password=%s | id_user=%d | id_driver=%d\n",
				usernameLower, passwordPlain, idUser, idDriver)
		}
	}

	fmt.Println("\n🎉 Selesai! Semua driver baru siap login di aplikasi Mobile.")
}
