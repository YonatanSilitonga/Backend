# ROADMAP MVP — Distribution Monitoring System (2 Minggu)

**Tim:** Magang Mobile & Backend/Dashboard · DC Tangerang · PT Sentral Logistik Bersama
**Timeline:** 10 hari kerja (Senin–Jumat) · **Presentasi: Jumat, 14 Agustus 2026**

> Versi markdown ini versi yang bisa di-commit (sumber: `Roadmap_Pengerjaan_MostViableProduct_2Minggu (1).xlsx` — file binary-nya sengaja tidak di-commit).

## Ringkasan

| Sub-Tim | Fokus | Status |
|---|---|---|
| Tim Mobile | Aplikasi Android Driver (Flutter) | On-Track (90%) |
| Tim Backend & Dashboard | REST API, PostgreSQL, Dashboard Web | On-Track (95%) |

---

## Timeline & Task (10 hari kerja)

| No | Fase / Modul | Detail Output & Deliverables | Mulai | Selesai | Durasi | Status | Progress |
|---|---|---|---|---|---|---|---|
| 1.0 | Inisiasi & Requirement | Observasi lapangan DC Tangerang & diskusi DirOps | 03-08 | 04-08 | 2 | ✅ Selesai | 100% |
| 1.1 | Inisiasi & Requirement | Pemetaan process flow ritase driver & titik bottleneck | 04-08 | 05-08 | 2 | ✅ Selesai | 100% |
| 1.2 | Inisiasi & Requirement | Penyusunan spesifikasi fitur MVP (Mobile & Dashboard) | 05-08 | 05-08 | 1 | ✅ Selesai | 100% |
| 2.0 | Tim Mobile (Android) | Setup project Flutter, UI/UX penugasan & status driver | 05-08 | 07-08 | 3 | ✅ Selesai | 100% |
| 2.1 | Tim Mobile (Android) | Modul form input AWB, Koli, & timestamp loading/unloading | 07-08 | 10-08 | 3 | ✅ Selesai | 100% |
| 2.2 | Tim Mobile (Android) | Integrasi background GPS tracking & pengiriman data ke server | 11-08 | 13-08 | 3 | 🔄 Dalam Proses | 70% |
| 3.0 | Tim Backend & DB | Perancangan database PostgreSQL & schema data ritase | 05-08 | 06-08 | 2 | ✅ Selesai | 100% |
| 3.1 | Tim Backend & DB | Pengembangan REST API Ingestion Data & Timestamp Driver | 06-08 | 10-08 | 3 | ✅ Selesai | 100% |
| 3.2 | Tim Backend & DB | Engine kalkulasi durasi ritase & deteksi bottleneck | 10-08 | 10-08 | 3 | ✅ Selesai | 100% |
| 4.0 | Dashboard Monitoring | Setup Dashboard Web, UI Live Tracking Status Armada | 07-08 | 10-08 | 3 | ✅ Selesai | 100% |
| 4.1 | Dashboard Monitoring | Modul Analytics Metrics (Rata-rata Loading, Travel, Unloading) | 10-08 | 10-08 | 3 | ✅ Selesai | 100% |
| 5.0 | Testing & Evaluasi | Integration testing Mobile App & REST API Backend | 11-08 | 12-08 | 2 | 🔄 Dalam Proses | 30% |
| 5.1 | Testing & Evaluasi | Uji coba lapangan (UAT) pengumpulan data operasional nyata | 12-08 | 13-08 | 2 | ⏳ Belum | 0% |
| 5.2 | Testing & Evaluasi | **Presentasi evaluasi data operasional ke DirOps & Dirut** | 13-08 | **14-08** | 1 | ⏳ Belum | 0% |

**Total progress: 78,6%**

---

## Struktur Pembagian Tugas Tim

| Sub-Tim | Fokus Utama | Teknologi / Stack | Status |
|---|---|---|---|
| Tim Mobile | Aplikasi Android Driver — penugasan, status perjalanan, GPS, AWB/Koli, offline handling | Android / Flutter | On-Track (90%) |
| Tim Backend & Dashboard | REST API + PostgreSQL, engine durasi, dashboard monitoring real-time + analytics | Golang, PostgreSQL, Web (Next.js) | On-Track (95%) |

---

## Catatan
- Backend & Dashboard selesai lebih awal dari jadwal; sisa waktu fokus ke UAT & presentasi.
- 2.2 (background GPS) menunggu **tes perangkat** & sinkronisasi dengan app driver.
- Presentasi final: **Jumat, 14 Agustus 2026**.
