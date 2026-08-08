// fill_jarak: hitung JARAK TEMPUH (jalan, OSRM gratis) dari 2 gudang ke tiap seller:
//   - jarak_tempuh_km = dari GUDANG OUTGOING
//   - jarak_dc_km     = dari GUDANG DC (Buaran Indah / incoming)
// Simpan di kolom seller tsb. Fallback ke garis lurus (haversine) kalau OSRM gagal.
// HANYA menulis kolom jarak_tempuh_km & jarak_dc_km + sync koordinat 2 gudang.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"backend/internal/config"
	"backend/internal/database"
)

// Gudang Outgoing (koordinat asli) & Gudang DC / Buaran Indah.
const (
	OUT_LAT = -6.171496373990977
	OUT_LON = 106.65715503860062
	DC_LAT  = -6.1848
	DC_LON  = 106.6511
)

const osrmURL = "https://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=false"

type sellerRow struct {
	ID    int64
	Lat   float64
	Lon   float64
}

type osrmResp struct {
	Routes []struct {
		Distance float64 `json:"distance"` // meter
	} `json:"routes"`
}

// haversineKm — jarak garis lurus (km).
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * R * math.Asin(math.Sqrt(a))
}

// osrmKm — jarak jalan dari OSRM (meter -> km). 0 + error kalau gagal.
func osrmKm(lat1, lon1, lat2, lon2 float64) (float64, error) {
	url := fmt.Sprintf(osrmURL, lon1, lat1, lon2, lat2)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("OSRM status %d", resp.StatusCode)
	}
	var r osrmResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, err
	}
	if len(r.Routes) == 0 {
		return 0, fmt.Errorf("OSRM: no route")
	}
	return r.Routes[0].Distance / 1000.0, nil
}

// kmFrom — hitung dari titik (lat1,lon1) ke seller; fallback haversine.
func kmFrom(lat1, lon1, sLat, sLon float64) (km float64, src string) {
	km, err := osrmKm(lat1, lon1, sLat, sLon)
	if err != nil || km <= 0 {
		return haversineKm(lat1, lon1, sLat, sLon), "straight"
	}
	return km, "osrm"
}

func main() {
	ctx := context.Background()
	cfg := config.Load()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Sinkronkan koordinat ke tabel gudang (idempotent, cuma update lat/lng 2 baris itu).
	if _, err := db.Exec(ctx,
		`UPDATE gudang SET latitude=$1, longitude=$2 WHERE tipe='outgoing'`,
		OUT_LAT, OUT_LON); err != nil {
		log.Printf("⚠ sync gudang outgoing: %v", err)
	} else {
		fmt.Println("✓ koordinat OUTGOING tersinkron ke gudang")
	}
	if _, err := db.Exec(ctx,
		`UPDATE gudang SET latitude=$1, longitude=$2 WHERE tipe='incoming'`,
		DC_LAT, DC_LON); err != nil {
		log.Printf("⚠ sync gudang incoming (DC): %v", err)
	} else {
		fmt.Println("✓ koordinat DC tersinkron ke gudang")
	}

	rows, err := db.Query(ctx, `
		SELECT id_seller, latitude, longitude
		FROM seller
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		ORDER BY id_seller`)
	if err != nil {
		log.Fatal(err)
	}
	var sellers []sellerRow
	for rows.Next() {
		var s sellerRow
		if err := rows.Scan(&s.ID, &s.Lat, &s.Lon); err != nil {
			log.Fatal(err)
		}
		sellers = append(sellers, s)
	}
	rows.Close()

	for _, s := range sellers {
		kmOut, srcOut := kmFrom(OUT_LAT, OUT_LON, s.Lat, s.Lon)
		kmDc, srcDc := kmFrom(DC_LAT, DC_LON, s.Lat, s.Lon)
		// HANYA update 2 kolom jarak — tidak menyentuh data lain.
		if _, err := db.Exec(ctx,
			`UPDATE seller SET jarak_tempuh_km = $2, jarak_dc_km = $3 WHERE id_seller = $1`,
			s.ID, kmOut, kmDc); err != nil {
			log.Printf("⚠ update seller %d gagal: %v", s.ID, err)
			continue
		}
		fmt.Printf("seller %d | OUT=%5.2f km (%s) | DC=%5.2f km (%s)\n", s.ID, kmOut, srcOut, kmDc, srcDc)
	}

	// Drop point (gateway) — jarak dari 2 gudang, simpan di drop_point.
	drows, err := db.Query(ctx, `
		SELECT id_drop_point, latitude, longitude
		FROM drop_point
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		ORDER BY id_drop_point`)
	if err != nil {
		log.Fatal(err)
	}
	var drops []sellerRow
	for drows.Next() {
		var d sellerRow
		if err := drows.Scan(&d.ID, &d.Lat, &d.Lon); err != nil {
			log.Fatal(err)
		}
		drops = append(drops, d)
	}
	drows.Close()

	for _, d := range drops {
		kmOut, stOut := kmFrom(OUT_LAT, OUT_LON, d.Lat, d.Lon)
		kmDc, srcDc := kmFrom(DC_LAT, DC_LON, d.Lat, d.Lon)
		if _, err := db.Exec(ctx,
			`UPDATE drop_point SET jarak_tempuh_km = $2, jarak_dc_km = $3 WHERE id_drop_point = $1`,
			d.ID, kmOut, kmDc); err != nil {
			log.Printf("⚠ update drop_point %d gagal: %v", d.ID, err)
			continue
		}
		fmt.Printf("drop    %d | OUT=%5.2f km (%s) | DC=%5.2f km (%s)\n", d.ID, kmOut, stOut, kmDc, srcDc)
	}

	fmt.Println("SELESAI — hanya kolom jarak_tempuh_km & jarak_dc_km yang diubah.")
}