-- 000006_create_seller_jarak_dc.up.sql
-- Tambah kolom JARAK DC -> seller (additive). Backfill via tools/fill_jarak.
-- Kolom ini = jarak tempuh dari GUDANG DC (Buaran Indah); jarak_tempuh_km = dari GUDANG OUTGOING.
ALTER TABLE seller ADD COLUMN IF NOT EXISTS jarak_dc_km DOUBLE PRECISION;