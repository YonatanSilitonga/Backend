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

// GetAnalyticsImplants delegasi ke repository (analitik per implant).
func (s *Service) GetAnalyticsImplants(ctx context.Context, from, to string) ([]ImplantAnalytics, error) {
	return s.repo.GetAnalyticsImplants(ctx, from, to)
}

// GetImplanDilayani delegasi ke repository (rekap implant dilayani per tanggal).
func (s *Service) GetImplanDilayani(ctx context.Context, tanggal string) (*DilayaniPage, error) {
	return s.repo.GetImplanDilayani(ctx, tanggal)
}

// GetImplanDilayaniDetail delegasi ke repository (detail satu implant).
func (s *Service) GetImplanDilayaniDetail(ctx context.Context, id int64, tanggal string) (*DilayaniDetail, error) {
	return s.repo.GetImplanDilayaniDetail(ctx, id, tanggal)
}

// GetDropperDilayani delegasi ke repository (rekap muatan dropper per tanggal).
func (s *Service) GetDropperDilayani(ctx context.Context, tanggal string) (*DropperDilayaniPage, error) {
	return s.repo.GetDropperDilayani(ctx, tanggal)
}

// GetSellerDilayani delegasi ke repository (rekap seller dilayani per tanggal/periode).
func (s *Service) GetSellerDilayani(ctx context.Context, tanggal, from, to string) (*DilayaniPage, error) {
	return s.repo.GetSellerDilayani(ctx, tanggal, from, to)
}

// GetSellerDilayaniDetail delegasi ke repository (detail satu seller).
func (s *Service) GetSellerDilayaniDetail(ctx context.Context, key, tanggal string) (*DilayaniDetail, error) {
	return s.repo.GetSellerDilayaniDetail(ctx, key, tanggal)
}

// ResolveAlert menandai alert sebagai ditangani tower control.
func (s *Service) ResolveAlert(ctx context.Context, idAlert int64, userID int64) error {
	return s.repo.ResolveAlert(ctx, idAlert, userID)
}

// GetAlertsByRitase mengembalikan riwayat alert untuk satu ritase.
func (s *Service) GetAlertsByRitase(ctx context.Context, idRitase int64) ([]AlertAnomali, error) {
	return s.repo.GetAlertsByRitase(ctx, idRitase)
}
