-- 000004_create_gudang.up.sql
-- Tabel gudang (entitas berbeda dengan seller/drop_point) + relasi di ritase_stop.
-- Semua additive (IF NOT EXISTS) — tidak menghapus/mengubah data existing.

CREATE TABLE IF NOT EXISTS gudang (
  id_gudang   BIGSERIAL PRIMARY KEY,
  nama_gudang VARCHAR NOT NULL,
  tipe        VARCHAR NOT NULL DEFAULT 'outgoing', -- outgoing | incoming
  alamat      TEXT,
  latitude    DOUBLE PRECISION,
  longitude   DOUBLE PRECISION
);

ALTER TABLE ritase_stop ADD COLUMN IF NOT EXISTS id_gudang INTEGER;