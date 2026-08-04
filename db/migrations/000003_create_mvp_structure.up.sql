-- 000003_create_mvp_structure.up.sql
-- Struktur MVP: rute ritase (ritase_stop), jadwal RIT, link user->driver, jenis driver.
-- Semua additive (IF NOT EXISTS) — tidak menghapus/mengubah data existing.

-- 1. Tabel rute per ritase (banyak titik: gudang -> seller(s) -> drop_point/GTW)
CREATE TABLE IF NOT EXISTS ritase_stop (
  id_stop       BIGSERIAL PRIMARY KEY,
  id_ritase     INTEGER NOT NULL REFERENCES ritase(id_ritase) ON DELETE CASCADE,
  urutan        INTEGER NOT NULL DEFAULT 1,
  jenis_stop    VARCHAR NOT NULL,          -- gudang | seller | drop_point
  id_seller     INTEGER,                   -- terisi jika jenis_stop = seller
  id_drop_point INTEGER,                   -- terisi jika jenis_stop = drop_point / gudang
  keterangan    TEXT
);

CREATE INDEX IF NOT EXISTS idx_ritase_stop_ritase ON ritase_stop(id_ritase, urutan);

-- 2. Jadwal RIT di tabel ritase (nullable, diisi belakangan)
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS jam_mulai TIME;
ALTER TABLE ritase ADD COLUMN IF NOT EXISTS jam_selesai TIME;

-- 3. Link user -> driver (untuk scoping: driver lihat ritase miliknya)
ALTER TABLE users ADD COLUMN IF NOT EXISTS id_driver INTEGER;

-- 4. Jenis driver (tetap / kondisional)
ALTER TABLE driver ADD COLUMN IF NOT EXISTS jenis_driver VARCHAR DEFAULT 'tetap';

-- 5. Rapikan kolom ad-hoc yang sebelumnya ditambah lewat cmd/ (idempotent)
ALTER TABLE armada_tracking ADD COLUMN IF NOT EXISTS jumlah_koli INTEGER DEFAULT 0;
ALTER TABLE seller ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE seller ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;
