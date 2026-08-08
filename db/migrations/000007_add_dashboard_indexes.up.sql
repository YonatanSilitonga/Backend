-- 000007_add_dashboard_indexes.up.sql
-- Index pendukung dashboard/query cepat. Semua additive (IF NOT EXISTS).
CREATE INDEX IF NOT EXISTS idx_ritase_status        ON ritase(status);
CREATE INDEX IF NOT EXISTS idx_ritase_tanggal       ON ritase(tanggal);
CREATE INDEX IF NOT EXISTS idx_ritase_id_driver     ON ritase(id_driver);
CREATE INDEX IF NOT EXISTS idx_ritase_id_kendaraan  ON ritase(id_kendaraan);
CREATE INDEX IF NOT EXISTS idx_ritase_event_status  ON ritase_event(status);
CREATE INDEX IF NOT EXISTS idx_seller_lat_lng       ON seller(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_gudang_tipe          ON gudang(tipe);