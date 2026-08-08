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

	var idDriver int64
	if user.IDDriver != nil {
		idDriver = *user.IDDriver
	}
	token, err := s.jwt.Generate(user.ID, user.Username, user.Role, idDriver)
	if err != nil {
		return nil, err
	}

	// Tandai session online (dipakai web: driver "Online" selama belum logout,
	// walau GPS-nya stale karena background/layar mati).
	_ = s.repo.SetLastLogin(ctx, user.ID)

	return &AuthResponse{User: *user, Token: token}, nil
}

// Logout membersihkan penanda session online.
func (s *Service) Logout(ctx context.Context, userID int64) error {
	return s.repo.ClearLastLogin(ctx, userID)
}

// Me mengambil user berdasarkan ID dari token.
func (s *Service) Me(ctx context.Context, userID int64) (*User, error) {
	return s.repo.FindByID(ctx, userID)
}
