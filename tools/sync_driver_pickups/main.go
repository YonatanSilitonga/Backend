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
		log.Fatalf("Gagal konek ke database: %v", err)
	}
	defer db.Close()

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║      SINKRONISASI DRIVER PICKUP KE TABEL DRIVER          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. Ambil semua akun driver_pickup
	rows, err := db.Query(ctx, `
		SELECT u.id_user, u.username, COALESCE(k.nama, u.username) AS nama, 
		       COALESCE(k.no_hp, '081200000000') AS no_hp, u.id_driver
		FROM users u
		LEFT JOIN karyawan k ON k.id_karyawan = u.karyawan_id
		WHERE LOWER(TRIM(u.role)) = 'driver_pickup'
		ORDER BY u.id_user ASC
	`)
	if err != nil {
		log.Fatalf("Gagal query users: %v", err)
	}
	defer rows.Close()

	type Item struct {
		IDUser   int64
		Username string
		Nama     string
		NoHP     string
		IDDriver *int64
	}
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.IDUser, &it.Username, &it.Nama, &it.NoHP, &it.IDDriver); err != nil {
			log.Fatalf("Scan err: %v", err)
		}
		items = append(items, it)
	}
	rows.Close()

	fmt.Printf("Ditemukan %d akun dengan role driver_pickup.\n\n", len(items))

	for _, it := range items {
		var targetDriverID int64

		// Cek apakah supir dengan nama tersebut sudah ada di tabel driver
		err := db.QueryRow(ctx, `
			SELECT id_driver 
			FROM driver 
			WHERE LOWER(TRIM(nama_driver)) = LOWER(TRIM($1)) 
			   OR LOWER(TRIM(nama_driver)) = LOWER(TRIM($2))
			LIMIT 1
		`, it.Username, it.Nama).Scan(&targetDriverID)

		if err != nil {
			// Belum ada -> Insert baru
			err = db.QueryRow(ctx, `
				INSERT INTO driver (nama_driver, no_hp, status_driver)
				VALUES ($1, $2, 'aktif')
				RETURNING id_driver
			`, it.Nama, it.NoHP).Scan(&targetDriverID)
			if err != nil {
				fmt.Printf("❌ Gagal membuat record driver untuk '%s': %v\n", it.Username, err)
				continue
			}
			fmt.Printf("✅ [BARU] Dibuat driver ID: %d untuk '%s' (%s)\n", targetDriverID, it.Username, it.Nama)
		} else {
			fmt.Printf("ℹ️  [ADA]  Driver ID: %d sudah ada untuk '%s'\n", targetDriverID, it.Username)
		}

		// Update users.id_driver jika belum tersambung
		if it.IDDriver == nil || *it.IDDriver != targetDriverID {
			_, err = db.Exec(ctx, `
				UPDATE users 
				SET id_driver = $1 
				WHERE id_user = $2
			`, targetDriverID, it.IDUser)
			if err != nil {
				fmt.Printf("❌ Gagal update users.id_driver untuk user %d: %v\n", it.IDUser, err)
			} else {
				fmt.Printf("🔗 [LINK] Akun User %d ('%s') berhasil ditautkan ke Driver ID %d\n", it.IDUser, it.Username, targetDriverID)
			}
		} else {
			fmt.Printf("👍 [OK]   Akun User %d sudah tertaut dengan Driver ID %d\n", it.IDUser, targetDriverID)
		}
	}

	fmt.Println("\nSelesai! Semua driver pickup kini memiliki id_driver valid di tabel driver.")
}
