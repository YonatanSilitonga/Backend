# API Contract — Backend Tower Control

Backend: Go + Echo, satu API `/api/v1`, dipakai **dua client**:
- **Web** (TowerControll, Next.js) — role `direktur`, `kapten` (JWT)
- **Mobile** (app driver, dikerjakan Whisnu) — role `driver`

## Aturan main (supaya gak saling rusak)

1. **Additive-only**: kalau perlu endpoint baru → TAMBAH. Jangan hapus/ganti bentuk respons yang sudah dipakai client lain.
2. **Login satu handler**: `POST /auth/login` terima `username` ATAU `email`, respons `{ user, token }`.
3. Kalau mau ubah respons endpoint yang dipakai bareng → konfirmasi dulu di sini.
4. Format respons semua endpoint: `{ success, data, message }`.

## Endpoint

### Public (tanpa token) — dipakai web & mobile
| Method | Path | Owner | Catatan |
|---|---|---|---|
| GET | `/health` | bareng | health check |
| POST | `/auth/login` | bareng | `{username|email, password}` → `{user, token}` |

### Mobile scope (public, milik Whisnu — JANGAN DIUBAH)
| Method | Path | Response |
|---|---|---|
| GET | `/sellers` | `[{id, code, name, address, city, pic, no_hp}]` |
| GET | `/drivers` | `[{id, name, no_hp, status}]` |
| GET | `/vehicles` | `[{id, plat, type, capacity_kg, status}]` |

### Shared (butuh token JWT)
| Method | Path | Catatan |
|---|---|---|
| GET | `/armada/ritase` | list penugasan; driver scoping menyusul |
| GET | `/armada/ritase/:id` | detail + timeline event |
| POST | `/armada/ritase/:id/status` | tombol status driver |
| PATCH | `/armada/ritase/:id/muatan` | AWB/koli/tertinggal |
| POST | `/armada/tracking` | kirim GPS |

### Web scope (JWT, role direktur/kapten)
| Method | Path | Catatan |
|---|---|---|
| GET | `/auth/me`, POST `/auth/logout` | sesi |
| GET | `/armada/kendaraan`, `/armada/driver` | master data |
| GET | `/armada/tracking` | monitor posisi |
| GET | `/dashboard/summary` | KPI |
| GET | `/dashboard/analisis` | durasi + bottleneck + alert |
| POST | `/armada/ritase` | assign driver (kapten) |

## DB

Sumber kebenaran: Supabase PostgreSQL. Web & mobile baca tabel yang sama
(kendaraan, driver, seller, ritase, ritase_event, armada_tracking, dst).
Migration terbaru: `db/migrations/000002_create_ritase_event.*` (tabel
`ritase_event` + kolom `paket_tertinggal`, `alasan_tertinggal`, `created_at`).

Tools: `tools/migrate` (jalankan migration), `tools/seed` (isi data contoh).
