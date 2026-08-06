// Diag dashboard: jalanin tiap query KPI satu per satu, print yang gagal + error asli.
// Dipakai buat nyari penyebab 500 di /dashboard/summary & /dashboard/analisis.
package main

import (
	"context"
	"fmt"
	"time"

	"backend/internal/config"
	"backend/internal/database"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Println("KONEKSI GAGAL:", err)
		return
	}
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	sumQueries := map[string]string{
		"total_kendaraan":  "SELECT count(*) FROM kendaraan",
		"armada_aktif":     "SELECT count(*) FROM kendaraan WHERE LOWER(status_kendaraan) IN ('aktif','berjalan','bertugas','tersedia')",
		"armada_selesai":   "SELECT count(*) FROM kendaraan WHERE LOWER(status_kendaraan) IN ('selesai','istirahat')",
		"armada_idle":      "SELECT count(*) FROM kendaraan WHERE LOWER(status_kendaraan) IN ('tersedia','idle','off')",
		"total_driver":     "SELECT count(*) FROM driver",
		"driver_aktif":     "SELECT count(*) FROM driver WHERE LOWER(status_driver) IN ('aktif','bertugas','on_duty')",
		"driver_libur":     "SELECT count(*) FROM driver WHERE LOWER(status_driver) IN ('libur','off','cuti')",
		"total_ritase":     "SELECT count(*) FROM ritase",
		"ritase_aktif":     "SELECT count(*) FROM ritase WHERE LOWER(status) NOT IN ('selesai','completed','done','batal','cancelled')",
		"ritase_selesai":   "SELECT count(*) FROM ritase WHERE LOWER(status) IN ('selesai','completed','done')",
		"ritase_today":     "SELECT count(*) FROM ritase WHERE tanggal = $1",
		"total_awb":        "SELECT COALESCE(sum(total_awb),0) FROM ritase",
		"total_awb_today":  "SELECT COALESCE(sum(total_awb),0) FROM ritase WHERE tanggal = $1",
		"total_koli":       "SELECT COALESCE(sum(total_koli),0) FROM ritase",
		"paket_tertinggal": "SELECT COALESCE(sum(paket_tertinggal),0) FROM ritase",
		"total_seller":     "SELECT count(*) FROM seller",
		"seller_terlayani": `SELECT count(DISTINCT rs.id_seller)
			FROM ritase_stop rs
			JOIN ritase r ON r.id_ritase = rs.id_ritase
			WHERE rs.jenis_stop = 'seller' AND LOWER(r.status) IN ('selesai','completed','done')`,
		"total_drop_point": "SELECT count(*) FROM drop_point",
		"total_karyawan":   "SELECT count(*) FROM karyawan",
		"total_manpower":   "SELECT COALESCE(sum(jumlah_manpower),0) FROM implant",
		"total_absensi":    "SELECT count(*) FROM absensi",
		"total_implant":    "SELECT count(*) FROM implant",
		"total_tracking":   "SELECT count(*) FROM armada_tracking",
		"driver_telat": `SELECT count(DISTINCT r.id_driver)
			FROM ritase r
			JOIN ritase_event e ON e.id_ritase = r.id_ritase
			WHERE LOWER(r.status) NOT IN ('selesai','completed','done','batal','cancelled')
			  AND e.created_at < now() - interval '6 hours'`,
	}

	fmt.Println("=== SUMMARY QUERIES ===")
	for name, q := range sumQueries {
		var out int64
		var err error
		if containsParam(q) {
			err = db.QueryRow(ctx, q, today).Scan(&out)
		} else {
			err = db.QueryRow(ctx, q).Scan(&out)
		}
		if err != nil {
			fmt.Printf("❌ %-20s ERR: %v\n", name, err)
		} else {
			fmt.Printf("✅ %-20s = %d\n", name, out)
		}
	}

	analisisQueries := map[string]string{
		"durasi_loading": `SELECT avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)))
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'selesai_loading'
			WHERE e1.status = 'mulai_loading' AND e2.created_at > e1.created_at`,
		"durasi_perjalanan": `SELECT avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)))
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'sampai_gudang'
			WHERE e1.status = 'berangkat_gudang' AND e2.created_at > e1.created_at`,
		"durasi_unloading": `SELECT avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)))
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'selesai_unloading'
			WHERE e1.status = 'mulai_unloading' AND e2.created_at > e1.created_at`,
		"total_ritase_dihitung": `SELECT count(DISTINCT id_ritase) FROM ritase_event
			WHERE status IN ('mulai_loading','berangkat_gudang','mulai_unloading')`,
		"bottleneck_seller": `SELECT 'seller', s.nama_seller, 'ritase terbanyak', count(DISTINCT rs.id_ritase)::float8
			FROM ritase_stop rs
			JOIN ritase r ON r.id_ritase = rs.id_ritase
			JOIN seller s ON s.id_seller = rs.id_seller
			WHERE rs.jenis_stop = 'seller' AND r.status NOT IN ('selesai','completed','done')
			GROUP BY s.nama_seller ORDER BY count(DISTINCT rs.id_ritase) DESC LIMIT 3`,
		"bottleneck_driver": `SELECT 'driver', d.nama_driver, 'paket tertinggal', COALESCE(sum(r.paket_tertinggal),0)::float8
			FROM ritase r JOIN driver d ON d.id_driver = r.id_driver
			GROUP BY d.nama_driver ORDER BY COALESCE(sum(r.paket_tertinggal),0) DESC LIMIT 3`,
		"alert_berhenti": `SELECT r.kode_ritase, now() - max(e.created_at)
			FROM ritase r
			JOIN ritase_event e ON e.id_ritase = r.id_ritase
			WHERE r.status NOT IN ('selesai','completed','done','batal','cancelled')
			GROUP BY r.kode_ritase
			HAVING now() - max(e.created_at) > interval '3 hours'
			ORDER BY now() - max(e.created_at) DESC LIMIT 5`,
		"alert_terlalu_lama": `SELECT r.kode_ritase
			FROM ritase r
			JOIN ritase_event e1 ON e1.id_ritase = r.id_ritase AND e1.status = 'berangkat_gudang'
			JOIN ritase_event e2 ON e2.id_ritase = r.id_ritase AND e2.status = 'sampai_gudang'
			WHERE r.status NOT IN ('selesai','completed','done','batal','cancelled')
			  AND EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)) > 8*3600
			LIMIT 5`,
	}

	fmt.Println("\n=== ANALISIS QUERIES ===")
	for name, q := range analisisQueries {
		rows, err := db.Query(ctx, q)
		if err != nil {
			fmt.Printf("❌ %-24s ERR: %v\n", name, err)
			continue
		}
		rows.Close()
		fmt.Printf("✅ %-24s OK\n", name)
	}
}

func containsParam(q string) bool {
	for i := 0; i+1 < len(q); i++ {
		if q[i] == '$' && q[i+1] >= '1' && q[i+1] <= '9' {
			return true
		}
	}
	return false
}
