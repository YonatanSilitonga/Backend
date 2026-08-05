package armada

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("data tidak ditemukan")
)

// Repository mengakses tabel-tabel armada.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

/* ---------- Kendaraan ---------- */

func (r *Repository) ListKendaraan(ctx context.Context) ([]Kendaraan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, jenis_kendaraan,
		       kapasitas_koli, status_kendaraan
		FROM kendaraan
		ORDER BY id_kendaraan
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Kendaraan
	for rows.Next() {
		var k Kendaraan
		if err := rows.Scan(&k.ID, &k.PlatNomor, &k.JenisKendaraan,
			&k.KapasitasKoli, &k.StatusKendaraan); err != nil {
			return nil, err
		}
		items = append(items, k)
	}
	return items, rows.Err()
}

/* ---------- Driver ---------- */

func (r *Repository) ListDriver(ctx context.Context) ([]Driver, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_driver, nama_driver, no_hp, no_sim, jenis_sim, status_driver
		FROM driver
		ORDER BY id_driver
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Driver
	for rows.Next() {
		var d Driver
		if err := rows.Scan(&d.ID, &d.NamaDriver, &d.NoHP, &d.NoSIM, &d.JenisSIM, &d.StatusDriver); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

/* ---------- Ritase ---------- */

const ritaseSelect = `
	SELECT r.id_ritase, r.kode_ritase, r.tanggal::text,
	       r.id_driver, COALESCE(d.nama_driver,''),
	       r.id_kendaraan, COALESCE(k.plat_nomor,''),
	       r.id_seller, COALESCE(s.nama_seller,''),
	       r.id_drop_point, COALESCE(dp.nama_drop_point,''),
	       r.ritase_ke, r.total_awb, r.total_koli,
	       r.paket_tertinggal, r.alasan_tertinggal,
	       r.jam_berangkat::text, r.jam_tiba::text, r.status, r.created_at
	FROM ritase r
	LEFT JOIN driver d ON d.id_driver = r.id_driver
	LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
	LEFT JOIN seller s ON s.id_seller = r.id_seller
	LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
`

func scanRitase(row pgx.Row) (*Ritase, error) {
	var r Ritase
	err := row.Scan(&r.ID, &r.KodeRitase, &r.Tanggal,
		&r.IDDriver, &r.NamaDriver,
		&r.IDKendaraan, &r.PlatNomor,
		&r.IDSeller, &r.NamaSeller,
		&r.IDDropPoint, &r.NamaDropPoint,
		&r.RitaseKe, &r.TotalAWB, &r.TotalKoli,
		&r.PaketTertinggal, &r.AlasanTertinggal,
		&r.JamBerangkat, &r.JamTiba, &r.Status, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRitase mengambil semua penugasan, opsional filter driver/tanggal.
func (r *Repository) ListRitase(ctx context.Context, idDriver int64, tanggal string) ([]Ritase, error) {
	query := ritaseSelect + `
		WHERE 1=1
	`
	var args []interface{}
	if idDriver > 0 {
		args = append(args, idDriver)
		query += " AND r.id_driver = $" + fmt.Sprint(len(args))
	}
	if tanggal != "" {
		args = append(args, tanggal)
		query += " AND r.tanggal = $" + fmt.Sprint(len(args))
	}
	query += " ORDER BY r.tanggal DESC, r.ritase_ke, r.id_ritase DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Ritase
	for rows.Next() {
		rit, err := scanRitase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *rit)
	}
	return items, rows.Err()
}

// GetRitase mengambil satu ritase + event timeline-nya.
func (r *Repository) GetRitase(ctx context.Context, id int64) (*RitaseDetail, error) {
	rit, err := scanRitase(r.db.QueryRow(ctx, ritaseSelect+" WHERE r.id_ritase = $1", id))
	if err != nil {
		return nil, err
	}

	events, err := r.ListEvents(ctx, id)
	if err != nil {
		return nil, err
	}

	return &RitaseDetail{Ritase: *rit, Events: events}, nil
}

// CreateRitase membuat penugasan baru.
func (r *Repository) CreateRitase(ctx context.Context, req CreateRitaseRequest) (*Ritase, error) {
	var newID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'direncanakan')
		RETURNING id_ritase
	`, req.KodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, req.IDSeller, req.IDDropPoint,
		req.RitaseKe, req.TotalAWB, req.TotalKoli).Scan(&newID)
	if err != nil {
		return nil, err
	}
	detail, err := r.GetRitase(ctx, newID)
	if err != nil {
		return nil, err
	}
	return &detail.Ritase, nil
}

// AddEvent mencatat status baru di timeline & update status ritase.
func (r *Repository) AddEvent(ctx context.Context, idRitase int64, req UpdateStatusRequest) (*RitaseEvent, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var ev RitaseEvent
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase_event (id_ritase, status, catatan, latitude, longitude, durasi_detik)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id_event, id_ritase, status, catatan, latitude, longitude, durasi_detik, created_at
	`, idRitase, req.Status, req.Catatan, req.Latitude, req.Longitude, req.DurasiDetik).Scan(
		&ev.ID, &ev.IDRitase, &ev.Status, &ev.Catatan, &ev.Latitude, &ev.Longitude, &ev.DurasiDetik, &ev.CreatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, "UPDATE ritase SET status = $1 WHERE id_ritase = $2", req.Status, idRitase); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ev, nil
}

// UpdateMuatan memperbarui data muatan ritase.
func (r *Repository) UpdateMuatan(ctx context.Context, idRitase int64, req UpdateMuatanRequest) (*Ritase, error) {
	if _, err := r.db.Exec(ctx, `
		UPDATE ritase
		SET total_awb = COALESCE($1, total_awb),
		    total_koli = COALESCE($2, total_koli),
		    paket_tertinggal = COALESCE($3, paket_tertinggal),
		    alasan_tertinggal = COALESCE($4, alasan_tertinggal)
		WHERE id_ritase = $5
	`, req.TotalAWB, req.TotalKoli, req.PaketTertinggal, req.AlasanTertinggal, idRitase); err != nil {
		return nil, err
	}
	detail, err := r.GetRitase(ctx, idRitase)
	if err != nil {
		return nil, err
	}
	return &detail.Ritase, nil
}

// ListEvents mengambil timeline status sebuah ritase.
func (r *Repository) ListEvents(ctx context.Context, idRitase int64) ([]RitaseEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_event, id_ritase, status, catatan, latitude, longitude, created_at
		FROM ritase_event
		WHERE id_ritase = $1
		ORDER BY created_at, id_event
	`, idRitase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RitaseEvent
	for rows.Next() {
		var e RitaseEvent
		if err := rows.Scan(&e.ID, &e.IDRitase, &e.Status, &e.Catatan, &e.Latitude, &e.Longitude, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

/* ---------- Tracking ---------- */

func (r *Repository) ListTracking(ctx context.Context, limit int) ([]Tracking, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id_tracking, id_ritase, id_kendaraan, id_driver,
		       latitude, longitude, kecepatan, arah, status, last_update
		FROM armada_tracking
		ORDER BY last_update DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Tracking
	for rows.Next() {
		var t Tracking
		if err := rows.Scan(&t.ID, &t.IDRitase, &t.IDKendaraan, &t.IDDriver,
			&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status, &t.LastUpdate); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// CreateTracking menyimpan posisi GPS terbaru.
func (r *Repository) CreateTracking(ctx context.Context, req CreateTrackingRequest) (*Tracking, error) {
	var t Tracking
	err := r.db.QueryRow(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
		RETURNING id_tracking, id_ritase, id_kendaraan, id_driver,
		          latitude, longitude, kecepatan, arah, status, last_update
	`, req.IDRitase, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude,
		req.Kecepatan, req.Arah, req.Status).Scan(
		&t.ID, &t.IDRitase, &t.IDKendaraan, &t.IDDriver,
		&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status, &t.LastUpdate)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
