package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect membuka koneksi pool PostgreSQL (Supabase) dan memverifikasi koneksinya.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL kosong, cek file .env")
	}

	// Pakai port 6543 (transaction pooler) untuk Supabase pooler jika masih 5432
	databaseURL = strings.Replace(databaseURL, "pooler.supabase.com:5432", "pooler.supabase.com:6543", 1)

	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("gagal parse DATABASE_URL: %w", err)
	}

	// Wajib SimpleProtocol mode untuk PgBouncer transaction pooler (mencegah error SQLSTATE 42P05: prepared statement already exists)
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// setelan dasar pool
	poolCfg.MaxConns = 3
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = 5 * time.Minute
	poolCfg.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("gagal buat pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var pingErr error
	// Retry ping up to 3 times for flaky connections
	for i := 0; i < 3; i++ {
		pingErr = pool.Ping(pingCtx)
		if pingErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if pingErr != nil {
		pool.Close()
		return nil, fmt.Errorf("gagal konek ke database: %w", pingErr)
	}

	return pool, nil
}
