package main

// Skrip sementara READ-ONLY: melacak asal selisih 476 AWB di SKI dan
// memverifikasi teori mismatch tanggal. HAPUS folder ini setelah selesai.

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_, src, _, _ := runtime.Caller(0)
	godotenv.Load(filepath.Join(filepath.Dir(src), "..", "..", ".env"))
	dbURL := os.Getenv("DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Koneksi gagal: %v", err)
	}
	defer db.Close()

	loc, _ := time.LoadLocation("Asia/Jakarta")
	fmt.Println("== WIB sekarang:", time.Now().In(loc).Format("2006-01-02 15:04:05"))

	// 0. Implant bernama SKI
	fmt.Println("\n== IMPLANT ~ski~:")
	rows, err := db.Query(ctx, `SELECT id_implant, nama_implant, status FROM implant WHERE LOWER(nama_implant) LIKE '%ski%'`)
	if err != nil {
		log.Fatal(err)
	}
	var skiIDs []int64
	for rows.Next() {
		var id int64
		var nama, status string
		_ = rows.Scan(&id, &nama, &status)
		fmt.Printf("  id=%d nama=%s status=%s\n", id, nama, status)
		skiIDs = append(skiIDs, id)
	}
	rows.Close()

	// 1. input_kapten hari ini (WIB) untuk implant SKI
	fmt.Println("\n== INPUT_KAPTEN hari ini (WIB) untuk implant SKI:")
	rows, err = db.Query(ctx, `
		SELECT id, id_implant, id_seller, jenis_ritase, ritase_ke, jumlah_awb, id_ritase, created_at
		FROM input_kapten
		WHERE (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		  AND (id_implant = ANY($1) OR id_seller IN (SELECT id_seller FROM implant WHERE id_implant = ANY($1)))
		ORDER BY id`, skiIDs)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var id, idImpl, ritKe, awb int64
		var idSeller, idRitase *int64
		var jenis string
		var created time.Time
		_ = rows.Scan(&id, &idImpl, &idSeller, &jenis, &ritKe, &awb, &idRitase, &created)
		fmt.Printf("  id=%d implant=%d seller=%v jenis=%s rit=%d awb=%d id_ritase=%v at=%s\n",
			id, idImpl, idSeller, jenis, ritKe, awb, idRitase, created.Format("15:04"))
	}
	rows.Close()

	// 2. konfirmasi_penjemputan hari ini (WIB) untuk implant SKI
	fmt.Println("\n== KONFIRMASI hari ini (WIB) untuk implant SKI:")
	rows, err = db.Query(ctx, `
		SELECT id, id_implant, id_seller, id_ritase, jenis_ritase, ritase_ke, jumlah_awb, created_at
		FROM konfirmasi_penjemputan
		WHERE (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		  AND (id_implant = ANY($1) OR id_seller IN (SELECT id_seller FROM implant WHERE id_implant = ANY($1)))
		ORDER BY id`, skiIDs)
	if err != nil {
		log.Fatal(err)
	}
	n := 0
	for rows.Next() {
		var id, idImpl, ritKe, awb int64
		var idSeller, idRitase *int64
		var jenis string
		var created time.Time
		_ = rows.Scan(&id, &idImpl, &idSeller, &idRitase, &jenis, &ritKe, &awb, &created)
		fmt.Printf("  id=%d implant=%d seller=%v ritase=%v jenis=%s rit=%d awb=%d at=%s\n",
			id, idImpl, idSeller, idRitase, jenis, ritKe, awb, created.Format("15:04"))
		n++
	}
	if n == 0 {
		fmt.Println("  (tidak ada konfirmasi hari ini)")
	}
	rows.Close()

	// 3. driver_pickup_log yang match fuzzy '%ski%'
	fmt.Println("\n== DRIVER_PICKUP_LOG match '%ski%' hari ini:")
	rows, err = db.Query(ctx, `
		SELECT id_log, id_user, nama_driver, jumlah_barang, koli, asal_seller, status, tanggal
		FROM driver_pickup_log
		WHERE LOWER(asal_seller) LIKE '%ski%'
		  AND (tanggal = CURRENT_DATE OR tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
		ORDER BY id_log`)
	if err != nil {
		log.Fatal(err)
	}
	n = 0
	for rows.Next() {
		var idLog, idUser, jb, koli int64
		var nama, asal, status, tanggal string
		_ = rows.Scan(&idLog, &idUser, &nama, &jb, &koli, &asal, &status, &tanggal)
		fmt.Printf("  log=%d user=%d nama=%s awb=%d koli=%d asal=%q status=%s tgl=%s\n",
			idLog, idUser, nama, jb, koli, asal, status, tanggal)
		n++
	}
	if n == 0 {
		fmt.Println("  (tidak ada)")
	}
	rows.Close()

	// 4. implan_barang_log hari ini untuk SKI
	fmt.Println("\n== IMPLAN_BARANG_LOG hari ini untuk SKI:")
	rows, err = db.Query(ctx, `
		SELECT id_log, id_implant, jumlah_barang, status, tanggal
		FROM implan_barang_log
		WHERE id_implant = ANY($1)
		  AND (tanggal = CURRENT_DATE OR tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
		ORDER BY id_log`, skiIDs)
	if err != nil {
		log.Fatal(err)
	}
	n = 0
	for rows.Next() {
		var idLog, idImpl, jb int64
		var status, tanggal string
		_ = rows.Scan(&idLog, &idImpl, &jb, &status, &tanggal)
		fmt.Printf("  log=%d implant=%d awb=%d status=%s tgl=%s\n", idLog, idImpl, jb, status, tanggal)
		n++
	}
	if n == 0 {
		fmt.Println("  (tidak ada)")
	}
	rows.Close()

	// 5. Ritase 13 & 14 Okt 2026 dengan driver
	fmt.Println("\n== RITASE 13-14 Okt 2026:")
	rows, err = db.Query(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.tanggal, r.ritase_ke, r.jenis_ritase, r.status,
		       r.id_driver, d.nama_driver
		FROM ritase r
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		WHERE r.tanggal IN ('2026-10-13', '2026-10-14')
		ORDER BY r.tanggal, r.ritase_ke`)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var id int64
		var kode, tgl, jenis, status string
		var ritKe int
		var idDrv *int64
		var namaDrv *string
		_ = rows.Scan(&id, &kode, &tgl, &ritKe, &jenis, &status, &idDrv, &namaDrv)
		fmt.Printf("  id=%d kode=%s tgl=%s rit=%d jenis=%s status=%s driver=%v %v\n",
			id, kode, tgl, ritKe, jenis, status, idDrv, namaDrv)
	}
	rows.Close()

	fmt.Println("\nSELESAI")
}
