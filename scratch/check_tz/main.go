package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// fallback: baca .env manual
		b, err := os.ReadFile(".env")
		if err != nil {
			log.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "DATABASE_URL=") {
				dbURL = strings.TrimPrefix(strings.TrimSpace(line), "DATABASE_URL=")
			}
		}
	}
	dbURL = strings.Replace(dbURL, "pooler.supabase.com:5432", "pooler.supabase.com:6543", 1)
	if i := strings.Index(dbURL, "?"); i >= 0 {
		dbURL = dbURL[:i] + "?sslmode=require"
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	var tz, now string
	_ = conn.QueryRow(ctx, "SELECT current_setting('TimeZone'), now()::text").Scan(&tz, &now)
	fmt.Printf("TimeZone DB : %s\nnow() DB    : %s\n\n", tz, now)

	rows, err := conn.Query(ctx, `
		SELECT id, id_seller, jenis_ritase, ritase_ke, catatan,
		       created_at::text,
		       (created_at + interval '7 hours')::date::text AS tanggal_plus7,
		       created_at::date::text AS tanggal_utc,
		       COALESCE(id_ritase, 0)
		FROM input_kapten ORDER BY id DESC LIMIT 5`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	fmt.Println("id | seller | jenis | rit | catatan | created_at | +7h date | raw date | id_ritase")
	for rows.Next() {
		var id, seller, rit, idRit int
		var jenis, cat string
		var createdAt, plus7, raw string
		var catatan *string
		if err := rows.Scan(&id, &seller, &jenis, &rit, &catatan, &createdAt, &plus7, &raw, &idRit); err != nil {
			log.Fatal(err)
		}
		if catatan != nil {
			cat = *catatan
		}
		fmt.Printf("%d | %d | %s | %d | %s | %s | %s | %s | %d\n", id, seller, jenis, rit, cat, createdAt, plus7, raw, idRit)
	}
}
