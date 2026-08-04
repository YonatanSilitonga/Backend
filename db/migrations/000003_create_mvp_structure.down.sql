-- 000003_create_mvp_structure.down.sql
DROP TABLE IF EXISTS ritase_stop;
ALTER TABLE ritase DROP COLUMN IF EXISTS jam_mulai;
ALTER TABLE ritase DROP COLUMN IF EXISTS jam_selesai;
ALTER TABLE users DROP COLUMN IF EXISTS id_driver;
ALTER TABLE driver DROP COLUMN IF EXISTS jenis_driver;
-- kolom jumlah_koli / latitude / longitude TIDAK di-drop (milik data real/ad-hoc)
