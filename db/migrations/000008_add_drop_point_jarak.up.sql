-- 000008_add_drop_point_jarak.up.sql
-- Tambah kolom jarak tempuh GATEWAY (drop point) -> gudang (Outgoing & DC).
-- Backfill via tools/fill_jarak. Additive.
ALTER TABLE drop_point ADD COLUMN IF NOT EXISTS jarak_tempuh_km DOUBLE PRECISION;
ALTER TABLE drop_point ADD COLUMN IF NOT EXISTS jarak_dc_km DOUBLE PRECISION;