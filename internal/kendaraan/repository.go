package kendaraan

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository mengakses tabel kendaraan.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListKendaraan mengambil semua kendaraan.
func (r *Repository) ListKendaraan(ctx context.Context) ([]Kendaraan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, jenis_kendaraan, kapasitas_koli, COALESCE(status_kendaraan, '')
		FROM kendaraan
		ORDER BY id_kendaraan ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Kendaraan
	for rows.Next() {
		var item Kendaraan
		if err := rows.Scan(&item.ID, &item.PlatNomor, &item.JenisKendaraan, &item.KapasitasKoli, &item.StatusKendaraan); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
