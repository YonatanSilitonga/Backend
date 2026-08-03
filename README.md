# Backend — Monitoring Armada / Tower Control (Go + Echo + PostgreSQL)

Backend untuk aplikasi monitoring armada & tower control. Framework: **Go** + **Echo v4**, database: **PostgreSQL** (via **Supabase** di cloud).

## Struktur

```
├── main.go                  # entry point: init Echo + konek DB + health check
├── internal/
│   ├── config/              # baca konfigurasi dari env
│   ├── database/            # koneksi pgx pool ke PostgreSQL
│   └── pkg/
│       ├── response/        # format JSON seragam { success, data, message }
│       └── middleware/      # logger, recover, cors
└── db/migrations/           # file migration SQL (golang-migrate)
```

## Setup

1. Buat project di [Supabase](https://supabase.com) (region: Singapore).
2. Salin connection string URI: **Settings → Database → Connection string → URI** (port `5432`).
3. Salin `.env.example` ke `.env`, isi `DATABASE_URL`.

```
cp .env.example .env   # lalu edit DATABASE_URL
```

4. Jalankan server:

```
go run main.go
```

5. Cek: `GET http://localhost:8080/health` → harus balas `{ "success": true, ... }`.

## Command

| Perintah | Fungsi |
|---|---|
| `make run` | jalankan server |
| `make build` | build binary ke `bin/backend` |
| `make migrate-up` | jalankan migration |
| `make migrate-down` | rollback 1 migration |

## Catatan

- `.env` **jangan pernah di-commit** (sudah ada di `.gitignore`).
- Arsitektur microservice & realtime tracking akan ditambahkan bertahap setelah framework dasar jalan.
