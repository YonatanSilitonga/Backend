-- 000010_add_users_last_open.up.sql
-- Kapan terakhir app mobile dibuka (telemetry). Dipakai web: "Terakhir buka app".
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_open TIMESTAMP;