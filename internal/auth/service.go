package auth

import (
	"context"
	"errors"

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

	token, err := s.jwt.Generate(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{User: *user, Token: token}, nil
}

// Me mengambil user berdasarkan ID dari token.
func (s *Service) Me(ctx context.Context, userID int64) (*User, error) {
	return s.repo.FindByID(ctx, userID)
}
