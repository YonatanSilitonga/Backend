package armada

import "context"

// Service berisi logic bisnis modul armada.
type Service struct {
	repo *Repository
	// Ambang offline (menit tanpa GPS) — default 15.
	offlineMin int
	// Ambang session (jam sejak login) — default 12.
	sessionHours int
	// Wajib session aktif buat LIVE (offline = GPS basi ATAU gak login).
	sessionRequired bool
}

func NewService(repo *Repository, offlineMin int, sessionHours int, sessionRequired bool) *Service {
	if offlineMin <= 0 {
		offlineMin = 15
	}
	if sessionHours <= 0 {
		sessionHours = 12
	}
	return &Service{repo: repo, offlineMin: offlineMin, sessionHours: sessionHours, sessionRequired: sessionRequired}
}

func (s *Service) ListKendaraan(ctx context.Context) ([]Kendaraan, error) {
	return s.repo.ListKendaraan(ctx)
}

func (s *Service) ListDriver(ctx context.Context) ([]Driver, error) {
	return s.repo.ListDriver(ctx)
}

func (s *Service) ListRitase(ctx context.Context, idDriver int64, startDate string, endDate string) ([]Ritase, error) {
	return s.repo.ListRitase(ctx, idDriver, startDate, endDate)
}

func (s *Service) GetRitase(ctx context.Context, id int64) (*RitaseDetail, error) {
	return s.repo.GetRitase(ctx, id)
}

func (s *Service) CreateRitase(ctx context.Context, req CreateRitaseRequest) (*Ritase, error) {
	return s.repo.CreateRitase(ctx, req)
}

func (s *Service) UpdateStatus(ctx context.Context, idRitase int64, req UpdateStatusRequest) (*RitaseEvent, error) {
	return s.repo.AddEvent(ctx, idRitase, req)
}

func (s *Service) UpdateMuatan(ctx context.Context, idRitase int64, req UpdateMuatanRequest) (*Ritase, error) {
	return s.repo.UpdateMuatan(ctx, idRitase, req)
}

func (s *Service) ListTracking(ctx context.Context, limit int) ([]Tracking, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListTracking(ctx, limit)
}

func (s *Service) CreateTracking(ctx context.Context, req CreateTrackingRequest) (*Tracking, error) {
	return s.repo.CreateTracking(ctx, req)
}

func (s *Service) GetTrackingMap(ctx context.Context) (*MapTracking, error) {
	vehicles, err := s.repo.ListLatestTracking(ctx, s.offlineMin, s.sessionHours, s.sessionRequired)
	if err != nil {
		return nil, err
	}
	sellers, err := s.repo.ListSellerLocations(ctx)
	if err != nil {
		return nil, err
	}
	gudang, err := s.repo.ListGudangLocations(ctx)
	if err != nil {
		return nil, err
	}
	drops, err := s.repo.ListDropPoints(ctx)
	if err != nil {
		return nil, err
	}
	return &MapTracking{Vehicles: vehicles, Sellers: sellers, Gudang: gudang, DropPoints: drops}, nil
}

func (s *Service) GetTrackingHistory(ctx context.Context, idKendaraan int64, tanggal string) ([]TrackingCheckpoint, error) {
	return s.repo.ListTrackingHistory(ctx, idKendaraan, tanggal)
}
