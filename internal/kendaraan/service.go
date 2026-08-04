package kendaraan

import "context"

// Service berisi logic bisnis kendaraan.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ListKendaraan mengambil daftar kendaraan.
func (s *Service) ListKendaraan(ctx context.Context) ([]Kendaraan, error) {
	return s.repo.ListKendaraan(ctx)
}
