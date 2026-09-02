// Migration: tambah kolom status di tabel users.
// ALTER TABLE ... ADD COLUMN IF NOT EXISTS — aman dijalankan berulang kali.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL tidak diset")
		os.Exit(1)
	}

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	// Cek apakah kolom sudah ada
	var exists bool
	err = conn.QueryRow(context.Background(),
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'status'
		)`).Scan(&exists)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cek kolom error: %v\n", err)
		os.Exit(1)
	}

	if exists {
		fmt.Println("Kolom 'status' sudah ada di tabel users. Tidak perlu migration.")
		return
	}

	// Tambah kolom status (default 'aktif')
	_, err = conn.Exec(context.Background(),
		`ALTER TABLE users ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'aktif'`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ALTER TABLE error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Kolom 'status' berhasil ditambahkan ke tabel users (default: 'aktif')")
}
