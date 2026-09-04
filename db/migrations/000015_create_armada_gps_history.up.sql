-- Tabel GPS history: menyimpan setiap titik GPS yang dikirim driver.
-- Digunakan untuk menggambar rute asli yang dilewati di peta.
CREATE TABLE IF NOT EXISTS armada_gps_history (
  id            BIGSERIAL PRIMARY KEY,
  id_ritase     INTEGER REFERENCES ritase(id_ritase) ON DELETE SET NULL,
  id_kendaraan  INTEGER NOT NULL REFERENCES kendaraan(id_kendaraan) ON DELETE CASCADE,
  id_driver     INTEGER NOT NULL REFERENCES driver(id_driver) ON DELETE CASCADE,
  latitude      DOUBLE PRECISION NOT NULL,
  longitude     DOUBLE PRECISION NOT NULL,
  kecepatan     INTEGER DEFAULT 0,
  created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_gps_history_ritase ON armada_gps_history(id_ritase, created_at);
CREATE INDEX IF NOT EXISTS idx_gps_history_kendaraan ON armada_gps_history(id_kendaraan, created_at);
