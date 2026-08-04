package driver

import "context"

// Service berisi logic bisnis driver.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ListDriver mengambil daftar driver.
func (s *Service) ListDriver(ctx context.Context) ([]Driver, error) {
	return s.repo.ListDriver(ctx)
}
