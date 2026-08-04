package dashboard

import "context"

// Service berisi logic bisnis dashboard.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetSummary(ctx context.Context) (*Summary, error) {
	return s.repo.GetSummary(ctx)
}

func (s *Service) GetAnalisis(ctx context.Context) (*Analisis, error) {
	durasi, err := s.repo.GetDurasiAnalisis(ctx)
	if err != nil {
		return nil, err
	}
	bottleneck, err := s.repo.GetBottleneck(ctx)
	if err != nil {
		return nil, err
	}
	alerts, err := s.repo.GetAlerts(ctx)
	if err != nil {
		return nil, err
	}
	return &Analisis{Durasi: durasi, Bottleneck: bottleneck, Alerts: alerts}, nil
}
