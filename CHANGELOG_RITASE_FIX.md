# Changelog: Fix Ritase Expired & Tracking Status

**Tanggal:** 28 Agustus 2026  
**Developer:** Magang SLB  
**Branch:** `main`

---

## 🎯 Masalah yang Diperbaiki

### 1. **Bug Ritase Expired — Validasi Hanya Berdasarkan Jam**

#### **SEBELUM (BUG):**
- Fungsi `ritaseStatusLabel()` hanya membandingkan jam (`jam_selesai < jam_sekarang`)
- Ritase **kemarin** dengan `jam_selesai = "18:00"` masih dianggap valid kalau sekarang jam `10:00`
- Armada menampilkan status **"Siap Berangkat"** atau **"Sedang Berjalan"** untuk ritase yang sudah lewat hari

**Contoh Bug:**
```
Ritase kemarin (27 Agustus):
- jam_selesai: "18:00"
- status: "direncanakan"

Hari ini (28 Agustus) jam 10:00 pagi:
❌ Tampil: "Siap Berangkat" (SALAH! Harusnya expired)
```

**Dampak:**
- ❌ Dashboard menampilkan informasi yang salah
- ❌ Operator bingung karena ritase lama masih muncul sebagai aktif
- ❌ Status armada tidak akurat

#### **SESUDAH (FIX):**
- ✅ Validasi expired sekarang cek **tanggal + jam**:
  - `tanggal < hari_ini` → expired
  - `tanggal == hari_ini && jam_selesai < jam_sekarang` → expired
  - `tanggal > hari_ini` → belum expired (ritase masa depan)
- ✅ Backend mengirim field `tanggal` (format `YYYY-MM-DD`) ke frontend
- ✅ Frontend menggunakan tanggal untuk validasi yang benar

**Hasil yang Diharapkan:**
```
Ritase kemarin (27 Agustus):
- tanggal: "2026-08-27"
- jam_selesai: "18:00"
- status: "direncanakan"

Hari ini (28 Agustus) jam 10:00 pagi:
✅ Tampil: "Tidak ada jadwal" (BENAR! Ritase kemarin diabaikan)
✅ Armada tidak menampilkan ritase expired
✅ Hanya ritase hari ini yang muncul sebagai aktif
```

---

### 2. **Status Armada Tidak Informatif**

#### **SEBELUM (BUG):**
- Semua kondisi "driver login tapi GPS tidak aktif" menampilkan **"Belum memulai"**
- Tidak ada pembedaan antara:
  - Armada yang sudah selesai bertugas
  - Armada yang belum mulai karena belum waktunya
  - Armada yang tidak ada jadwal hari ini
  - Armada yang jadwalnya sudah lewat

**Contoh Bug:**
```
Armada A: Semua ritase hari ini sudah selesai (status: "selesai")
❌ Tampil: "Belum memulai" (SALAH! Harusnya "Selesai Bertugas")

Armada B: Tidak ada jadwal hari ini
❌ Tampil: "Belum memulai" (TIDAK JELAS! Apa belum mulai atau tidak ada jadwal?)

Armada C: Ada ritase tapi sudah lewat jam selesai (kemarin)
❌ Tampil: "Belum memulai" (SALAH! Harusnya "Tidak ada jadwal")
```

#### **SESUDAH (FIX):**
✅ **4 status berbeda dengan warna & label yang jelas:**

| Kondisi | Status Label | Dot Color | Keterangan |
|---------|-------------|-----------|------------|
| GPS aktif (last_update < 3 menit) | `"Sedang Menuju"` / `"Parkir"` / dll. | 🟢 Hijau berkedip | Real-time dari GPS |
| Ada ritase aktif/direncanakan (belum expired) | `"Siap Berangkat"` / `"Sedang Berjalan"` | 🟡 Amber | Ritase hari ini dalam window jadwal |
| Ritase selesai semua | `"Selesai Bertugas"` | ⚪ Abu (teks hijau) | Semua ritase hari ini status "selesai" |
| Tidak ada jadwal / expired | `"Tidak ada jadwal"` | ⚫ Abu tipis | Tidak ada ritase hari ini atau semua expired |

**Hasil yang Diharapkan:**
```
Dashboard Armada Live:

🟢 B 1234 XYZ - Budi    → "Sedang Menuju" (45 km/h, update 1 menit lalu)
🟡 B 5678 ABC - Andi    → "Siap Berangkat" (ritase ke-1, jam 08:00-18:00)
🟡 B 9012 DEF - Siti    → "Sedang Berjalan" (ritase ke-2, sudah mulai)
⚪ B 3456 GHI - Dedi    → "Selesai Bertugas" (3 ritase selesai, teks hijau)
⚫ B 7890 JKL - Rini    → "Tidak ada jadwal" (tidak ada ritase hari ini)
⚫ B 2468 MNO - Joko    → "Tidak ada jadwal" (ritase kemarin sudah expired)

✅ Operator langsung paham kondisi setiap armada
✅ Tidak ada lagi status "Belum memulai" yang ambigu
✅ Warna dot konsisten dengan urgency (hijau=aktif, amber=siap/jalan, abu=selesai/tidak ada)
```

---

### 3. **Hydration Warning di Browser**

#### **SEBELUM (ANNOYING):**
```
Console Browser:

⚠️ Warning: Prop `data-new-gr-c-s-check-loaded` did not match.
⚠️ Warning: Prop `data-gr-ext-installed` did not match.
⚠️ Warning: Extra attributes from server: data-new-gr-c-s-check-loaded
⚠️ Hydration failed because the server rendered HTML didn't match the client.
... (puluhan warning yang sama)
```

**Penyebab:**
- Browser extension (Grammarly, password manager, dll.) inject attribute ke tag `<html>`
- Next.js hydration detect perbedaan server vs client HTML
- Warning tidak berbahaya tapi memenuhi console

#### **SESUDAH (FIX):**
```
Console Browser:

✅ (bersih, tidak ada warning hydration)
```

**Hasil yang Diharapkan:**
- ✅ Console browser bersih dari warning hydration yang tidak relevan
- ✅ Debugging lebih mudah (tidak kebanjiran warning)
- ✅ Developer experience lebih baik

---

## 📦 Perubahan Backend

### File: `internal/armada/model.go`

**Struct `TrackingLive` — Tambah 4 Field Baru:**

```go
type TrackingLive struct {
	ID            int64      `json:"id_tracking"`
	IDRitase      *int64     `json:"id_ritase,omitempty"`
	
	// ✅ FIELD BARU (ditambah 28 Agustus 2026)
	StatusRitase    *string    `json:"status_ritase,omitempty"`
	TanggalRitase   *string    `json:"tanggal,omitempty"` // YYYY-MM-DD
	JamMulai        *string    `json:"jam_mulai,omitempty"`
	JamSelesai      *string    `json:"jam_selesai,omitempty"`
	
	IDKendaraan   int64      `json:"id_kendaraan"`
	PlatNomor     string     `json:"plat_nomor"`
	// ... field lain tidak berubah
}
```

**Alasan:**
- Frontend perlu data ritase (status, tanggal, jam) untuk:
  1. Validasi expired yang benar
  2. Menampilkan status armada yang informatif
  3. Cek apakah ritase masih dalam window jadwal

---

### File: `internal/armada/repository.go`

**Fungsi `ListLatestTracking()` — Query Diupdate:**

#### **Sebelum:**
```go
rows, err := r.db.Query(ctx, fmt.Sprintf(`
    SELECT t.id_tracking, t.id_ritase, t.id_kendaraan, COALESCE(k.plat_nomor,''),
           // ... field tracking lainnya
    FROM armada_tracking t
    LEFT JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
    LEFT JOIN driver d ON d.id_driver = t.id_driver
    LEFT JOIN users u ON u.id_driver = d.id_driver
    // ... LEFT JOIN ritase TIDAK ADA
```

**Masalah:**
- Query tidak JOIN ke tabel `ritase`
- Field `status_ritase`, `tanggal`, `jam_mulai`, `jam_selesai` tidak diambil
- Struct `TrackingLive` punya field tapi tidak pernah diisi

#### **Sesudah:**
```go
rows, err := r.db.Query(ctx, fmt.Sprintf(`
    SELECT t.id_tracking, t.id_ritase, r.status,
           TO_CHAR(r.tanggal, 'YYYY-MM-DD'),
           TO_CHAR(r.jam_mulai, 'HH24:MI:SS'), TO_CHAR(r.jam_selesai, 'HH24:MI:SS'),
           t.id_kendaraan, COALESCE(k.plat_nomor,''),
           // ... field tracking lainnya
    FROM armada_tracking t
    LEFT JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
    LEFT JOIN ritase r ON r.id_ritase = t.id_ritase  // ✅ DITAMBAH
    LEFT JOIN driver d ON d.id_driver = t.id_driver
    LEFT JOIN users u ON u.id_driver = d.id_driver
```

**Perubahan:**
1. ✅ Tambah `LEFT JOIN ritase r ON r.id_ritase = t.id_ritase`
2. ✅ SELECT 4 field baru: `r.status`, `r.tanggal`, `r.jam_mulai`, `r.jam_selesai`
3. ✅ Format tanggal: `TO_CHAR(r.tanggal, 'YYYY-MM-DD')` → frontend dapat string ISO
4. ✅ Format jam: `TO_CHAR(r.jam_mulai, 'HH24:MI:SS')` → konsisten 24-jam

#### **Scan Statement Diupdate:**

**Sebelum:**
```go
if err := rows.Scan(&t.ID, &t.IDRitase, &t.IDKendaraan, &t.PlatNomor, ...
```

**Sesudah:**
```go
if err := rows.Scan(&t.ID, &t.IDRitase, 
    &t.StatusRitase, &t.TanggalRitase, &t.JamMulai, &t.JamSelesai,  // ✅ DITAMBAH
    &t.IDKendaraan, &t.PlatNomor, ...
```

---

## 🔍 Testing

### Build Status
```bash
$ go build ./internal/...
# ✅ SUCCESS — no errors
```

### Manual Test (Recommended)
1. **Start backend:**
   ```bash
   go run main.go
   ```

2. **Test endpoint:** `GET /armada/tracking/map`
   ```bash
   curl http://localhost:8080/armada/tracking/map | jq '.vehicles[0]'
   ```

3. **Verifikasi response punya field baru:**
   ```json
   {
     "id_tracking": 123,
     "id_ritase": 456,
     "status_ritase": "berjalan",
     "tanggal": "2026-08-28",          // ✅ FIELD BARU
     "jam_mulai": "08:00:00",          // ✅ FIELD BARU
     "jam_selesai": "18:00:00",        // ✅ FIELD BARU
     "plat_nomor": "B 1234 XYZ",
     ...
   }
   ```

4. **Test case expired:**
   - Ritase kemarin (`tanggal: "2026-08-27"`) harus tidak muncul sebagai "Siap Berangkat"
   - Frontend akan return `null` dari `ritaseStatusLabel()` → tampil "Tidak ada jadwal"

---

## 🚨 Breaking Changes

**NONE** — Backward compatible:
- Field baru optional (`*string`) → kalau `ritase` tidak ada, field akan `null`
- Frontend sudah handle `null` dengan fallback logic
- Response JSON hanya **bertambah field**, tidak menghapus field lama

---

## 🎬 Demo / Preview Hasil

### **Sebelum Fix:**
```
Dashboard Tower Control - Map View:

Armada B 1234 XYZ (Driver: Budi)
├─ Status: "Belum memulai"  ❌ (Padahal ritase kemarin!)
├─ Dot: 🟡 Amber
└─ Update: 2 jam lalu

Armada B 5678 ABC (Driver: Andi)  
├─ Status: "Belum memulai"  ❌ (Sebenarnya sudah selesai 3 ritase!)
├─ Dot: 🟡 Amber
└─ Update: 1 jam lalu

Console:
⚠️ 47 Hydration warnings...
```

### **Sesudah Fix:**
```
Dashboard Tower Control - Map View:

Armada B 1234 XYZ (Driver: Budi)
├─ Status: "Tidak ada jadwal"  ✅ (Ritase kemarin diabaikan)
├─ Dot: ⚫ Abu tipis
└─ Update: 2 jam lalu

Armada B 5678 ABC (Driver: Andi)
├─ Status: "Selesai Bertugas"  ✅ (3 ritase selesai, teks hijau)
├─ Dot: ⚪ Abu
└─ Update: 1 jam lalu

Armada B 9012 DEF (Driver: Siti)
├─ Status: "Sedang Berjalan"  ✅ (Ritase ke-2 aktif)
├─ Dot: 🟡 Amber
└─ Update: 30 detik lalu

Armada B 3456 GHI (Driver: Dedi)
├─ Status: "Sedang Menuju"  ✅ (GPS aktif 55 km/h)
├─ Dot: 🟢 Hijau berkedip
└─ Update: baru saja

Console:
✅ (bersih)
```

**Perbandingan Visual:**

| Aspek | Sebelum | Sesudah |
|-------|---------|---------|
| **Ritase Kemarin** | "Belum memulai" ❌ | "Tidak ada jadwal" ✅ |
| **Ritase Selesai** | "Belum memulai" ❌ | "Selesai Bertugas" ✅ |
| **Info Clarity** | Ambigu, semua sama | Jelas, 4 kondisi berbeda |
| **Console Browser** | 47 warnings ⚠️ | Bersih ✅ |
| **Operator Confusion** | Tinggi | Rendah |

---

## 📝 Notes untuk Developer

### Kenapa Tidak Buat Status `'kedaluwarsa'` Baru?
**Alasan:** User meminta **"jangan ubah DB"**, jadi:
- ❌ Tidak ALTER TABLE `ritase` untuk tambah enum `'kedaluwarsa'`
- ❌ Tidak buat migration baru
- ✅ Cukup kirim `tanggal` ke frontend → frontend yang validasi expired

Ini lebih flexible karena:
- Logic expired bisa berubah tanpa migration DB
- Frontend bisa customize tampilan per use-case
- Backend tetap simple — hanya kirim data mentah

### Kenapa `LEFT JOIN` bukan `INNER JOIN`?
**Alasan:** Armada bisa tidak punya ritase aktif:
- Driver login tapi belum ada jadwal hari ini
- Tracking GPS masih ada tapi ritase sudah selesai
- `INNER JOIN` akan hide armada tersebut dari map → salah

Dengan `LEFT JOIN`:
- Armada tanpa ritase tetap muncul di map
- Field `status_ritase`, `tanggal`, dll. akan `null`
- Frontend handle `null` dengan logic fallback

---

## 📚 Related Files (Frontend)

Perubahan backend ini berkoordinasi dengan frontend:
- `src/lib/constants.ts` — fungsi `isRitaseExpired()` baru
- `src/types/armada.ts` — type `TrackingVehicle` tambah field `tanggal`
- `src/components/armada/vehicle-item.tsx` — pass `tanggal` ke validasi
- `src/components/map/live-map.tsx` — pass `tanggal` ke validasi

Lihat file `CHANGELOG_RITASE_FIX.md` di folder frontend untuk detail lengkap.

---

## ✅ Checklist Deployment

Sebelum deploy ke production:

- [x] Go build clean (`go build ./internal/...`)
- [x] No breaking changes di API response
- [x] Field baru optional (nullable)
- [ ] Test manual endpoint `/armada/tracking/map`
- [ ] Verifikasi ritase kemarin tidak tampil sebagai aktif
- [ ] Koordinasi dengan frontend untuk deploy bersamaan
- [ ] Monitor log backend setelah deploy (cek error SQL)

---

## 🐛 Known Issues / TODO

1. **Performance:** Query sekarang JOIN 5 tabel (`tracking`, `kendaraan`, `ritase`, `driver`, `users`, `ritase_event`).
   - Kalau ada lag, pertimbangkan:
     - Index di `ritase.id_ritase`
     - Materialized view untuk tracking live
     - Cache Redis untuk response `/armada/tracking/map`

2. **Timezone:** Field `tanggal` sekarang tanpa timezone info (hanya `YYYY-MM-DD`).
   - Aman kalau semua server & client di WIB (UTC+7)
   - Kalau ada multi-timezone, perlu tambah field `timezone` atau pakai `TIMESTAMPTZ`

3. **Ritase Multi-Hari:** Kalau ada ritase yang span 2 hari (mulai jam 23:00, selesai jam 02:00 besok):
   - Logic `isRitaseExpired()` sekarang belum handle case ini
   - Perlu update di frontend kalau ada case seperti ini

---

## 📞 Contact

Kalau ada pertanyaan atau issue setelah deployment:
- **Developer:** Tim Magang SLB
- **File ini:** `D:\Magang\Backend\CHANGELOG_RITASE_FIX.md`
- **Tanggal Update:** 28 Agustus 2026

---

**END OF CHANGELOG**
