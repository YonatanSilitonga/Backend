package admin

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ValidRoles adalah daftar role yang diakui sistem.
var ValidRoles = map[string]bool{
	"admin":         true,
	"direktur":      true,
	"tower_control": true,
	"koor_gudang":   true,
	"driver":        true,
	"driver_pickup": true,
	"kapten":        true,
}

func checkRole(role string) error {
	if !ValidRoles[role] {
		return fmt.Errorf("role tidak dikenal: %s", role)
	}
	return nil
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

// ──────── Implant (master lokasi implant, terpisah dari seller) ────────
func (s *Service) ListImplant(ctx context.Context) ([]Implant, error)                                   { return s.repo.ListImplant(ctx) }
func (s *Service) CreateImplant(ctx context.Context, r ImplantRequest, createdBy int64) (int64, error)   { return s.repo.CreateImplant(ctx, r) }
func (s *Service) UpdateImplant(ctx context.Context, id int64, r ImplantRequest, updatedBy int64) error  { return s.repo.UpdateImplant(ctx, id, r) }
func (s *Service) DeleteImplant(ctx context.Context, id int64) error                                    { return s.repo.DeleteImplant(ctx, id) }

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
	if err := checkRole(r.Role); err != nil {
		return 0, err
	}
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
	if err := checkRole(role); err != nil {
		return err
	}
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

// ──────── Kapten (profil + akun + mapping seller) ────────

const defaultKaptenPassword = "kapten123"

// usernameKapten membuat username unik "kapten <nama>" (collision → tambah angka).
func (s *Service) usernameKapten(ctx context.Context, nama string) (string, error) {
	base := "kapten " + strings.ToLower(strings.TrimSpace(nama))
	if base == "kapten " {
		base = "kapten"
	}
	candidate := base
	for n := 2; ; n++ {
		exists, err := s.repo.UserExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s %d", base, n)
	}
}

func (s *Service) ListKapten(ctx context.Context) ([]Kapten, error) {
	return s.repo.ListKapten(ctx)
}

func (s *Service) GetKapten(ctx context.Context, id int64) (*Kapten, error) {
	return s.repo.GetKapten(ctx, id)
}

// CreateKapten membuat akun login + profil + mapping dalam satu transaksi.
// Username auto bila kosong; password default kapten123 bila kosong (dikembalikan sekali).
func (s *Service) CreateKapten(ctx context.Context, r KaptenRequest, createdBy int64) (*KaptenCreated, error) {
	if strings.TrimSpace(r.Nama) == "" {
		return nil, fmt.Errorf("nama kapten wajib diisi")
	}
	status := r.Status
	if status == "" {
		status = "aktif"
	}
	if status != "aktif" && status != "nonaktif" {
		return nil, fmt.Errorf("status harus 'aktif' atau 'nonaktif'")
	}
	username := ""
	if r.Username != nil {
		username = strings.TrimSpace(*r.Username)
	}
	if username == "" {
		var err error
		username, err = s.usernameKapten(ctx, r.Nama)
		if err != nil {
			return nil, err
		}
	} else {
		exists, err := s.repo.UserExists(ctx, username)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrUsernameExists
		}
	}
	password := ""
	if r.Password != nil {
		password = *r.Password
	}
	if password == "" {
		password = defaultKaptenPassword
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("password minimal 6 karakter")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	// Implant wajib: kapten baru harus memegang minimal 1 implant.
	if len(r.ImplantIDs) == 0 {
		return nil, fmt.Errorf("kapten wajib memegang minimal 1 implant")
	}
	idKapten, idUser, err := s.repo.CreateKaptenTx(ctx, username, string(hash), strings.TrimSpace(r.Nama), r.NoHP, status, r.SellerIDs, r.ImplantIDs, r.ImplantPeran, createdBy)
	if err != nil {
		return nil, err
	}
	return &KaptenCreated{
		IDKapten:     idKapten,
		IDUser:       idUser,
		Username:     username,
		PasswordAwal: password,
		SellerCount:  len(r.SellerIDs),
		ImplantCount: len(r.ImplantIDs),
	}, nil
}

// UpdateKapten ubah profil; sellerIDs==nil berarti mapping seller tidak diubah,
// implantIDs==nil berarti mapping implant tidak diubah.
// Bila implant_ids dikirim, minimal 1 (implant wajib punya kapten & kapten wajib pegang implant).
func (s *Service) UpdateKapten(ctx context.Context, id int64, r KaptenRequest, updatedBy int64) error {
	if strings.TrimSpace(r.Nama) == "" {
		return fmt.Errorf("nama kapten wajib diisi")
	}
	status := r.Status
	if status == "" {
		status = "aktif"
	}
	if status != "aktif" && status != "nonaktif" {
		return fmt.Errorf("status harus 'aktif' atau 'nonaktif'")
	}
	if r.ImplantIDs != nil && len(r.ImplantIDs) == 0 {
		return fmt.Errorf("kapten wajib memegang minimal 1 implant")
	}
	return s.repo.UpdateKaptenTx(ctx, id, strings.TrimSpace(r.Nama), r.NoHP, status, r.SellerIDs, r.ImplantIDs, r.SellerIDs != nil, r.ImplantIDs != nil, r.ImplantPeran, updatedBy)
}

func (s *Service) DeleteKapten(ctx context.Context, id int64, updatedBy int64) error {
	return s.repo.DeleteKaptenTx(ctx, id, updatedBy)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	return s.repo.DeleteUser(ctx, id)
}
