-- 000002_create_ritase_event.up.sql
-- Tabel timeline status perjalanan (10 status tombol driver) + kolom muatan ritase.

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

-- kolom muatan tambahan di ritase
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS paket_tertinggal INTEGER DEFAULT 0;
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS alasan_tertinggal TEXT;

-- audit timestamp dibuatnya ritase
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
