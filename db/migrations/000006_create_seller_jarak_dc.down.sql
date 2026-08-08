-- 000006_create_seller_jarak_dc.down.sql
-- Rollback: hapus kolom jarak dari DC (data lain tidak tersentuh).
ALTER TABLE seller DROP COLUMN IF EXISTS jarak_dc_km;