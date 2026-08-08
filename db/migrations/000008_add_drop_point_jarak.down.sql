-- 000008_add_drop_point_jarak.down.sql
ALTER TABLE drop_point DROP COLUMN IF EXISTS jarak_tempuh_km;
ALTER TABLE drop_point DROP COLUMN IF EXISTS jarak_dc_km;