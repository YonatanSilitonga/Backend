package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/config"
	"backend/internal/database"
)

// SEED MVP — INSERT-ONLY & IDEMPOTENT. TIDAK ada TRUNCATE/DELETE.
// Aman dijalankan berulang: cuma nambah yang belum ada, gak nimpa data existing.

var db *pgxpool.Pool

type stopDef struct {
	Jenis     string
	Seller    string // nama seller (dicari), kosong jika bukan seller
	DropPoint string // nama drop_point, kosong jika bukan drop_point
	Keterangan string
}

func main() {
	ctx := context.Background()
	cfg := config.Load()
	var err error
	db, err = database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	// ── Driver TETAP: AWALUDIN (Awal) & Ridwan ──
	idAwal := ensureDriver(ctx, "AWALUDIN", "0812-9999-8888")
	idRidwan := ensureDriver(ctx, "RIDWAN CUCU", "0896-3030-7115")
	fmt.Printf("✓ driver: AWALUDIN id=%d, RIDWAN CUCU id=%d\n", idAwal, idRidwan)

	// ── Link users.id_driver (AWALUDIN + ridwan) ──
	linkUserDriver(ctx, "AWALUDIN", idAwal)
	ensureUser(ctx, "ridwan", "ridwan123", "driver", idRidwan)
	linkUserDriver(ctx, "ridwan", idRidwan)
	fmt.Printf("✓ users.id_driver ter-link (AWALUDIN → %d, ridwan → %d)\n", idAwal, idRidwan)

	// ── Kendaraan (cari by plat dari data real) ──
	idKendAwal := findKendaraanByPlat(ctx, "B 9806")
	idKendRidwan := findKendaraanByPlat(ctx, "B 9567")
	fmt.Printf("✓ kendaraan: Awal plat B 9806 → id=%d, Ridwan plat B 9567 → id=%d\n", idKendAwal, idKendRidwan)

	// ── GTW (gateway) = drop_point final ──
	idGTW := ensureDropPoint(ctx, "DP-GTW", "GTW JKT")
	fmt.Printf("✓ drop_point GTW JKT → id=%d\n", idGTW)

	// ── Seller (cari by nama dari data real) ──
	seller := func(nama string) int64 {
		id, err := findSellerByName(ctx, nama)
		if err != nil {
			log.Fatalf("seller '%s' tidak ditemukan: %v", nama, err)
		}
		return id
	}
	idSKI := seller("SKI")
	idTA := seller("TITIP AJA") // TA
	idSomething := seller("SOMETHING")
	fmt.Printf("✓ seller: SKI=%d TA=%d SOMETHING=%d\n", idSKI, idTA, idSomething)

	// ── Ritase contoh (INSERT-only, skip kalau kode sudah ada) ──
	createRitase(ctx, "RTS-AWL-01", today, idAwal, idKendAwal, 1, idSKI, idGTW, []stopDef{
		{Jenis: "gudang", Keterangan: "Gudang Outgoing"},
		{Jenis: "seller", Seller: "SKI"},
		{Jenis: "seller", Seller: "TITIP AJA"},
		{Jenis: "drop_point", DropPoint: "GTW JKT"},
	})
	createRitase(ctx, "RTS-RDW-01", today, idRidwan, idKendRidwan, 1, idSomething, idGTW, []stopDef{
		{Jenis: "gudang", Keterangan: "Gudang Outgoing"},
		{Jenis: "seller", Seller: "SOMETHING"},
		{Jenis: "drop_point", DropPoint: "GTW JKT"},
	})

	fmt.Println("\nSEED MVP DONE ✓ (insert-only, idempotent)")
}

/* ---------- helpers (semua INSERT-only / UPDATE presisi) ---------- */

func ensureDriver(ctx context.Context, nama, hp string) int64 {
	var id int64
	err := db.QueryRow(ctx, "SELECT id_driver FROM driver WHERE nama_driver = $1 LIMIT 1", nama).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = db.QueryRow(ctx, `
			INSERT INTO driver (nama_driver, no_hp, status_driver, jenis_driver)
			VALUES ($1,$2,'bertugas','tetap')
			RETURNING id_driver`, nama, hp).Scan(&id)
		if err != nil {
			log.Fatalf("gagal insert driver %s: %v", nama, err)
		}
	} else if err != nil {
		log.Fatalf("gagal cek driver %s: %v", nama, err)
	}
	return id
}

func linkUserDriver(ctx context.Context, username string, idDriver int64) {
	_, err := db.Exec(ctx, `
		UPDATE users SET id_driver = $1
		WHERE username = $2 AND (id_driver IS NULL OR id_driver = 0)`, idDriver, username)
	if err != nil {
		log.Fatalf("gagal link user %s -> driver %d: %v", username, idDriver, err)
	}
}

func ensureUser(ctx context.Context, username, password, role string, idDriver int64) {
	// Realign sequence id_user (aman, tidak mengubah data) — mencegah konflik PK
	if _, err := db.Exec(ctx, `SELECT setval('users_id_user_seq', GREATEST((SELECT COALESCE(MAX(id_user),1) FROM users), 1))`); err != nil {
		log.Fatalf("gagal realign users sequence: %v", err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	_, err := db.Exec(ctx, `
		INSERT INTO users (username, password, role)
		VALUES ($1,$2,$3)
		ON CONFLICT (username) DO NOTHING`, username, string(hash), role)
	if err != nil {
		log.Fatalf("gagal insert user %s: %v", username, err)
	}
}

func findKendaraanByPlat(ctx context.Context, platPrefix string) int64 {
	var id int64
	err := db.QueryRow(ctx, `
		SELECT id_kendaraan FROM kendaraan WHERE plat_nomor ILIKE $1 LIMIT 1`, platPrefix+"%").Scan(&id)
	if err != nil {
		log.Fatalf("kendaraan plat '%s' tidak ditemukan: %v", platPrefix, err)
	}
	return id
}

func ensureDropPoint(ctx context.Context, kode, nama string) int64 {
	var id int64
	err := db.QueryRow(ctx, "SELECT id_drop_point FROM drop_point WHERE nama_drop_point = $1 LIMIT 1", nama).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = db.QueryRow(ctx, `
			INSERT INTO drop_point (kode_dp, nama_drop_point, status)
			VALUES ($1,$2,'aktif')
			RETURNING id_drop_point`, kode, nama).Scan(&id)
		if err != nil {
			log.Fatalf("gagal insert drop_point %s: %v", nama, err)
		}
	} else if err != nil {
		log.Fatalf("gagal cek drop_point %s: %v", nama, err)
	}
	return id
}

func findSellerByName(ctx context.Context, nama string) (int64, error) {
	var id int64
	err := db.QueryRow(ctx, `
		SELECT id_seller FROM seller WHERE nama_seller ILIKE $1 LIMIT 1`, nama).Scan(&id)
	return id, err
}

func createRitase(ctx context.Context, kode, tanggal string, idDriver, idKendaraan int64,
	ritaseKe int, idSeller, idDropPoint int64, stops []stopDef) {

	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM ritase WHERE kode_ritase = $1", kode).Scan(&count); err != nil {
		log.Fatalf("cek ritase %s: %v", kode, err)
	}
	if count > 0 {
		fmt.Printf("· ritase %s sudah ada, skip (idempotent)\n", kode)
		return
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	var newID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'direncanakan')
		RETURNING id_ritase`, kode, tanggal, idDriver, idKendaraan, idSeller, idDropPoint, ritaseKe).Scan(&newID)
	if err != nil {
		log.Fatalf("gagal insert ritase %s: %v", kode, err)
	}

	for i, s := range stops {
		var idS, idD *int64
		if s.Jenis == "seller" {
			v := findSellerOrFatal(ctx, s.Seller)
			idS = &v
		}
		if s.Jenis == "drop_point" {
			v := ensureDropPoint(ctx, "DP-GTW", s.DropPoint)
			idD = &v
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, id_seller, id_drop_point, keterangan)
			VALUES ($1,$2,$3,$4,$5,$6)`, newID, i+1, s.Jenis, idS, idD, s.Keterangan); err != nil {
			log.Fatalf("gagal insert stop %d ritase %s: %v", i+1, kode, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit ritase %s: %v", kode, err)
	}
	fmt.Printf("✓ ritase %s (RIT %d) + %d stops dibuat\n", kode, ritaseKe, len(stops))
}

func findSellerOrFatal(ctx context.Context, nama string) int64 {
	id, err := findSellerByName(ctx, nama)
	if err != nil {
		log.Fatalf("seller '%s' tidak ditemukan: %v", nama, err)
	}
	return id
}
