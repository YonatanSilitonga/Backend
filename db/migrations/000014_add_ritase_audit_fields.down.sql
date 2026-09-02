-- Ritase
ALTER TABLE ritase DROP COLUMN IF EXISTS updated_at;
ALTER TABLE ritase DROP COLUMN IF EXISTS created_by;
ALTER TABLE ritase DROP COLUMN IF EXISTS updated_by;

-- jadwal_ritase_config
ALTER TABLE jadwal_ritase_config DROP COLUMN IF EXISTS created_at;
ALTER TABLE jadwal_ritase_config DROP COLUMN IF EXISTS created_by;
ALTER TABLE jadwal_ritase_config DROP COLUMN IF EXISTS updated_at;
ALTER TABLE jadwal_ritase_config DROP COLUMN IF EXISTS updated_by;

-- driver_ritase_jenis
ALTER TABLE driver_ritase_jenis DROP COLUMN IF EXISTS created_at;
ALTER TABLE driver_ritase_jenis DROP COLUMN IF EXISTS created_by;
ALTER TABLE driver_ritase_jenis DROP COLUMN IF EXISTS updated_at;
ALTER TABLE driver_ritase_jenis DROP COLUMN IF EXISTS updated_by;

-- ritase_route_template
ALTER TABLE ritase_route_template DROP COLUMN IF EXISTS created_at;
ALTER TABLE ritase_route_template DROP COLUMN IF EXISTS created_by;
ALTER TABLE ritase_route_template DROP COLUMN IF EXISTS updated_at;
ALTER TABLE ritase_route_template DROP COLUMN IF EXISTS updated_by;

-- ritase_stop_template
ALTER TABLE ritase_stop_template DROP COLUMN IF EXISTS created_at;
ALTER TABLE ritase_stop_template DROP COLUMN IF EXISTS created_by;
ALTER TABLE ritase_stop_template DROP COLUMN IF EXISTS updated_at;
ALTER TABLE ritase_stop_template DROP COLUMN IF EXISTS updated_by;
