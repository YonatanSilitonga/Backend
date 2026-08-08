-- 000009_add_users_last_login.down.sql
ALTER TABLE users DROP COLUMN IF EXISTS last_login;