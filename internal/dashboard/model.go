package dashboard

import "time"

// Summary adalah ringkasan KPI untuk dashboard Direktur/Kapten.
type Summary struct {
	// Armada
	TotalKendaraan int64 `json:"total_kendaraan"`
	ArmadaAktif    int64 `json:"armada_aktif"`
	ArmadaSelesai  int64 `json:"armada_selesai"`
	ArmadaIdle     int64 `json:"armada_idle"`
	// Driver
	TotalDriver    int64 `json:"total_driver"`
	DriverAktif    int64 `json:"driver_aktif"`
	DriverLibur    int64 `json:"driver_libur"`
	DriverTelat    int64 `json:"driver_telat"`
	// Operasional
	TotalRitase      int64 `json:"total_ritase"`
	RitaseAktif      int64 `json:"ritase_aktif"`
	RitaseSelesai    int64 `json:"ritase_selesai"`
	RitaseToday      int64 `json:"ritase_hari_ini"`
	TotalAWB         int64 `json:"total_awb"`
	TotalAWBToday    int64 `json:"total_awb_hari_ini"`
	TotalKoli        int64 `json:"total_koli"`
	PaketTertinggal  int64 `json:"paket_tertinggal"`
	// Lainnya
	TotalSeller    int64 `json:"total_seller"`
	SellerTerlayani int64 `json:"seller_terlayani"`
	TotalDropPoint int64 `json:"total_drop_point"`
	TotalKaryawan  int64 `json:"total_karyawan"`
	TotalManpower  int64 `json:"total_manpower"`
	TotalAbsensi   int64 `json:"total_absensi"`
	TotalImplant   int64 `json:"total_implant"`
	TotalTracking  int64 `json:"total_tracking"`
}

// DurasiAnalisis adalah ringkasan durasi proses (dari timeline ritase_event).
type DurasiAnalisis struct {
	RataRataLoading   string `json:"rata_rata_loading"`
	RataRataPerjalanan string `json:"rata_rata_perjalanan"`
	RataRataUnloading string `json:"rata_rata_unloading"`
	TotalRitaseDihitung int64 `json:"total_ritase_dihitung"`
}

// Bottleneck adalah daftar titik yang berpotensi menjadi hambatan.
type Bottleneck struct {
	Kategori string `json:"kategori"`
	Label    string `json:"label"`
	Indikator string `json:"indikator"`
	Nilai    float64 `json:"nilai"`
}

// AlertAnomali adalah notifikasi otomatis untuk kondisi abnormal.
type AlertAnomali struct {
	Tingkat  string    `json:"tingkat"` // info / warning / critical
	Pesan    string    `json:"pesan"`
	Kategori string    `json:"kategori"`
	Waktu    time.Time `json:"waktu"`
}

// Analisis adalah bundle lengkap untuk dashboard analitik.
type Analisis struct {
	Durasi     *DurasiAnalisis `json:"durasi"`
	Bottleneck []Bottleneck    `json:"bottleneck"`
	Alerts     []AlertAnomali  `json:"alerts"`
}
