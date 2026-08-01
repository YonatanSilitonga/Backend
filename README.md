# Tower Control API — PT Sentral Logistik Bersama

Backend API untuk sistem **Tower Control** PT SLB. Laravel 12 + MongoDB + Sanctum (token auth).

> 📄 Dokumentasi lengkap (arsitektur, skema DB, semua endpoint, troubleshooting): **[DOKUMENTASI.md](./DOKUMENTASI.md)**

## Stack

- **Laravel 12** (PHP 8.2)
- **MongoDB** (via `mongodb/laravel-mongodb` v5)
- **Laravel Sanctum** (personal access token — buat dashboard SPA & mobile app)

## Struktur Modular

```
app/Modules/
├── Auth/          → register, login, logout, me (token Sanctum)
├── Armada/        → fleet, vehicle, driver, trip + update status
├── SupplyChain/   → shipment, tracking event, control tower summary
└── Tower/         → tower fisik, vendor, kontrak sewa, maintenance, invoice/billing
```

## Setup

```bash
composer install
cp .env.example .env          # sesuaikan koneksi MongoDB
php artisan key:generate
php artisan migrate --seed    # seed admin + sample data
php artisan serve             # http://localhost:8000
```

## Akun Default (Seeder)

| Role | Email | Password |
|---|---|---|
| Admin | admin@slb.co.id | password123 |
| Supervisor | supervisor@slb.co.id | password123 |

## API Endpoints (prefix `/api/v1`)

### Auth
- `POST /auth/register` — daftar user baru (role: admin, supervisor, driver, vendor, finance)
- `POST /auth/login` — login, balikin `token` + `user`
- `POST /auth/logout` — revoke token
- `GET /auth/me` — user yang login

### Armada
- `GET|POST /armada/fleets` • `GET|PUT|DELETE /armada/fleets/{id}` • `PATCH /armada/fleets/{id}/status`
- `GET|POST /armada/vehicles` • `GET|PUT|DELETE /armada/vehicles/{id}` • `PATCH /armada/vehicles/{id}/status`
- `GET|POST /armada/drivers` • `GET|PUT|DELETE /armada/drivers/{id}` • `PATCH /armada/drivers/{id}/status`
- `GET|POST /armada/trips` • `GET|PUT|DELETE /armada/trips/{id}` • `PATCH /armada/trips/{id}/status`

### Supply Chain
- `GET /supplychain/control-tower/summary` — **control tower dashboard** (butuh role admin/supervisor): total shipment, breakdown status, tren harian, top rute, status kendaraan, live events
- `GET|POST /supplychain/shipments` • `GET|PUT|DELETE /supplychain/shipments/{id}`
- `PATCH /supplychain/shipments/{id}/status` — update status + auto catat tracking event
- `GET|POST /supplychain/shipments/{id}/tracking` — riwayat & tambah event
- `GET /supplychain/tracking/{no_resi}` — **public**, cek resi tanpa login

### Tower Fisik
- `GET|POST /tower/towers` • `GET|PUT|DELETE /tower/towers/{id}`
- `GET|POST /tower/vendors` • `GET|PUT|DELETE /tower/vendors/{id}`
- `GET|POST /tower/contracts` • `GET|PUT|DELETE /tower/contracts/{id}`
- `GET|POST /tower/maintenance` • `GET|PUT|DELETE /tower/maintenance/{id}`
- `GET /tower/invoices` • `POST /tower/invoices/generate` (billing dari kontrak)
- `PATCH /tower/invoices/{id}/paid` • `GET /tower/invoices/billing/summary`

## Auth

Semua endpoint (kecuali `auth/login`, `auth/register`, `tracking/{no_resi}`) butuh header:

```
Authorization: Bearer <token>
```

Role check via middleware `role:admin,supervisor` (contoh: control-tower summary).

## Catatan Teknis

- Model token Sanctum di-override (`App\Models\PersonalAccessToken`) biar kompatibel dengan MongoDB (tanpa ini `createToken` crash).
- Semua model extend `App\Models\BaseModel` yang convert ObjectId → string, jadi response JSON pakai `id` string biasa, bukan Extended JSON.
- Koneksi MongoDB: `DB_CONNECTION=mongodb`, database `tower_control`.
