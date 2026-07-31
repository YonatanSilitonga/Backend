# 📦 Dokumentasi Backend — Tower Control PT Sentral Logistik Bersama

> Backend API untuk sistem **Tower Control** PT SLB.
> Laravel 12 + MongoDB + Sanctum (token auth) — dibangun 31 Juli 2026.

---

## 1. Ringkasan

Project ini adalah backend API untuk **3 domain operasional sekaligus**:

| Domain | Fungsi | Modul |
|---|---|---|
| 🚚 Monitoring Armada & Driver | Kelola fleet, kendaraan, driver, trip + status realtime | `Armada` |
| 📦 Control Tower Supply Chain | Shipment, tracking paket end-to-end, dashboard agregasi | `SupplyChain` |
| 🗼 Tower Fisik / BTS | Manajemen tower, vendor, kontrak sewa, maintenance, billing | `Tower` |

API dikonsumsi oleh **dashboard Next.js (SPA)** dan **aplikasi mobile**, keduanya via **token Sanctum** (`Authorization: Bearer <token>`).

---

## 2. Stack Teknologi

| Komponen | Versi | Keterangan |
|---|---|---|
| PHP | 8.2.12 | ZTS, XAMPP |
| Laravel Framework | 12.64.0 | `composer create-project laravel/laravel` |
| MongoDB | 8.0.3 | Server lokal di `127.0.0.1:27017` (Windows service) |
| `mongodb/laravel-mongodb` | ^5.9 | Driver resmi MongoDB untuk Eloquent |
| `laravel/sanctum` | ^4.3 | Personal access token (API-only) |

**Database:** `tower_control` (MongoDB lokal, tanpa auth user/password).

---

## 3. Struktur Folder

```
D:\Magang\Backend\tower-control-api\
├── app\
│   ├── Http\Middleware\CheckRole.php      → middleware role (admin, supervisor, dll)
│   ├── Models\
│   │   ├── BaseModel.php                  → base model (convert ObjectId → string)
│   │   ├── Concerns\SerializesObjectIds.php
│   │   ├── User.php                       → extends MongoDB Auth User + HasApiTokens
│   │   └── PersonalAccessToken.php        → override token model biar jalan di MongoDB
│   ├── Sanctum\AccessToken.php            → hasil createToken custom (ganti NewAccessToken)
│   └── Modules\
│       ├── Auth\Http\Controllers\AuthController.php
│       ├── Armada\Models\{Fleet,Vehicle,Driver,Trip}.php
│       ├── Armada\Http\Controllers\{Fleet,Vehicle,Driver,Trip}Controller.php
│       ├── SupplyChain\Models\{Shipment,TrackingEvent}.php
│       ├── SupplyChain\Http\Controllers\{Shipment,Tracking,ControlTower}Controller.php
│       └── Tower\Models\{Tower,Vendor,TowerContract,MaintenanceTask,Invoice}.php
│       └── Tower\Http\Controllers\{Tower,Vendor,Contract,Maintenance,Invoice}Controller.php
├── database\
│   ├── migrations\                        → users + personal_access_tokens (MongoDB compatible)
│   └── seeders\DatabaseSeeder.php         → admin + sample data semua modul
└── routes\
    ├── api.php                            → prefix /api/v1 + route named 'login' (JSON 401)
    └── v1\
        ├── api.php                        → master: public + group auth:sanctum
        ├── auth.php                       → register, login, logout, me
        ├── armada.php                     → resource CRUD + PATCH status
        ├── supplychain.php                → shipments, tracking, control-tower summary
        └── tower.php                      → towers, vendors, contracts, maintenance, invoices
```

**Alur request:** `routes/api.php` → `routes/v1/api.php` (group `auth:sanctum`) → controller per modul → MongoDB.

---

## 4. Database (Collections MongoDB)

13 collection terbuat otomatis dari model + migration:

| Collection | Isi penting |
|---|---|
| `users` | name, email, password, **role** (admin/supervisor/driver/vendor/finance), **active** |
| `personal_access_tokens` | token Sanctum (model custom MongoDB) |
| `fleets` | kode, nama, lokasi, status (active/inactive) |
| `vehicles` | plat, tipe, kapasitas_kg, fleet_id, status (available/in_transit/maintenance/off) |
| `drivers` | nama, nik, no_sim, telepon, fleet_id, status (on_duty/off) |
| `trips` | kode, vehicle_id, driver_id, asal, tujuan, jarak_km, status (planned/in_progress/completed/cancelled) |
| `shipments` | no_resi, pengirim, penerima, asal, tujuan, berat_kg, status, trip_id |
| `tracking_events` | shipment_id, status, lokasi, latitude, longitude, deskripsi, event_time |
| `towers` | kode, nama, lokasi, tipe, tinggi_m, jumlah_tenant, status |
| `vendors` | nama, kontak, telepon, spesialisasi, status |
| `tower_contracts` | kode, tower_id, vendor_id, tipe_sewa, biaya_bulanan, tanggal_mulai/selesai, status |
| `maintenance_tasks` | kode, tower_id, vendor_id, jenis, jadwal, biaya, status |
| `invoices` | no_invoice, contract_id, vendor_id, tower_id, periode, jumlah, status (unpaid/paid), due_date, paid_at |

> **Catatan relasi:** relasi antar model pakai referensi `_id` (ObjectId). Di response JSON, `_id` tampil sebagai **`id` (string)** — sudah otomatis di-convert oleh `BaseModel`.

---

## 5. Endpoint API (prefix `/api/v1`)

### 5.1 Auth

| Method | Endpoint | Auth | Deskripsi |
|---|---|---|---|
| POST | `/auth/register` | ❌ | Daftar user (role opsional) |
| POST | `/auth/login` | ❌ | Login → `{ user, token }` |
| POST | `/auth/logout` | ✅ | Revoke token aktif |
| GET | `/auth/me` | ✅ | Data user yang login |

**Contoh login:**
```bash
curl -X POST http://127.0.0.1:8000/api/v1/auth/login \
  -H "Content-Type: application/json" -H "Accept: application/json" \
  -d '{"email":"admin@slb.co.id","password":"password123"}'
```
```json
{
  "success": true,
  "data": {
    "user": { "id": "...", "name": "Admin SLB", "role": "admin", "active": true },
    "token": "1|abc123..."
  }
}
```

### 5.2 Armada

| Method | Endpoint | Deskripsi |
|---|---|---|
| GET/POST | `/armada/fleets` | List / buat fleet |
| GET/PUT/DELETE | `/armada/fleets/{id}` | Detail / update / hapus |
| PATCH | `/armada/fleets/{id}/status` | Update status |
| GET/POST | `/armada/vehicles` | List / buat kendaraan |
| GET/PUT/DELETE | `/armada/vehicles/{id}` | Detail / update / hapus |
| PATCH | `/armada/vehicles/{id}/status` | Set status (available/in_transit/maintenance/off) |
| GET/POST | `/armada/drivers` | List / buat driver |
| GET/PUT/DELETE | `/armada/drivers/{id}` | Detail / update / hapus |
| PATCH | `/armada/drivers/{id}/status` | Set status (on_duty/off) |
| GET/POST | `/armada/trips` | List / buat trip |
| GET/PUT/DELETE | `/armada/trips/{id}` | Detail / update / hapus |
| PATCH | `/armada/trips/{id}/status` | Set status — otomatis isi `started_at`/`completed_at` |

### 5.3 Supply Chain

| Method | Endpoint | Auth | Deskripsi |
|---|---|---|---|
| GET | `/supplychain/control-tower/summary` | ✅ + **role admin/supervisor** | Dashboard agregasi |
| GET/POST | `/supplychain/shipments` | ✅ | List / buat shipment (no_resi otomatis `SLB-XXXXXXXXXX`) |
| GET/PUT/DELETE | `/supplychain/shipments/{id}` | ✅ | Detail / update / hapus |
| PATCH | `/supplychain/shipments/{id}/status` | ✅ | Update status **+ auto catat tracking event** |
| GET | `/supplychain/shipments/{id}/tracking` | ✅ | Riwayat event shipment |
| POST | `/supplychain/shipments/{id}/tracking` | ✅ | Tambah event manual |
| GET | `/supplychain/tracking/{no_resi}` | ❌ **public** | Cek resi (bisa buat customer) |

**Control Tower summary** mengembalikan (via aggregation MongoDB):
- `total_shipments` — jumlah seluruh shipment
- `shipment_by_status` — breakdown per status
- `shipment_trend_days` — tren per hari (default 7 hari, bisa `?days=30`)
- `top_routes` — top 5 rute asal→tujuan
- `vehicle_by_status` — kondisi armada
- `recent_events` — 10 tracking event terbaru (live feed)

### 5.4 Tower Fisik

| Method | Endpoint | Deskripsi |
|---|---|---|
| GET/POST | `/tower/towers` | List / buat tower |
| GET/PUT/DELETE | `/tower/towers/{id}` | Detail / update / hapus |
| GET/POST | `/tower/vendors` | List / buat vendor |
| GET/PUT/DELETE | `/tower/vendors/{id}` | Detail / update / hapus |
| GET/POST | `/tower/contracts` | List / buat kontrak sewa (kode otomatis `KTR-XXXXXXXX`) |
| GET/PUT/DELETE | `/tower/contracts/{id}` | Detail / update / hapus |
| GET/POST | `/tower/maintenance` | List / buat task maintenance |
| GET/PUT/DELETE | `/tower/maintenance/{id}` | Detail / update / hapus |
| GET | `/tower/invoices` | List invoice |
| POST | `/tower/invoices/generate` | Generate invoice dari kontrak (`contract_id` + `periode`) |
| PATCH | `/tower/invoices/{id}/paid` | Tandai lunas |
| GET | `/tower/invoices/billing/summary` | Total billing, lunas, belum bayar, overdue |

---

## 6. Autentikasi & Role

- **Semua endpoint** butuh token, **kecuali**: `auth/login`, `auth/register`, `supplychain/tracking/{no_resi}`.
- Kirim header pada setiap request:
  ```
  Authorization: Bearer <token>
  Accept: application/json
  ```
- **Role:** `admin`, `supervisor`, `driver`, `vendor`, `finance` (disimpan di `users.role`).
- Middleware `role:admin,supervisor` dipakai pada `control-tower/summary`. Contoh blocked (403):
  ```json
  { "success": false, "message": "Forbidden. Role tidak diizinkan untuk akses ini." }
  ```
- Logout otomatis **revoke token** → token bekas langsung 401.

### Akun default (seeder)

| Role | Email | Password |
|---|---|---|
| Admin | admin@slb.co.id | password123 |
| Supervisor | supervisor@slb.co.id | password123 |

---

## 7. ⚠️ Jebatan Teknis & Solusi (penting untuk pengembangan lanjutan)

### 7.1 Migration `personal_access_tokens` gagal di MongoDB
`$table->morphs('tokenable')` memicu `numericMorphs()` → error `after()` di MongoDB schema builder.
**Solusi:** tulis manual:
```php
$table->string('tokenable_type');
$table->objectId('tokenable_id');
$table->index(['tokenable_type', 'tokenable_id']);
```

### 7.2 Sanctum `createToken()` crash — "Call to a member function prepare() on null"
`Laravel\Sanctum\PersonalAccessToken` extends Eloquent SQL biasa → insert lewat SQL builder → crash di koneksi MongoDB (driver MongoDB tidak mengimplementasikan `prepare()`).
**Solusi berlapis:**
1. `App\Models\PersonalAccessToken` extends `MongoDB\Laravel\Eloquent\Model`, implement `HasAbilities` (method `can`, `cant`, `findToken`), daftarkan via `Sanctum::usePersonalAccessTokenModel()` di `AppServiceProvider`.
2. `NewAccessToken` punya type-hint ketat ke model Sanctum → **tidak bisa extends** → buat `App\Sanctum\AccessToken` (tanpa extends) + **override `createToken()` di `User` model**.

### 7.3 ObjectId → Extended JSON di response
Tanpa fix, `_id` tampil sebagai `{"$oid": "..."}` (Extended JSON) — merepotkan frontend.
**Solusi:** `App\Models\BaseModel` (trait `SerializesObjectIds`) convert semua ObjectId → string rekursif saat `toArray()`. Semua model module extends `BaseModel`; `User` & `PersonalAccessToken` pakai trait langsung. Response JSON memakai `id` string.

### 7.4 "Route [login] not defined" di API-only
Terjadi saat request **tanpa header `Accept: application/json`** — Sanctum menganggap request web dan mencoba redirect ke route `login`.
**Solusi:** definisikan route named `login` di `routes/api.php` yang return JSON 401, **dan** pastikan client kirim header `Accept: application/json`.

### 7.5 Encoding PowerShell 5.1
`Set-Content -Encoding UTF8` menulis **BOM** di awal file → PHP fatal `Namespace declaration statement has to be the very first statement`.
**Solusi:** gunakan `[System.IO.File]::WriteAllText($path, $c, [System.Text.UTF8Encoding]::new($false))`.

---

## 8. Cara Menjalankan

```bash
# 1. Install dependencies
composer install

# 2. Environment
cp .env.example .env
#   pastikan: DB_CONNECTION=mongodb, DB_DATABASE=tower_control
php artisan key:generate

# 3. Migrasi + seed (harus MongoDB jalan di 127.0.0.1:27017)
php artisan migrate --seed

# 4. Jalankan server
php artisan serve        # → http://127.0.0.1:8000
```

**Cek server:** `GET http://127.0.0.1:8000/up` → `200`.

**Reset data:**
```bash
php artisan migrate:fresh --seed
```

---

## 9. Status & Roadmap

### ✅ Selesai (31 Juli 2026)
- [x] Setup Laravel 12 + MongoDB + Sanctum
- [x] Struktur modular (Auth, Armada, SupplyChain, Tower)
- [x] Auth lengkap (register, login, logout, me, role)
- [x] CRUD Armada + update status realtime
- [x] Shipment + tracking event + tracking public by resi
- [x] Control Tower summary (aggregation MongoDB)
- [x] Tower Fisik: tower, vendor, kontrak, maintenance, invoice/billing
- [x] Seeder data contoh
- [x] **Verifikasi: 15/15 endpoint PASS**

### 📋 Belum (ide lanjutan)
- [ ] Commit & push ke git repo `D:\Magang\Backend`
- [ ] RAG / Knowledge Base (biar AI bisa jawab dari data operasional)
- [ ] SQL/Query generator via AI
- [ ] Realtime monitoring (WebSocket / polling terpadu)
- [ ] Pagination di endpoint list (saat ini `limit` default)
- [ ] Rate limiting & refresh token
- [ ] Test otomatis (PHPUnit)

---

## 10. Catatan Debug

- Log error: `storage/logs/laravel.log`
- Cek collection langsung:
  ```bash
  mongosh tower_control --eval "db.shipments.find().limit(3).pretty()"
  ```
- List route:
  ```bash
  php artisan route:list --path=api
  ```
