package driver

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository mengakses tabel driver.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListDriver mengambil semua driver.
func (r *Repository) ListDriver(ctx context.Context) ([]Driver, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_driver, COALESCE(nama_driver, ''), no_hp, COALESCE(status_driver, '')
		FROM driver
		ORDER BY id_driver ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Driver
	for rows.Next() {
		var item Driver
		if err := rows.Scan(&item.ID, &item.NamaDriver, &item.NoHP, &item.StatusDriver); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
