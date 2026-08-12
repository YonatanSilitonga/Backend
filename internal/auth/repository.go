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

// SetLastOpen mencatat kapan terakhir app mobile dibuka.
func (r *Repository) SetLastOpen(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_open = now() WHERE id_user = $1`, id)
	return err
}

// UpdatePassword mengganti password hash user (bcrypt sudah di-hash oleh service).
func (r *Repository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password = $1 WHERE id_user = $2`, passwordHash, id)
	return err
}

// GetPasswordHash mengambil hash password user (untuk verifikasi password lama).
func (r *Repository) GetPasswordHash(ctx context.Context, id int64) (string, error) {
	var h string
	err := r.db.QueryRow(ctx, `SELECT password FROM users WHERE id_user = $1`, id).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return h, err
}

// FindUserByUsernameWithPhone mengambil id_user + no_hp driver yang terhubung
// ke akun (users.id_driver → driver.no_hp). Dipakai verifikasi "lupa password".
// Gagal (ErrNotFound) kalau username tidak ada / bukan akun driver (id_driver NULL).
func (r *Repository) FindUserByUsernameWithPhone(ctx context.Context, username string) (*User, string, error) {
	query := `
		SELECT u.id_user, u.username, COALESCE(k.nama, u.username) AS name,
		       u.role, u.karyawan_id, u.id_driver,
		       COALESCE(d.no_hp, '')
		FROM users u
		LEFT JOIN karyawan k ON k.id_karyawan = u.karyawan_id
		LEFT JOIN driver d ON d.id_driver = u.id_driver
		WHERE LOWER(u.username) = LOWER($1)
	`
	var u User
	var noHP string
	err := r.db.QueryRow(ctx, query, username).
		Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.KaryawanID, &u.IDDriver, &noHP)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if u.IDDriver == nil || noHP == "" {
		return nil, "", ErrNotFound
	}
	return &u, noHP, nil
}
