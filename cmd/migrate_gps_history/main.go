package main

import (
	"context"
	"log"

	"backend/internal/config"
	"backend/internal/database"
)

var migrationSQL = `
CREATE TABLE IF NOT EXISTS armada_gps_history (
  id            BIGSERIAL PRIMARY KEY,
  id_ritase     INTEGER REFERENCES ritase(id_ritase) ON DELETE SET NULL,
  id_kendaraan  INTEGER NOT NULL REFERENCES kendaraan(id_kendaraan) ON DELETE CASCADE,
  id_driver     INTEGER NOT NULL REFERENCES driver(id_driver) ON DELETE CASCADE,
  latitude      DOUBLE PRECISION NOT NULL,
  longitude     DOUBLE PRECISION NOT NULL,
  kecepatan     INTEGER DEFAULT 0,
  created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_gps_history_ritase ON armada_gps_history(id_ritase, created_at);
CREATE INDEX IF NOT EXISTS idx_gps_history_kendaraan ON armada_gps_history(id_kendaraan, created_at);
`

func main() {
	ctx := context.Background()
	cfg := config.Load()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(ctx, migrationSQL); err != nil {
		log.Fatalf("migrasi gagal: %v", err)
	}
	log.Println("MIGRATION 000015 OK ✓")
}
