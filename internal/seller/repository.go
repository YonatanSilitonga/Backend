package seller

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository mengakses tabel seller.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListSeller mengambil semua seller.
func (r *Repository) ListSeller(ctx context.Context) ([]Seller, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_seller,
		       COALESCE(kode_seller, ''),
		       COALESCE(nama_seller, ''),
		       COALESCE(alamat, ''),
		       COALESCE(kota, ''),
		       COALESCE(area, ''),
		       COALESCE(pic, ''),
		       COALESCE(no_hp, ''),
		       COALESCE(jam_mulai_pickup::text, ''),
		       COALESCE(jam_selesai_pickup::text, ''),
		       forecast_harian,
		       COALESCE(status, ''),
		       latitude,
		       longitude
		FROM seller
		ORDER BY id_seller ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Seller
	for rows.Next() {
		var item Seller
		if err := rows.Scan(
			&item.ID,
			&item.KodeSeller,
			&item.NamaSeller,
			&item.Alamat,
			&item.Kota,
			&item.Area,
			&item.Pic,
			&item.NoHP,
			&item.JamMulaiPickup,
			&item.JamSelesaiPickup,
			&item.ForecastHarian,
			&item.Status,
			&item.Latitude,
			&item.Longitude,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
