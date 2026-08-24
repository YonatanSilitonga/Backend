package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Try multiple paths for .env
	loaded := false
	for _, p := range []string{"../../.env", "../../../.env", "../.env", ".env"} {
		if err := godotenv.Load(p); err == nil {
			fmt.Println("Loaded .env from", p)
			loaded = true
			break
		}
	}
	if !loaded {
		// Try loading from Backend root using absolute-ish path
		cwd, _ := os.Getwd()
		fmt.Println("CWD:", cwd)
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL kosong")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc).Format("2006-01-02")
	fmt.Println("=== DIAGNOSTIK MUATAN HARI INI ===")
	fmt.Println("Tanggal:", today)
	fmt.Println()

	// 1. Cek semua event hari ini
	fmt.Println("--- 1. Semua ritase_event hari ini ---")
	rows, err := db.Query(ctx, `
		SELECT id_ritase, status, 
		       COALESCE(jumlah_koli,0), COALESCE(jumlah_ecer,0), COALESCE(jumlah_high_value,0),
		       created_at
		FROM ritase_event
		WHERE created_at::date = $1
		ORDER BY id_ritase, created_at
	`, today)
	if err != nil {
		log.Fatal("query event:", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		var status string
		var koli, ecer, hv int
		var createdAt time.Time
		if err := rows.Scan(&id, &status, &koli, &ecer, &hv, &createdAt); err != nil {
			log.Fatal("scan:", err)
		}
		fmt.Printf("  ritase=%d | %-25s | koli=%d ecer=%d hv=%d | %s\n", id, status, koli, ecer, hv, createdAt.Format("15:04:05"))
		count++
	}
	if count == 0 {
		fmt.Println("  [KOSONG] Tidak ada event hari ini!")
	}
	fmt.Println()

	// 2. Cek SUM dari Bongkar Muat Barang
	fmt.Println("--- 2. SUM dari Bongkar Muat Barang per ritase ---")
	rows2, err := db.Query(ctx, `
		SELECT ev.id_ritase,
		       sum(ev.jumlah_koli) AS koli, sum(ev.jumlah_high_value) AS hv, sum(ev.jumlah_ecer) AS ecer
		FROM ritase_event ev
		WHERE ev.status = 'Bongkar Muat Barang' AND ev.created_at::date = $1
		GROUP BY ev.id_ritase
	`, today)
	if err != nil {
		log.Fatal("query distinct:", err)
	}
	defer rows2.Close()
	totalKoli, totalHv, totalEcer := 0, 0, 0
	for rows2.Next() {
		var id int64
		var koli, ecer, hv int
		if err := rows2.Scan(&id, &koli, &hv, &ecer); err != nil {
			log.Fatal("scan:", err)
		}
		fmt.Printf("  ritase=%d | MAX koli=%d ecer=%d hv=%d\n", id, koli, ecer, hv)
		totalKoli += koli
		totalEcer += ecer
		totalHv += hv
	}
	fmt.Printf("  TOTAL → koli=%d hv=%d ecer=%d\n", totalKoli, totalHv, totalEcer)
	fmt.Println()

	// 3. Cek summary query yang dipakai dashboard
	fmt.Println("--- 3. Dashboard Summary Query (SUM Bongkar Muat Barang) ---")
	var sumKoli, sumHv, sumEcer int
	err = db.QueryRow(ctx, `
		SELECT COALESCE(sum(koli),0), COALESCE(sum(hv),0), COALESCE(sum(ecer),0)
		FROM (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli, sum(ev.jumlah_high_value) AS hv, sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.created_at::date = $1 AND ev.status = 'Bongkar Muat Barang'
			GROUP BY ev.id_ritase
		) latest
	`, today).Scan(&sumKoli, &sumHv, &sumEcer)
	if err != nil {
		log.Fatal("query summary:", err)
	}
	fmt.Printf("  Dashboard would show → koli=%d hv=%d ecer=%d\n", sumKoli, sumHv, sumEcer)
	fmt.Println()

	// 4. Cek armada_tracking juga
	fmt.Println("--- 4. armada_tracking muatan hari ini ---")
	rows3, err := db.Query(ctx, `
		SELECT id_kendaraan, id_ritase,
		       COALESCE(jumlah_koli,0), COALESCE(jumlah_ecer,0), COALESCE(jumlah_high_value,0),
		       last_update
		FROM armada_tracking
		WHERE last_update::date = $1
		ORDER BY last_update DESC
	`, today)
	if err != nil {
		log.Fatal("query tracking:", err)
	}
	defer rows3.Close()
	count3 := 0
	for rows3.Next() {
		var idK, idR int64
		var koli, ecer, hv int
		var lastUp time.Time
		if err := rows3.Scan(&idK, &idR, &koli, &ecer, &hv, &lastUp); err != nil {
			log.Fatal("scan:", err)
		}
		fmt.Printf("  kendaraan=%d ritase=%d | koli=%d ecer=%d hv=%d | %s\n", idK, idR, koli, ecer, hv, lastUp.Format("15:04:05"))
		count3++
	}
	if count3 == 0 {
		fmt.Println("  [KOSONG] Tidak ada tracking update hari ini!")
	}
	fmt.Println()

	// 5. Cek semua unique status di ritase_event
	fmt.Println("--- 5. Semua unique status di ritase_event ---")
	rows5, err := db.Query(ctx, `
		SELECT status, count(*) FROM ritase_event
		GROUP BY status ORDER BY count(*) DESC
	`)
	if err != nil {
		log.Fatal("query status:", err)
	}
	defer rows5.Close()
	for rows5.Next() {
		var status string
		var cnt int
		if err := rows5.Scan(&status, &cnt); err != nil {
			log.Fatal("scan:", err)
		}
		fmt.Printf("  %-30s → %d events\n", status, cnt)
	}
	fmt.Println()

	// 6. Cek event loading/perjalanan per ritase
	fmt.Println("--- 6. Cek pasangan event durasi (30 hari terakhir) ---")
	rows6, err := db.Query(ctx, `
		SELECT e1.id_ritase, e1.status AS start_status, e2.status AS end_status,
		       EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)) AS dur_detik
		FROM ritase_event e1
		JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.created_at > e1.created_at
		WHERE e1.status IN ('Bongkar Muat Barang','Keluar Gudang','mulai_unloading')
		  AND e2.status IN ('selesai_loading','tiba','selesai_unloading')
		ORDER BY e1.id_ritase, e1.created_at
		LIMIT 20
	`)
	if err != nil {
		log.Fatal("query pairs:", err)
	}
	defer rows6.Close()
	count6 := 0
	for rows6.Next() {
		var idRitase int64
		var startStatus, endStatus string
		var dur float64
		if err := rows6.Scan(&idRitase, &startStatus, &endStatus, &dur); err != nil {
			log.Fatal("scan:", err)
		}
		fmt.Printf("  ritase=%d | %s → %s | %.0f detik\n", idRitase, startStatus, endStatus, dur)
		count6++
	}
	if count6 == 0 {
		fmt.Println("  [KOSONG] Tidak ada pasangan event durasi ditemukan!")
	}
	fmt.Println()
	fmt.Println("=== SELESAI ===")
}
