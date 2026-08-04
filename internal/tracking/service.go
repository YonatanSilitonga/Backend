package tracking

import "context"

// Service berisi logic bisnis tracking.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateTracking menyimpan posisi terbaru.
func (s *Service) CreateTracking(ctx context.Context, req CreateTrackingRequest) (*Tracking, error) {
	return s.repo.CreateTracking(ctx, req)
}
