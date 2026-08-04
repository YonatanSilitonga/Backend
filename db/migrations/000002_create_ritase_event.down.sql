-- 000002_create_ritase_event.down.sql
DROP TABLE IF EXISTS ritase_event;
ALTER TABLE ritase DROP COLUMN IF EXISTS paket_tertinggal;
ALTER TABLE ritase DROP COLUMN IF EXISTS alasan_tertinggal;
ALTER TABLE ritase DROP COLUMN IF EXISTS created_at;
