-- 000007_add_dashboard_indexes.down.sql
DROP INDEX IF EXISTS idx_ritase_status;
DROP INDEX IF EXISTS idx_ritase_tanggal;
DROP INDEX IF EXISTS idx_ritase_id_driver;
DROP INDEX IF EXISTS idx_ritase_id_kendaraan;
DROP INDEX IF EXISTS idx_ritase_event_status;
DROP INDEX IF EXISTS idx_seller_lat_lng;
DROP INDEX IF EXISTS idx_gudang_tipe;