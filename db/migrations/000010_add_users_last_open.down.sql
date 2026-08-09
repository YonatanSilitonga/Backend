-- 000010_add_users_last_open.down.sql
ALTER TABLE users DROP COLUMN IF EXISTS last_open;