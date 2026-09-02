-- Ritase (created_at sudah ada)
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS created_by INTEGER;
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS updated_by INTEGER;

-- jadwal_ritase_config
ALTER TABLE jadwal_ritase_config ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE jadwal_ritase_config ADD COLUMN IF NOT EXISTS created_by INTEGER;
ALTER TABLE jadwal_ritase_config ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
ALTER TABLE jadwal_ritase_config ADD COLUMN IF NOT EXISTS updated_by INTEGER;

-- driver_ritase_jenis
ALTER TABLE driver_ritase_jenis ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE driver_ritase_jenis ADD COLUMN IF NOT EXISTS created_by INTEGER;
ALTER TABLE driver_ritase_jenis ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
ALTER TABLE driver_ritase_jenis ADD COLUMN IF NOT EXISTS updated_by INTEGER;

-- ritase_route_template
ALTER TABLE ritase_route_template ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE ritase_route_template ADD COLUMN IF NOT EXISTS created_by INTEGER;
ALTER TABLE ritase_route_template ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
ALTER TABLE ritase_route_template ADD COLUMN IF NOT EXISTS updated_by INTEGER;

-- ritase_stop_template
ALTER TABLE ritase_stop_template ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE ritase_stop_template ADD COLUMN IF NOT EXISTS created_by INTEGER;
ALTER TABLE ritase_stop_template ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;
ALTER TABLE ritase_stop_template ADD COLUMN IF NOT EXISTS updated_by INTEGER;
