package tracking

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository mengakses tabel armada_tracking.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateTracking menyimpan posisi GPS terbaru.
func (r *Repository) CreateTracking(ctx context.Context, req CreateTrackingRequest) (*Tracking, error) {
	var t Tracking
	err := r.db.QueryRow(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
		RETURNING id_tracking, id_ritase, id_kendaraan, id_driver,
		          latitude, longitude, kecepatan, arah, status
	`, req.IDRitase, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude,
		req.Kecepatan, req.Arah, req.Status).Scan(
		&t.ID, &t.IDRitase, &t.IDKendaraan, &t.IDDriver,
		&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
