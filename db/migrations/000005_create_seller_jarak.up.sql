-- 000005_create_seller_jarak.up.sql
-- Tambah kolom JARAK TEMPUH DC -> seller (additive, pending backfill via tools/fill_jarak).
-- Hanya menambah kolom — TIDAK mengubah data existing.
ALTER TABLE seller ADD COLUMN IF NOT EXISTS jarak_tempuh_km DOUBLE PRECISION;