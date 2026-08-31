-- 000012_create_jadwal_ritase_config.up.sql
-- Konfigurasi jadwal ritase dinamis (ganti hardcoded di Go code).
-- Berisi jam mulai/selesai per jenis (outgoing/incoming) + ritase_ke.

-- Tabel 1: Jam Ritase Config
CREATE TABLE IF NOT EXISTS jadwal_ritase_config (
  id          SERIAL PRIMARY KEY,
  jenis       VARCHAR(20) NOT NULL,   -- 'outgoing' | 'incoming'
  ritase_ke   INTEGER NOT NULL,
  jam_mulai   TIME NOT NULL,
  jam_selesai TIME NOT NULL,
  UNIQUE(jenis, ritase_ke)
);

-- Tabel 2: Driver → Jenis Ritase (mapping dinamis)
-- Default: kalau driver + ritase_ke tidak ada di tabel ini → outgoing
CREATE TABLE IF NOT EXISTS driver_ritase_jenis (
  id          SERIAL PRIMARY KEY,
  id_driver   INTEGER NOT NULL REFERENCES driver(id_driver),
  ritase_ke   INTEGER NOT NULL,
  jenis       VARCHAR(20) NOT NULL,   -- 'outgoing' | 'incoming'
  UNIQUE(id_driver, ritase_ke)
);

-- Tabel 3: Route Template (ganti defaultFixedRoutes)
CREATE TABLE IF NOT EXISTS ritase_route_template (
  id              SERIAL PRIMARY KEY,
  id_driver       INTEGER NOT NULL REFERENCES driver(id_driver),
  id_kendaraan    INTEGER NOT NULL REFERENCES kendaraan(id_kendaraan),
  id_drop_point   INTEGER NOT NULL REFERENCES drop_point(id_drop_point),
  ritase_ke       INTEGER NOT NULL,
  jenis_ritase    VARCHAR(20),
  aktif           BOOLEAN DEFAULT TRUE,
  urutan_template INTEGER DEFAULT 0
);

-- Tabel 4: Stop Template (detail stops per route)
CREATE TABLE IF NOT EXISTS ritase_stop_template (
  id                SERIAL PRIMARY KEY,
  id_route_template INTEGER NOT NULL REFERENCES ritase_route_template(id) ON DELETE CASCADE,
  urutan            INTEGER NOT NULL,
  jenis_stop        VARCHAR(20) NOT NULL,
  id_lokasi         INTEGER NOT NULL,
  kolom_lokasi      VARCHAR(30) NOT NULL,
  keterangan        TEXT
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_jadwal_ritase_config_jenis ON jadwal_ritase_config(jenis);
CREATE INDEX IF NOT EXISTS idx_driver_ritase_jenis_driver ON driver_ritase_jenis(id_driver);
CREATE INDEX IF NOT EXISTS idx_ritase_route_template_driver ON ritase_route_template(id_driver);
CREATE INDEX IF NOT EXISTS idx_ritase_stop_template_route ON ritase_stop_template(id_route_template);

-- ══════════════════════════════════════════════════════════════
-- SEED DATA (dari hardcoded di admin_ritase_handlers.go)
-- ══════════════════════════════════════════════════════════════

-- Seed 1: Jam Ritase Config
INSERT INTO jadwal_ritase_config (jenis, ritase_ke, jam_mulai, jam_selesai) VALUES
  ('outgoing', 1, '16:00:00', '20:00:00'),
  ('outgoing', 2, '20:01:00', '00:00:00'),
  ('outgoing', 3, '00:01:00', '03:00:00'),
  ('incoming', 1, '01:00:00', '04:30:00'),
  ('incoming', 2, '07:00:00', '10:30:00'),
  ('incoming', 3, '13:00:00', '16:30:00'),
  ('incoming', 4, '19:00:00', '22:30:00')
ON CONFLICT (jenis, ritase_ke) DO NOTHING;

-- Seed 2: Driver → Jenis Ritase (D10 & D11 incoming)
INSERT INTO driver_ritase_jenis (id_driver, ritase_ke, jenis) VALUES
  (10, 1, 'incoming'),
  (10, 4, 'incoming'),
  (11, 2, 'incoming'),
  (11, 3, 'incoming')
ON CONFLICT (id_driver, ritase_ke) DO NOTHING;

-- Seed 3: Route Template (11 rute dari defaultFixedRoutes)
-- ID auto-increment: 1-11 sesuai urutan INSERT
-- D3 outgoing R1
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (3, 2, 2, 1, 'outgoing', TRUE, 1);
-- D3 outgoing R2
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (3, 2, 2, 2, 'outgoing', TRUE, 2);
-- D2 outgoing R1
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (2, 6, 2, 1, 'outgoing', TRUE, 3);
-- D2 outgoing R2
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (2, 6, 2, 2, 'outgoing', TRUE, 4);
-- D1 outgoing R2
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (1, 11, 2, 2, 'outgoing', TRUE, 5);
-- D4 outgoing R2
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (4, 15, 2, 2, 'outgoing', TRUE, 6);
-- D15 outgoing R3
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (15, 3, 2, 3, 'outgoing', TRUE, 7);
-- D11 incoming R2
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (11, 9, 3, 2, 'incoming', TRUE, 8);
-- D11 incoming R3
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (11, 9, 3, 3, 'incoming', TRUE, 9);
-- D10 incoming R1
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (10, 9, 3, 1, 'incoming', TRUE, 10);
-- D10 incoming R4
INSERT INTO ritase_route_template (id_driver, id_kendaraan, id_drop_point, ritase_ke, jenis_ritase, aktif, urutan_template)
VALUES (10, 9, 3, 4, 'incoming', TRUE, 11);

-- Seed 4: Stop Template (detail stops per route)
-- Route 1: D3 outgoing R1 (Gudang1 → Seller3 → Seller1 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (1, 1, 'gudang', 1, 'id_gudang', 'Mulai dari gudang origin'),
  (1, 2, 'seller', 3, 'id_seller', 'Ambil paket di Seller 3'),
  (1, 3, 'seller', 1, 'id_seller', 'Ambil paket di Seller 1'),
  (1, 4, 'gateway', 2, 'id_drop_point', 'Tujuan akhir Gateway 2');

-- Route 2: D3 outgoing R2 (Gudang1 → Seller3 → Gudang1 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (2, 1, 'gudang', 1, 'id_gudang', 'Gudang 1'),
  (2, 2, 'seller', 3, 'id_seller', 'Seller 3'),
  (2, 3, 'gudang', 1, 'id_gudang', 'Gudang 1'),
  (2, 4, 'gateway', 2, 'id_drop_point', 'Gateway 2');

-- Route 3: D2 outgoing R1 (Gudang1 → Seller2 → Gudang2 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (3, 1, 'gudang', 1, 'id_gudang', 'Gudang 1'),
  (3, 2, 'seller', 2, 'id_seller', 'Seller 2'),
  (3, 3, 'gudang', 2, 'id_gudang', 'Gudang 2'),
  (3, 4, 'gateway', 2, 'id_drop_point', 'Gateway 2');

-- Route 4: D2 outgoing R2 (GW2 → Seller2 → Gudang2 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (4, 1, 'gateway', 2, 'id_drop_point', 'Gateway 2'),
  (4, 2, 'seller', 2, 'id_seller', 'Seller 2'),
  (4, 3, 'gudang', 2, 'id_gudang', 'Gudang 2'),
  (4, 4, 'gateway', 2, 'id_drop_point', 'Gateway 2');

-- Route 5: D1 outgoing R2 (GW2 → Seller4 → Seller1 → Gudang1 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (5, 1, 'gateway', 2, 'id_drop_point', 'Gateway 2'),
  (5, 2, 'seller', 4, 'id_seller', 'Seller 4'),
  (5, 3, 'seller', 1, 'id_seller', 'Seller 1'),
  (5, 4, 'gudang', 1, 'id_gudang', 'Gudang 1'),
  (5, 5, 'gateway', 2, 'id_drop_point', 'Gateway 2');

-- Route 6: D4 outgoing R2 (Gudang1 → Seller7 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (6, 1, 'gudang', 1, 'id_gudang', 'Gudang 1'),
  (6, 2, 'seller', 7, 'id_seller', 'PGA2 Seller 7'),
  (6, 3, 'gateway', 2, 'id_drop_point', 'Gateway 2');

-- Route 7: D15 outgoing R3 (Gudang1 → GW2 → GW2)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (7, 1, 'gudang', 1, 'id_gudang', 'Gudang 1'),
  (7, 2, 'gateway', 2, 'id_drop_point', 'Gateway 2'),
  (7, 3, 'gateway', 2, 'id_drop_point', 'Gateway 2');

-- Route 8: D11 incoming R2 (Gudang2 → GW3)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (8, 1, 'gudang', 2, 'id_gudang', 'Gudang 2'),
  (8, 2, 'gateway', 3, 'id_drop_point', 'Gateway 3');

-- Route 9: D11 incoming R3 (Gudang3 → Gudang2 → GW3)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (9, 1, 'gudang', 3, 'id_gudang', 'Gudang 3'),
  (9, 2, 'gudang', 2, 'id_gudang', 'Gudang 2'),
  (9, 3, 'gateway', 3, 'id_drop_point', 'Gateway 3');

-- Route 10: D10 incoming R1 (Gudang2 → GW3)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (10, 1, 'gudang', 2, 'id_gudang', 'Gudang 2'),
  (10, 2, 'gateway', 3, 'id_drop_point', 'Gateway 3');

-- Route 11: D10 incoming R4 (Gudang2 → GW3)
INSERT INTO ritase_stop_template (id_route_template, urutan, jenis_stop, id_lokasi, kolom_lokasi, keterangan) VALUES
  (11, 1, 'gudang', 2, 'id_gudang', 'Gudang 2'),
  (11, 2, 'gateway', 3, 'id_drop_point', 'Gateway 3');
