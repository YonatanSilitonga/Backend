-- 000009_add_users_last_login.up.sql
-- Penanda session online driver (login → now(), logout → NULL).
-- Dipakai web: driver "Online (session)" walau GPS stale (background/layar mati).
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login TIMESTAMP;