package auth

import "time"

// User merepresentasikan baris di tabel users, digabung dengan nama dari karyawan.
type User struct {
	ID        int64     `json:"id_user"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	KaryawanID *int64   `json:"karyawan_id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// AuthResponse adalah hasil login: user + token.
type AuthResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

// LoginRequest adalah body untuk endpoint login.
// Menerima username (web) maupun email (mobile) — salah satu wajib.
type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}
