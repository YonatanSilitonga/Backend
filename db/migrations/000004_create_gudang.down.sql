-- 000004_create_gudang.down.sql
ALTER TABLE ritase_stop DROP COLUMN IF EXISTS id_gudang;
DROP TABLE IF EXISTS gudang;