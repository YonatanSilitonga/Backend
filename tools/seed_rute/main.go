package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/config"
	"backend/internal/database"
)

// SEED RUTE — INSERT-ONLY & IDEMPOTENT. TIDAK menghapus data lain.
// Hanya menyentuh ritase dengan kode tertentu (RTS-AWL-01/02, RTS-RDW-01/02) + stops-nya.

var db *pgxpool.Pool

type stop struct {
	Jenis   string // gudang | seller | drop_point
	Gudang  string // nama gudang (outgoing/incoming)
	Seller  string // nama seller
	Drop    string // nama drop_point
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

	// ── Gudang ──
	idGudangOut := ensureGudang(ctx, "Gudang Outgoing", "outgoing")
	idGudangIn := ensureGudang(ctx, "Gudang Incoming", "incoming")
	fmt.Printf("✓ gudang: outgoing=%d incoming=%d\n", idGudangOut, idGudangIn)

	// ── Driver & kendaraan ──
	idAwal := driverID(ctx, "AWALUDIN")
	idRidwan := driverID(ctx, "RIDWAN CUCU")
	kenAwal := kendaraanID(ctx, "B 9806")
	kenRidwan := kendaraanID(ctx, "B 9567")
	fmt.Printf("✓ driver Awal=%d Ridwan=%d | kend Awal=%d Ridwan=%d\n", idAwal, idRidwan, kenAwal, kenRidwan)

	// ── GTW & seller ──
	gtw := dropPointID(ctx, "GTW JKT")
	ski := sellerID(ctx, "SKI")
	ta := sellerID(ctx, "TITIP AJA")
	something := sellerID(ctx, "SOMETHING")
	kacamata := sellerID(ctx, "PAYUTRUS KACAMATA")
	fmt.Printf("✓ GTW=%d seller: SKI=%d TA=%d SOMETHING=%d KACAMATA=%d\n", gtw, ski, ta, something, kacamata)

	// ── 4 RIT (stops sesuai tabel operasional) ──
	ensureRitase(ctx, "RTS-AWL-01", today, idAwal, kenAwal, 1, []stop{
		{Jenis: "gudang", Gudang: "Gudang Outgoing"},
		{Jenis: "seller", Seller: "SKI"},
		{Jenis: "seller", Seller: "TITIP AJA"},
		{Jenis: "drop_point", Drop: "GTW JKT"},
	})
	ensureRitase(ctx, "RTS-AWL-02", today, idAwal, kenAwal, 2, []stop{
		{Jenis: "drop_point", Drop: "GTW JKT"},
		{Jenis: "seller", Seller: "SKI"},
		{Jenis: "gudang", Gudang: "Gudang Outgoing"},
		{Jenis: "drop_point", Drop: "GTW JKT"},
	})
	ensureRitase(ctx, "RTS-RDW-01", today, idRidwan, kenRidwan, 1, []stop{
		{Jenis: "gudang", Gudang: "Gudang Outgoing"},
		{Jenis: "seller", Seller: "SOMETHING"},
		{Jenis: "gudang", Gudang: "Gudang Incoming"},
		{Jenis: "drop_point", Drop: "GTW JKT"},
	})
	ensureRitase(ctx, "RTS-RDW-02", today, idRidwan, kenRidwan, 2, []stop{
		{Jenis: "drop_point", Drop: "GTW JKT"},
		{Jenis: "seller", Seller: "SOMETHING"},
		{Jenis: "gudang", Gudang: "Gudang Incoming"},
		{Jenis: "seller", Seller: "PAYUTRUS KACAMATA"},
	})

	fmt.Println("\nSEED RUTE DONE ✓ (insert-only, idempotent)")
}

/* ---------- helpers ---------- */

func ensureGudang(ctx context.Context, nama, tipe string) int64 {
	var id int64
	err := db.QueryRow(ctx, "SELECT id_gudang FROM gudang WHERE nama_gudang = $1 LIMIT 1", nama).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = db.QueryRow(ctx, `
			INSERT INTO gudang (nama_gudang, tipe) VALUES ($1,$2) RETURNING id_gudang`, nama, tipe).Scan(&id)
		if err != nil {
			log.Fatalf("gagal insert gudang %s: %v", nama, err)
		}
	} else if err != nil {
		log.Fatalf("gagal cek gudang %s: %v", nama, err)
	}
	return id
}

func driverID(ctx context.Context, nama string) int64 {
	var id int64
	if err := db.QueryRow(ctx, "SELECT id_driver FROM driver WHERE nama_driver = $1 LIMIT 1", nama).Scan(&id); err != nil {
		log.Fatalf("driver %s tidak ditemukan: %v", nama, err)
	}
	return id
}

func kendaraanID(ctx context.Context, plat string) int64 {
	var id int64
	if err := db.QueryRow(ctx, "SELECT id_kendaraan FROM kendaraan WHERE plat_nomor ILIKE $1 LIMIT 1", plat+"%").Scan(&id); err != nil {
		log.Fatalf("kendaraan %s tidak ditemukan: %v", plat, err)
	}
	return id
}

func dropPointID(ctx context.Context, nama string) int64 {
	var id int64
	if err := db.QueryRow(ctx, "SELECT id_drop_point FROM drop_point WHERE nama_drop_point = $1 LIMIT 1", nama).Scan(&id); err != nil {
		log.Fatalf("drop_point %s tidak ditemukan: %v", nama, err)
	}
	return id
}

func sellerID(ctx context.Context, nama string) int64 {
	var id int64
	if err := db.QueryRow(ctx, "SELECT id_seller FROM seller WHERE nama_seller ILIKE $1 LIMIT 1", nama).Scan(&id); err != nil {
		log.Fatalf("seller %s tidak ditemukan: %v", nama, err)
	}
	return id
}

// ensureRitase: buat ritase (kalau belum ada) + ganti stops-nya dengan rute yang benar.
// Hanya menyentuh ritase dengan kode ini — data lain tidak diutak-atik.
func ensureRitase(ctx context.Context, kode, tanggal string, idDriver, idKendaraan int64, ritaseKe int, stops []stop) {
	var idRitase int64
	err := db.QueryRow(ctx, "SELECT id_ritase FROM ritase WHERE kode_ritase = $1 LIMIT 1", kode).Scan(&idRitase)
	if errors.Is(err, pgx.ErrNoRows) {
		// kolom id_seller & id_drop_point di ritase NOT NULL → isi dari rute
		var firstSeller, lastDrop int64
		for _, s := range stops {
			if s.Jenis == "seller" && firstSeller == 0 {
				firstSeller = sellerID(ctx, s.Seller)
			}
			if s.Jenis == "drop_point" {
				lastDrop = dropPointID(ctx, s.Drop)
			}
		}
		// buat baru
		err = db.QueryRow(ctx, `
			INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'direncanakan')
			RETURNING id_ritase`, kode, tanggal, idDriver, idKendaraan, firstSeller, lastDrop, ritaseKe).Scan(&idRitase)
		if err != nil {
			log.Fatalf("gagal insert ritase %s: %v", kode, err)
		}
		fmt.Printf("✓ ritase %s dibuat (RIT %d)\n", kode, ritaseKe)
	} else if err != nil {
		log.Fatalf("gagal cek ritase %s: %v", kode, err)
	} else {
		fmt.Printf("· ritase %s sudah ada, perbarui stops-nya\n", kode)
	}

	// ganti stops dengan rute yang benar (scoped hanya untuk ritase ini)
	if _, err := db.Exec(ctx, "DELETE FROM ritase_stop WHERE id_ritase = $1", idRitase); err != nil {
		log.Fatalf("gagal bersihkan stops %s: %v", kode, err)
	}
	for i, s := range stops {
		var idG, idS, idD *int64
		switch s.Jenis {
		case "gudang":
			v := ensureGudang(ctx, s.Gudang, map[string]string{"Gudang Outgoing": "outgoing", "Gudang Incoming": "incoming"}[s.Gudang])
			idG = &v
		case "seller":
			v := sellerID(ctx, s.Seller)
			idS = &v
		case "drop_point":
			v := dropPointID(ctx, s.Drop)
			idD = &v
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, id_gudang, id_seller, id_drop_point)
			VALUES ($1,$2,$3,$4,$5,$6)`, idRitase, i+1, s.Jenis, idG, idS, idD); err != nil {
			log.Fatalf("gagal insert stop %d ritase %s: %v", i+1, kode, err)
		}
	}
	fmt.Printf("  → %d stops (rute: %s)\n", len(stops), routeSummary(stops))
}

func routeSummary(stops []stop) string {
	out := ""
	for i, s := range stops {
		if i > 0 {
			out += " → "
		}
		switch s.Jenis {
		case "gudang":
			out += s.Gudang
		case "seller":
			out += s.Seller
		case "drop_point":
			out += s.Drop
		}
	}
	return out
}
