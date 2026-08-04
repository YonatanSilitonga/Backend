package seller

import "context"

// Service berisi aturan bisnis untuk seller.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ListSeller mengambil daftar seller.
func (s *Service) ListSeller(ctx context.Context) ([]Seller, error) {
	return s.repo.ListSeller(ctx)
}
