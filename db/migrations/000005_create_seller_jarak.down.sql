-- 000005_create_seller_jarak.down.sql
-- Hapus kolom jarak tempuh (rollback). Nilai DB lain tidak tersentuh.
ALTER TABLE seller DROP COLUMN IF EXISTS jarak_tempuh_km;