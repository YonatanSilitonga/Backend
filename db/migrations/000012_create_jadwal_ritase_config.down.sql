-- 000012_create_jadwal_ritase_config.down.sql
-- Hapus semua tabel yang dibuat di migration ini.

DROP TABLE IF EXISTS ritase_stop_template;
DROP TABLE IF EXISTS ritase_route_template;
DROP TABLE IF EXISTS driver_ritase_jenis;
DROP TABLE IF EXISTS jadwal_ritase_config;
