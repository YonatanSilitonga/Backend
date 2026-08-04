package main

import (
	"context"
	"log"

	"backend/internal/config"
	"backend/internal/database"
)

var migrationSQL = `
CREATE TABLE IF NOT EXISTS ritase_event (
  id_event   BIGSERIAL PRIMARY KEY,
  id_ritase  INTEGER NOT NULL REFERENCES ritase(id_ritase) ON DELETE CASCADE,
  status     VARCHAR NOT NULL,
  catatan    TEXT,
  latitude   NUMERIC,
  longitude  NUMERIC,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ritase_event_ritase ON ritase_event(id_ritase);
CREATE INDEX IF NOT EXISTS idx_ritase_event_created ON ritase_event(id_ritase, created_at);
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS paket_tertinggal INTEGER DEFAULT 0;
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS alasan_tertinggal TEXT;
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
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
	log.Println("MIGRATION OK ✓")
}
