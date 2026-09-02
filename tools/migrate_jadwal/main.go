// migrate_jadwal — Jalankan migration 000012 (jadwal_ritase_config tables).
//
// Usage:
//
//	go run ./tools/migrate_jadwal
//
// Baca DATABASE_URL dari .env (supaya konsisten dengan backend).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL tidak ditemukan di environment. Cek file .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Gagal koneksi ke database: %v", err)
	}
	defer conn.Close(ctx)

	// Baca file migration
	sqlBytes, err := os.ReadFile("db/migrations/000012_create_jadwal_ritase_config.up.sql")
	if err != nil {
		log.Fatalf("Gagal baca file migration: %v", err)
	}

	sqlContent := string(sqlBytes)

	// Split per statement (ecara sederhana: split by semicolon, filter kosong)
	statements := splitSQL(sqlContent)

	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println("  Migration: 000012_create_jadwal_ritase_config")
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Printf("Total statements: %d\n\n", len(statements))

	successCount := 0
	errorCount := 0

	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Preview (potong kalau terlalu panjang)
		preview := stmt
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		fmt.Printf("[%d/%d] %s\n", i+1, len(statements), preview)

		_, err := conn.Exec(ctx, stmt)
		if err != nil {
			fmt.Printf("  ❌ Error: %v\n\n", err)
			errorCount++
		} else {
			fmt.Printf("  ✅ OK\n\n")
			successCount++
		}
	}

	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Printf("  Selesai: %d berhasil, %d gagal\n", successCount, errorCount)
	fmt.Println("═══════════════════════════════════════════════════")

	// Verify
	fmt.Println("\nVerifikasi tabel baru:")
	tables := []string{"jadwal_ritase_config", "driver_ritase_jenis", "ritase_route_template", "ritase_stop_template"}
	for _, t := range tables {
		var count int
		err := conn.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&count)
		if err != nil {
			fmt.Printf("  ❌ %s: %v\n", t, err)
		} else {
			fmt.Printf("  ✅ %s: %d row\n", t, count)
		}
	}

	if errorCount > 0 {
		log.Fatalf("\nMigration selesai dengan %d error!", errorCount)
	}
}

// splitSQL memecah SQL content per semicolon, mengabaikan comment dan baris kosong.
func splitSQL(content string) []string {
	var result []string
	var current strings.Builder

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Skip comment
		if strings.HasPrefix(trimmed, "--") {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		// Kalau baris berakhir dengan semicolon, itu akhir statement
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				result = append(result, stmt)
			}
			current.Reset()
		}
	}

	// Sisa terakhir
	if current.Len() > 0 {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" {
			result = append(result, stmt)
		}
	}

	return result
}
