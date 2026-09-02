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
		log.Fatal(err)
	}
	defer db.Close()

	// ── 1. Insert driver "Taras" ──
	var idDriver int64
	err = db.QueryRow(ctx,
		`INSERT INTO driver (nama_driver, status_driver) VALUES ('Taras', 'bertugas') RETURNING id_driver`,
	).Scan(&idDriver)
	if err != nil {
		log.Fatalf("Gagal insert driver: %v", err)
	}
	fmt.Printf("✅ Driver 'Taras' berhasil dibuat (id_driver=%d)\n", idDriver)

	// ── 2. Hash password ──
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Gagal hash password: %v", err)
	}

	// ── 3. Insert user "dir_ops" ──
	var idUser int64
	err = db.QueryRow(ctx,
		`INSERT INTO users (username, password, role, id_driver) VALUES ($1, $2, 'direktur', $3) RETURNING id_user`,
		"dir_ops", string(hash), idDriver,
	).Scan(&idUser)
	if err != nil {
		log.Fatalf("Gagal insert user: %v", err)
	}
	fmt.Printf("✅ User 'dir_ops' berhasil dibuat (id_user=%d, role=direktur, id_driver=%d)\n", idUser, idDriver)
	fmt.Println()
	fmt.Println("Login: dir_ops / password123 (role: direktur)")
}
