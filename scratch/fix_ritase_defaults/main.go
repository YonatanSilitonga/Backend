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

	stmts := []string{
		`UPDATE ritase SET total_koli = 0 WHERE total_koli IS NULL`,
		`UPDATE ritase SET total_awb = 0 WHERE total_awb IS NULL`,
		`ALTER TABLE ritase ALTER COLUMN total_koli SET DEFAULT 0`,
		`ALTER TABLE ritase ALTER COLUMN total_awb SET DEFAULT 0`,
	}
	for _, s := range stmts {
		tag, err := conn.Exec(ctx, s)
		if err != nil {
			log.Fatalf("GAGAL [%s]: %v", s, err)
		}
		fmt.Printf("OK [%s] rows=%d\n", s, tag.RowsAffected())
	}

	var k, a int
	_ = conn.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE total_koli IS NULL), COUNT(*) FILTER (WHERE total_awb IS NULL) FROM ritase`).Scan(&k, &a)
	fmt.Printf("Sisa NULL: total_koli=%d total_awb=%d\n", k, a)
}
