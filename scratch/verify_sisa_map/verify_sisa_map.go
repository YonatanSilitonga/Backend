package main

// Verifikasi sementara (read-only): menjalankan query peta yang diubah
// (sisa_barang/sisa_koli + total_awb konfirmasi kapten) ke DB lokal.
// HAPUS folder ini setelah verifikasi.

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"backend/internal/armada"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func intVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func strVal(p *string) string {
	if p == nil {
		return "-"
	}
	return *p
}

func main() {
	// .env relatif terhadap file sumber (work dari direktori mana pun).
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

	repo := armada.NewRepository(db)

	// 1. Sellers
	sellers, err := repo.ListSellerLocations(ctx)
	if err != nil {
		log.Fatalf("ListSellerLocations GAGAL: %v", err)
	}
	fmt.Printf("SELLERS: %d baris OK\n", len(sellers))
	shown := 0
	for _, s := range sellers {
		jb, sb := intVal(s.JumlahBarang), intVal(s.SisaBarang)
		if jb > 0 || sb > 0 || strVal(s.StatusPickup) == "sudah_diambil" {
			fmt.Printf("  seller=%d %-22.22s jumlah=%d sisa=%d status=%s\n",
				s.IDSeller, s.NamaSeller, jb, sb, strVal(s.StatusPickup))
			shown++
			if sb > jb {
				log.Fatalf("INVARIANT RUSAK seller=%d: sisa(%d) > jumlah(%d)", s.IDSeller, sb, jb)
			}
		}
	}
	if shown == 0 {
		fmt.Println("  (tidak ada seller bermuatan hari ini)")
	}

	// 2. Implants
	implants, err := repo.ListImplantLocations(ctx)
	if err != nil {
		log.Fatalf("ListImplantLocations GAGAL: %v", err)
	}
	fmt.Printf("IMPLANTS: %d baris OK\n", len(implants))
	shown = 0
	for _, im := range implants {
		jb, sb := intVal(im.JumlahBarang), intVal(im.SisaBarang)
		if jb > 0 || sb > 0 || strVal(im.StatusPickup) == "sudah_diambil" {
			fmt.Printf("  implant=%d %-22.22s jumlah=%d sisa=%d status=%s\n",
				im.IDImplant, im.NamaImplant, jb, sb, strVal(im.StatusPickup))
			shown++
			if sb > jb {
				log.Fatalf("INVARIANT RUSAK implant=%d: sisa(%d) > jumlah(%d)", im.IDImplant, sb, jb)
			}
		}
	}
	if shown == 0 {
		fmt.Println("  (tidak ada implant bermuatan hari ini)")
	}

	// 3. Tracking (total_awb + cf join)
	vehicles, err := repo.ListLatestTracking(ctx, 15, 2, false)
	if err != nil {
		log.Fatalf("ListLatestTracking GAGAL: %v", err)
	}
	fmt.Printf("VEHICLES: %d baris OK\n", len(vehicles))
	for _, v := range vehicles {
		if intVal(v.TotalAWB) > 0 {
			fmt.Printf("  kend=%d %-12.12s driver=%-18.18s awb=%d ritase=%v\n",
				v.IDKendaraan, v.PlatNomor, v.NamaDriver, intVal(v.TotalAWB), v.IDRitase)
		}
	}

	fmt.Println("SEMUA QUERY OK")
}
