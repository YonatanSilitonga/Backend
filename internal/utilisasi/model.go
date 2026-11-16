package utilisasi

// Metric menentukan satuan muatan yang dipakai untuk menilai utilisasi.
type Metric string

const (
	MetricKoli Metric = "koli"
	MetricAWB  Metric = "awb"
)

// Status keterisian muatan.
type Status string

const (
	StatusUnder   Status = "under"
	StatusOptimal Status = "optimal"
	StatusOver    Status = "over"
	StatusNoData  Status = "no_data"
)

// Category kategori hari.
type Category string

const (
	CategoryPeak   Category = "peak"
	CategoryNormal Category = "normal"
	CategoryLow    Category = "low"
)

// Cell satu sel agregasi (harian/mingguan).
type Cell struct {
	Trips         int      `json:"trips"`
	TripsWithData int      `json:"trips_with_data"`
	Load          int      `json:"load"`
	Pct           *float64 `json:"pct"`
	Status        Status   `json:"status"`
}

type DailyRit struct {
	RitaseKe int  `json:"ritase_ke"`
	Cell     Cell `json:"cell"`
}

type DailyVehicle struct {
	IDKendaraan    int64      `json:"id_kendaraan"`
	PlatNomor      string     `json:"plat_nomor"`
	JenisKendaraan string     `json:"jenis_kendaraan"`
	Metric         Metric     `json:"metric"`
	CapacityMin    int        `json:"capacity_min"`
	CapacityMax    int        `json:"capacity_max"`
	Rits           []DailyRit `json:"rits"`
	Avg            Cell       `json:"avg"`
}

type DailyGroup struct {
	Tanggal     string         `json:"tanggal"`
	DayCategory Category       `json:"day_category"`
	Vehicles    []DailyVehicle `json:"vehicles"`
}

type WeeklyVehicle struct {
	IDKendaraan    int64             `json:"id_kendaraan"`
	PlatNomor      string            `json:"plat_nomor"`
	JenisKendaraan string            `json:"jenis_kendaraan"`
	Metric         Metric            `json:"metric"`
	CapacityMin    int               `json:"capacity_min"`
	CapacityMax    int               `json:"capacity_max"`
	Categories     map[Category]Cell `json:"categories"`
	Total          Cell              `json:"total"`
}

type WeeklyGroup struct {
	WeekStart string          `json:"week_start"`
	WeekEnd   string          `json:"week_end"`
	Vehicles  []WeeklyVehicle `json:"vehicles"`
}

// MonthCell satu sel bulanan: rata-rata dari nilai mingguan yang punya data.
type MonthCell struct {
	Weeks  int      `json:"weeks"`
	AvgPct *float64 `json:"avg_pct"`
	Status Status   `json:"status"`
}

type MonthlyVehicle struct {
	IDKendaraan    int64                  `json:"id_kendaraan"`
	PlatNomor      string                 `json:"plat_nomor"`
	JenisKendaraan string                 `json:"jenis_kendaraan"`
	Metric         Metric                 `json:"metric"`
	CapacityMin    int                    `json:"capacity_min"`
	CapacityMax    int                    `json:"capacity_max"`
	Categories     map[Category]MonthCell `json:"categories"`
}

type MonthlyGroup struct {
	Month    string           `json:"month"`
	Vehicles []MonthlyVehicle `json:"vehicles"`
}

type Coverage struct {
	TotalTrips    int     `json:"total_trips"`
	TripsWithKoli int     `json:"trips_with_koli"`
	TripsWithAWB  int     `json:"trips_with_awb"`
	KoliPct       float64 `json:"koli_pct"`
	AWBPct        float64 `json:"awb_pct"`
}

type CapacityInfo struct {
	Jenis  string `json:"jenis"`
	Metric Metric `json:"metric"`
	Min    int    `json:"min"`
	Max    int    `json:"max"`
	AWBMax int    `json:"awb_max,omitempty"`
}

type Calibration struct {
	Jenis  string `json:"jenis"`
	Metric Metric `json:"metric"`
	Trips  int    `json:"trips"`
	P95    int    `json:"p95"`
	Min    int    `json:"min"`
	Max    int    `json:"max"`
}

type Meta struct {
	Coverage    Coverage       `json:"coverage"`
	Capacities  []CapacityInfo `json:"capacities"`
	Calibration []Calibration  `json:"calibration"`
}

type TripDetail struct {
	IDRitase       int64    `json:"id_ritase"`
	Tanggal        string   `json:"tanggal"`
	RitaseKe       int      `json:"ritase_ke"`
	WeekStart      string   `json:"week_start"`
	DayCategory    Category `json:"day_category"`
	IDKendaraan    int64    `json:"id_kendaraan"`
	PlatNomor      string   `json:"plat_nomor"`
	JenisKendaraan string   `json:"jenis_kendaraan"`
	NamaDriver     string   `json:"nama_driver"`
	Metric         Metric   `json:"metric"`
	CapacityMin    int      `json:"capacity_min"`
	CapacityMax    int      `json:"capacity_max"`
	Load           int      `json:"load"`
	Pct            *float64 `json:"pct"`
	Status         Status   `json:"status"`
	HasData        bool     `json:"has_data"`
	Source         string   `json:"source"`
	Koli           int      `json:"koli"`
	AWB            int      `json:"awb"`
}

type Response struct {
	StartDate string         `json:"start_date"`
	EndDate   string         `json:"end_date"`
	Meta      Meta           `json:"meta"`
	Daily     []DailyGroup   `json:"daily"`
	Weekly    []WeeklyGroup  `json:"weekly"`
	Monthly   []MonthlyGroup `json:"monthly"`
	Trips     []TripDetail   `json:"trips"`
}

type DayCategoryItem struct {
	Tanggal    string   `json:"tanggal"`
	Kategori   Category `json:"kategori"`
	Source     string   `json:"source"`
	Keterangan string   `json:"keterangan,omitempty"`
}
