package admin

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ──────── Driver ────────
func (s *Service) ListDriver(ctx context.Context) ([]Driver, error)                                   { return s.repo.ListDriver(ctx) }
func (s *Service) CreateDriver(ctx context.Context, r DriverRequest, createdBy int64) (int64, error)   { return s.repo.CreateDriver(ctx, r, createdBy) }
func (s *Service) UpdateDriver(ctx context.Context, id int64, r DriverRequest, updatedBy int64) error  { return s.repo.UpdateDriver(ctx, id, r, updatedBy) }
func (s *Service) DeleteDriver(ctx context.Context, id int64) error                                    { return s.repo.DeleteDriver(ctx, id) }

// ──────── Kendaraan ────────
func (s *Service) ListKendaraan(ctx context.Context) ([]Kendaraan, error)                                        { return s.repo.ListKendaraan(ctx) }
func (s *Service) CreateKendaraan(ctx context.Context, r KendaraanRequest, createdBy int64) (int64, error)        { return s.repo.CreateKendaraan(ctx, r, createdBy) }
func (s *Service) UpdateKendaraan(ctx context.Context, id int64, r KendaraanRequest, updatedBy int64) error       { return s.repo.UpdateKendaraan(ctx, id, r, updatedBy) }
func (s *Service) DeleteKendaraan(ctx context.Context, id int64) error                                            { return s.repo.DeleteKendaraan(ctx, id) }

// ──────── Seller ────────
func (s *Service) ListSeller(ctx context.Context) ([]Seller, error)                                     { return s.repo.ListSeller(ctx) }
func (s *Service) CreateSeller(ctx context.Context, r SellerRequest, createdBy int64) (int64, error)    { return s.repo.CreateSeller(ctx, r, createdBy) }
func (s *Service) UpdateSeller(ctx context.Context, id int64, r SellerRequest, updatedBy int64) error   { return s.repo.UpdateSeller(ctx, id, r, updatedBy) }
func (s *Service) DeleteSeller(ctx context.Context, id int64) error                                     { return s.repo.DeleteSeller(ctx, id) }

// ──────── Gudang ────────
func (s *Service) ListGudang(ctx context.Context) ([]Gudang, error)                                     { return s.repo.ListGudang(ctx) }
func (s *Service) CreateGudang(ctx context.Context, r GudangRequest, createdBy int64) (int64, error)    { return s.repo.CreateGudang(ctx, r, createdBy) }
func (s *Service) UpdateGudang(ctx context.Context, id int64, r GudangRequest, updatedBy int64) error   { return s.repo.UpdateGudang(ctx, id, r, updatedBy) }
func (s *Service) DeleteGudang(ctx context.Context, id int64) error                                     { return s.repo.DeleteGudang(ctx, id) }

// ──────── DropPoint ────────
func (s *Service) ListDropPoint(ctx context.Context) ([]DropPoint, error)                                        { return s.repo.ListDropPoint(ctx) }
func (s *Service) CreateDropPoint(ctx context.Context, r DropPointRequest, createdBy int64) (int64, error)       { return s.repo.CreateDropPoint(ctx, r, createdBy) }
func (s *Service) UpdateDropPoint(ctx context.Context, id int64, r DropPointRequest, updatedBy int64) error      { return s.repo.UpdateDropPoint(ctx, id, r, updatedBy) }
func (s *Service) DeleteDropPoint(ctx context.Context, id int64) error                                           { return s.repo.DeleteDropPoint(ctx, id) }

// ──────── User ────────
func (s *Service) ListUser(ctx context.Context) ([]User, error) { return s.repo.ListUser(ctx) }

func (s *Service) CreateUser(ctx context.Context, r UserRequest, createdBy int64) (int64, error) {
	exists, err := s.repo.UserExists(ctx, r.Username)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, ErrUsernameExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(r.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	return s.repo.CreateUser(ctx, r, string(hash), createdBy)
}

func (s *Service) UpdateUserRole(ctx context.Context, id int64, role string, updatedBy int64) error {
	return s.repo.UpdateUserRole(ctx, id, role, updatedBy)
}

func (s *Service) UpdateUserStatus(ctx context.Context, id int64, status string, updatedBy int64) error {
	return s.repo.UpdateUserStatus(ctx, id, status, updatedBy)
}

func (s *Service) ResetPassword(ctx context.Context, id int64, newPassword string, updatedBy int64) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.ResetPassword(ctx, id, string(hash), updatedBy)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	return s.repo.DeleteUser(ctx, id)
}
