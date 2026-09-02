-- Audit fields untuk 6 tabel admin
-- created_at, created_by, updated_at, updated_by

-- driver
ALTER TABLE driver ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE driver ADD COLUMN created_by INTEGER;
ALTER TABLE driver ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE driver ADD COLUMN updated_by INTEGER;

-- kendaraan
ALTER TABLE kendaraan ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE kendaraan ADD COLUMN created_by INTEGER;
ALTER TABLE kendaraan ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE kendaraan ADD COLUMN updated_by INTEGER;

-- seller
ALTER TABLE seller ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE seller ADD COLUMN created_by INTEGER;
ALTER TABLE seller ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE seller ADD COLUMN updated_by INTEGER;

-- gudang
ALTER TABLE gudang ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE gudang ADD COLUMN created_by INTEGER;
ALTER TABLE gudang ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE gudang ADD COLUMN updated_by INTEGER;

-- drop_point
ALTER TABLE drop_point ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE drop_point ADD COLUMN created_by INTEGER;
ALTER TABLE drop_point ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE drop_point ADD COLUMN updated_by INTEGER;

-- users
ALTER TABLE users ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
ALTER TABLE users ADD COLUMN created_by INTEGER;
ALTER TABLE users ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE users ADD COLUMN updated_by INTEGER;
