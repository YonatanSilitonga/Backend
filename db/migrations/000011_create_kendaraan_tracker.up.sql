-- 000011_create_kendaraan_tracker.up.sql
-- Mapping perangkat GPS tracker (IMEI) -> kendaraan.
-- Sumber posisi cadangan saat HP driver mati: tracker tetap kirim lokasi,
-- dashboard tetap LIVE selama tracker hidup.
CREATE TABLE IF NOT EXISTS kendaraan_tracker (
  id_tracker    BIGSERIAL PRIMARY KEY,
  imei          VARCHAR(64) NOT NULL UNIQUE,
  id_kendaraan  INTEGER NOT NULL REFERENCES kendaraan(id_kendaraan) ON DELETE CASCADE,
  nama_device   VARCHAR(100),
  aktif         BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kendaraan_tracker_imei ON kendaraan_tracker(imei);
CREATE INDEX IF NOT EXISTS idx_kendaraan_tracker_kendaraan ON kendaraan_tracker(id_kendaraan);
