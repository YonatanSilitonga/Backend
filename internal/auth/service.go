package auth

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"backend/internal/pkg/jwt"
)

// Service berisi logic login/logout & verifikasi kredensial.
type Service struct {
	repo *Repository
	jwt  *jwt.Manager
}

func NewService(repo *Repository, jwtManager *jwt.Manager) *Service {
	return &Service{repo: repo, jwt: jwtManager}
}

// Login memverifikasi username/email + password lalu mengembalikan user & token.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	identifier := req.Username
	if identifier == "" {
		identifier = req.Email
	}
	if identifier == "" || req.Password == "" {
		return nil, errors.New("username/email dan password wajib diisi")
	}

	user, pwHash, err := s.repo.FindByUsername(ctx, identifier)
	if errors.Is(err, ErrNotFound) {
		return nil, errors.New("username atau password salah")
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(pwHash), []byte(req.Password)); err != nil {
		return nil, errors.New("username atau password salah")
	}

	var idDriver int64
	if user.IDDriver != nil {
		idDriver = *user.IDDriver
	}
	token, err := s.jwt.Generate(user.ID, user.Username, user.Role, idDriver)
	if err != nil {
		return nil, err
	}

	// Tandai session online
	_ = s.repo.SetLastLogin(ctx, user.ID)

	return &AuthResponse{User: *user, Token: token}, nil
}

// Logout membersihkan penanda session online.
func (s *Service) Logout(ctx context.Context, userID int64) error {
	return s.repo.ClearLastLogin(ctx, userID)
}

// OpenApp mencatat kapan terakhir app dibuka (dipanggil mobile saat start/resume).
func (s *Service) OpenApp(ctx context.Context, userID int64) error {
	return s.repo.SetLastOpen(ctx, userID)
}

// Me mengambil user berdasarkan ID dari token.
func (s *Service) Me(ctx context.Context, userID int64) (*User, error) {
	return s.repo.FindByID(ctx, userID)
}

// ChangePassword mengganti password user yang sedang login (verifikasi password lama).
func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password baru minimal 6 karakter")
	}

	hash, err := s.repo.GetPasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)); err != nil {
		return errors.New("password lama salah")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userID, string(newHash))
}

// ResetPassword mengganti password via verifikasi username + no_hp driver
// (tanpa OTP — untuk akun driver di app mobile).
func (s *Service) ResetPassword(ctx context.Context, username, noHP, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password baru minimal 6 karakter")
	}

	user, phone, err := s.repo.FindUserByUsernameWithPhone(ctx, username)
	if err != nil {
		return errors.New("username tidak ditemukan")
	}

	// Bandingkan no_hp tanpa spasi/strip, case-insensitive.
	norm := func(v string) string { return strings.ToLower(strings.ReplaceAll(v, " ", "")) }
	if norm(phone) != norm(noHP) {
		return errors.New("nomor HP tidak sesuai dengan akun")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, user.ID, string(newHash))
}
