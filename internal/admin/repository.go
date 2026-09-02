package admin

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ──────── Driver ────────

func (r *Repository) ListDriver(ctx context.Context) ([]Driver, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_driver, nama_driver, no_hp, no_sim, jenis_sim, status_driver,
		       created_at, created_by, updated_at, updated_by
		FROM driver ORDER BY id_driver`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Driver
	for rows.Next() {
		var d Driver
		if err := rows.Scan(&d.ID, &d.NamaDriver, &d.NoHP, &d.NoSIM, &d.JenisSIM, &d.StatusDriver,
			&d.CreatedAt, &d.CreatedBy, &d.UpdatedAt, &d.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r *Repository) CreateDriver(ctx context.Context, req DriverRequest, createdBy int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO driver (nama_driver, no_hp, no_sim, jenis_sim, status_driver, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id_driver`,
		req.NamaDriver, req.NoHP, req.NoSIM, req.JenisSIM, req.StatusDriver, createdBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateDriver(ctx context.Context, id int64, req DriverRequest, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE driver SET nama_driver=$1, no_hp=$2, no_sim=$3, jenis_sim=$4, status_driver=$5,
		                  updated_at=NOW(), updated_by=$6
		WHERE id_driver=$7`,
		req.NamaDriver, req.NoHP, req.NoSIM, req.JenisSIM, req.StatusDriver, updatedBy, id,
	)
	return err
}

func (r *Repository) DeleteDriver(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE driver SET status_driver='nonaktif' WHERE id_driver=$1`, id)
	return err
}

// ──────── Kendaraan ────────

func (r *Repository) ListKendaraan(ctx context.Context) ([]Kendaraan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, jenis_kendaraan, kapasitas_kg, status_kendaraan,
		       created_at, created_by, updated_at, updated_by
		FROM kendaraan ORDER BY id_kendaraan`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Kendaraan
	for rows.Next() {
		var k Kendaraan
		if err := rows.Scan(&k.ID, &k.PlatNomor, &k.JenisKendaraan, &k.KapasitasKg, &k.StatusKendaraan,
			&k.CreatedAt, &k.CreatedBy, &k.UpdatedAt, &k.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, k)
	}
	return items, rows.Err()
}

func (r *Repository) CreateKendaraan(ctx context.Context, req KendaraanRequest, createdBy int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO kendaraan (plat_nomor, jenis_kendaraan, kapasitas_kg, status_kendaraan, created_by)
		VALUES ($1, $2, $3, $4, $5) RETURNING id_kendaraan`,
		req.PlatNomor, req.JenisKendaraan, req.KapasitasKg, req.StatusKendaraan, createdBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateKendaraan(ctx context.Context, id int64, req KendaraanRequest, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE kendaraan SET plat_nomor=$1, jenis_kendaraan=$2, kapasitas_kg=$3, status_kendaraan=$4,
		                     updated_at=NOW(), updated_by=$5
		WHERE id_kendaraan=$6`,
		req.PlatNomor, req.JenisKendaraan, req.KapasitasKg, req.StatusKendaraan, updatedBy, id,
	)
	return err
}

func (r *Repository) DeleteKendaraan(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE kendaraan SET status_kendaraan='nonaktif' WHERE id_kendaraan=$1`, id)
	return err
}

// ──────── Seller ────────

func (r *Repository) ListSeller(ctx context.Context) ([]Seller, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_seller, kode_seller, nama_seller, alamat, kota, area, pic, no_hp,
		       forecast_harian, status, latitude, longitude,
		       created_at, created_by, updated_at, updated_by
		FROM seller ORDER BY id_seller`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Seller
	for rows.Next() {
		var s Seller
		if err := rows.Scan(&s.ID, &s.KodeSeller, &s.NamaSeller, &s.Alamat, &s.Kota,
			&s.Area, &s.Pic, &s.NoHP, &s.ForecastHarian, &s.Status, &s.Latitude, &s.Longitude,
			&s.CreatedAt, &s.CreatedBy, &s.UpdatedAt, &s.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) CreateSeller(ctx context.Context, req SellerRequest, createdBy int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO seller (kode_seller, nama_seller, alamat, kota, area, pic, no_hp, forecast_harian, status, latitude, longitude, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id_seller`,
		req.KodeSeller, req.NamaSeller, req.Alamat, req.Kota, req.Area, req.Pic,
		req.NoHP, req.ForecastHarian, req.Status, req.Latitude, req.Longitude, createdBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateSeller(ctx context.Context, id int64, req SellerRequest, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE seller SET kode_seller=$1, nama_seller=$2, alamat=$3, kota=$4, area=$5,
		       pic=$6, no_hp=$7, forecast_harian=$8, status=$9, latitude=$10, longitude=$11,
		       updated_at=NOW(), updated_by=$12
		WHERE id_seller=$13`,
		req.KodeSeller, req.NamaSeller, req.Alamat, req.Kota, req.Area, req.Pic,
		req.NoHP, req.ForecastHarian, req.Status, req.Latitude, req.Longitude, updatedBy, id,
	)
	return err
}

func (r *Repository) DeleteSeller(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE seller SET status='nonaktif' WHERE id_seller=$1`, id)
	return err
}

// ──────── Gudang ────────

func (r *Repository) ListGudang(ctx context.Context) ([]Gudang, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_gudang, nama_gudang, alamat, kota, latitude, longitude, status,
		       created_at, created_by, updated_at, updated_by
		FROM gudang ORDER BY id_gudang`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Gudang
	for rows.Next() {
		var g Gudang
		if err := rows.Scan(&g.ID, &g.NamaGudang, &g.Alamat, &g.Kota, &g.Latitude, &g.Longitude, &g.Status,
			&g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

func (r *Repository) CreateGudang(ctx context.Context, req GudangRequest, createdBy int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO gudang (nama_gudang, alamat, kota, latitude, longitude, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id_gudang`,
		req.NamaGudang, req.Alamat, req.Kota, req.Latitude, req.Longitude, req.Status, createdBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateGudang(ctx context.Context, id int64, req GudangRequest, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE gudang SET nama_gudang=$1, alamat=$2, kota=$3, latitude=$4, longitude=$5, status=$6,
		                  updated_at=NOW(), updated_by=$7
		WHERE id_gudang=$8`,
		req.NamaGudang, req.Alamat, req.Kota, req.Latitude, req.Longitude, req.Status, updatedBy, id,
	)
	return err
}

func (r *Repository) DeleteGudang(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE gudang SET status='nonaktif' WHERE id_gudang=$1`, id)
	return err
}

// ──────── User ────────

func (r *Repository) ListUser(ctx context.Context) ([]User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id_user, u.username, COALESCE(k.nama, '') AS name, u.role, u.karyawan_id,
		       CASE WHEN u.last_login IS NOT NULL AND u.last_login > now() - interval '30 minutes' THEN true ELSE false END AS is_active,
		       COALESCE(u.status, 'aktif') AS status,
		       u.created_at, u.created_by, u.updated_at, u.updated_by
		FROM users u
		LEFT JOIN karyawan k ON k.id_karyawan = u.karyawan_id
		ORDER BY u.id_user`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.KaryawanID, &u.IsActive, &u.Status,
			&u.CreatedAt, &u.CreatedBy, &u.UpdatedAt, &u.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, u)
	}
	return items, rows.Err()
}

func (r *Repository) CreateUser(ctx context.Context, req UserRequest, passwordHash string, createdBy int64) (int64, error) {
	status := req.Status
	if status == "" {
		status = "aktif"
	}
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (username, password, role, karyawan_id, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id_user`,
		req.Username, passwordHash, req.Role, req.KaryawanID, status, createdBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateUserRole(ctx context.Context, id int64, role string, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET role=$1, updated_at=NOW(), updated_by=$2 WHERE id_user=$3`, role, updatedBy, id)
	return err
}

func (r *Repository) UpdateUserStatus(ctx context.Context, id int64, status string, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET status=$1, updated_at=NOW(), updated_by=$2 WHERE id_user=$3`, status, updatedBy, id)
	return err
}

func (r *Repository) ResetPassword(ctx context.Context, id int64, passwordHash string, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password=$1, updated_at=NOW(), updated_by=$2 WHERE id_user=$3`, passwordHash, updatedBy, id)
	return err
}

func (r *Repository) DeleteUser(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM users WHERE id_user=$1`, id)
	return err
}

// UserExists checks if username already exists.
func (r *Repository) UserExists(ctx context.Context, username string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(username) = LOWER($1)`, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DropPoint — admin CRUD.

type DropPoint struct {
	ID            int64    `json:"id_drop_point"`
	NamaDropPoint string   `json:"nama_drop_point"`
	Alamat        *string  `json:"alamat,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	Status        string   `json:"status"`
	CreatedAt     *string  `json:"created_at,omitempty"`
	CreatedBy     *int64   `json:"created_by,omitempty"`
	UpdatedAt     *string  `json:"updated_at,omitempty"`
	UpdatedBy     *int64   `json:"updated_by,omitempty"`
}

type DropPointRequest struct {
	NamaDropPoint string   `json:"nama_drop_point"`
	Alamat        *string  `json:"alamat,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	Status        string   `json:"status"`
}

func (r *Repository) ListDropPoint(ctx context.Context) ([]DropPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_drop_point, nama_drop_point, alamat, latitude, longitude, COALESCE(status, 'aktif'),
		       created_at, created_by, updated_at, updated_by
		FROM drop_point ORDER BY id_drop_point`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DropPoint
	for rows.Next() {
		var dp DropPoint
		if err := rows.Scan(&dp.ID, &dp.NamaDropPoint, &dp.Alamat, &dp.Latitude, &dp.Longitude, &dp.Status,
			&dp.CreatedAt, &dp.CreatedBy, &dp.UpdatedAt, &dp.UpdatedBy); err != nil {
			return nil, fmt.Errorf("scan drop_point: %w", err)
		}
		items = append(items, dp)
	}
	return items, rows.Err()
}

func (r *Repository) CreateDropPoint(ctx context.Context, req DropPointRequest, createdBy int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO drop_point (nama_drop_point, alamat, latitude, longitude, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id_drop_point`,
		req.NamaDropPoint, req.Alamat, req.Latitude, req.Longitude, req.Status, createdBy,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateDropPoint(ctx context.Context, id int64, req DropPointRequest, updatedBy int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE drop_point SET nama_drop_point=$1, alamat=$2, latitude=$3, longitude=$4, status=$5,
		                      updated_at=NOW(), updated_by=$6
		WHERE id_drop_point=$7`,
		req.NamaDropPoint, req.Alamat, req.Latitude, req.Longitude, req.Status, updatedBy, id,
	)
	return err
}

func (r *Repository) DeleteDropPoint(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE drop_point SET status='nonaktif' WHERE id_drop_point=$1`, id)
	return err
}
