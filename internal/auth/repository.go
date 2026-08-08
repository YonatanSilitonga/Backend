package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("user tidak ditemukan")
)

// Repository mengakses tabel users (+ join karyawan untuk nama).
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindByUsername mengambil user + password hash berdasarkan username.
func (r *Repository) FindByUsername(ctx context.Context, username string) (*User, string, error) {
	query := `
		SELECT u.id_user, u.username, COALESCE(k.nama, u.username) AS name,
		       u.role, u.karyawan_id, u.id_driver
		FROM users u
		LEFT JOIN karyawan k ON k.id_karyawan = u.karyawan_id
		WHERE LOWER(u.username) = LOWER($1)
	`
	var u User
	var pwHash string
	err := r.db.QueryRow(ctx, query, username).Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.KaryawanID, &u.IDDriver)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}

	// ambil password hash terpisah (gak dimasukkan ke struct user)
	if err := r.db.QueryRow(ctx, "SELECT password FROM users WHERE id_user = $1", u.ID).Scan(&pwHash); err != nil {
		return nil, "", err
	}

	return &u, pwHash, nil
}

// FindByID mengambil user berdasarkan ID.
func (r *Repository) FindByID(ctx context.Context, id int64) (*User, error) {
	query := `
		SELECT u.id_user, u.username, COALESCE(k.nama, u.username) AS name,
		       u.role, u.karyawan_id, u.id_driver
		FROM users u
		LEFT JOIN karyawan k ON k.id_karyawan = u.karyawan_id
		WHERE u.id_user = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.KaryawanID, &u.IDDriver)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// SetLastLogin menandai user sedang online (session aktif).
func (r *Repository) SetLastLogin(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login = now() WHERE id_user = $1`, id)
	return err
}

// ClearLastLogin menandai user logout (session selesai).
func (r *Repository) ClearLastLogin(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login = NULL WHERE id_user = $1`, id)
	return err
}
