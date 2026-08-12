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

// GetAnalyticsTrend delegasi ke repository (trend harian ritase).
func (s *Service) GetAnalyticsTrend(ctx context.Context, from, to string) ([]TrendPoint, error) {
	return s.repo.GetAnalyticsTrend(ctx, from, to)
}

// GetAnalyticsDrivers delegasi ke repository (performa per driver).
func (s *Service) GetAnalyticsDrivers(ctx context.Context, from, to string) ([]DriverPerf, error) {
	return s.repo.GetAnalyticsDrivers(ctx, from, to)
}

// GetAnalyticsSellers delegasi ke repository (analitik per seller).
func (s *Service) GetAnalyticsSellers(ctx context.Context, from, to string) ([]SellerAnalytics, error) {
	return s.repo.GetAnalyticsSellers(ctx, from, to)
}
