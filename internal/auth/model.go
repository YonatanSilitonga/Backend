package auth

import "time"

// User merepresentasikan baris di tabel users, digabung dengan nama dari karyawan.
type User struct {
	ID         int64   `json:"id_user"`
	Username   string  `json:"username"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	KaryawanID *int64  `json:"karyawan_id,omitempty"`
	IDDriver   *int64  `json:"id_driver,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
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

// ChangePasswordRequest adalah body untuk ganti password (user yang login).
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ResetPasswordRequest adalah body untuk lupa password (tanpa OTP).
// Verifikasi identitas: username + no_hp driver yang terhubung ke akun.
type ResetPasswordRequest struct {
	Username    string `json:"username"`
	NoHP        string `json:"no_hp"`
	NewPassword string `json:"new_password"`
}
