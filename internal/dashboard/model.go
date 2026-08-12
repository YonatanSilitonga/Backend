package dashboard

import "time"

// Summary adalah ringkasan KPI untuk dashboard Direktur/Kapten.
type Summary struct {
	// Armada
	TotalKendaraan int64 `json:"total_kendaraan"`
	ArmadaAktif    int64 `json:"armada_aktif"`
	ArmadaSelesai  int64 `json:"armada_selesai"`
	ArmadaIdle     int64 `json:"armada_idle"`
	// ArmadaOnline = jumlah kendaraan yang punya posisi terbaru ≤ 5 menit (GPS fresh).
	ArmadaOnline int64 `json:"armada_online"`
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
	Kategori   string  `json:"kategori"`
	Label      string  `json:"label"`
	Indikator  string  `json:"indikator"`
	Nilai      float64 `json:"nilai"`
	Deskripsi  string  `json:"deskripsi"`
	Rekomendasi string `json:"rekomendasi"`
}

// AlertAnomali adalah notifikasi otomatis untuk kondisi abnormal.
type AlertAnomali struct {
	Tingkat     string    `json:"tingkat"` // info / warning / critical
	Pesan       string    `json:"pesan"`
	Kategori    string    `json:"kategori"`
	Waktu       time.Time `json:"waktu"`
	Deskripsi   string    `json:"deskripsi"`
	Rekomendasi string    `json:"rekomendasi"`
}

// Analisis adalah bundle lengkap untuk dashboard analitik.
type Analisis struct {
	Durasi     *DurasiAnalisis `json:"durasi"`
	Bottleneck []Bottleneck    `json:"bottleneck"`
	Alerts     []AlertAnomali  `json:"alerts"`
}

// Arah ritase berdasarkan drop point (gateway): JKT = outgoing, SEG = incoming.
// Disepakati dengan tim: GTW JKT → barang keluar, GTW SEG → barang masuk.

// TrendPoint adalah satu titik trend harian (GROUP BY ritase.tanggal — tanggal JADWAL).
type TrendPoint struct {
	Tanggal         string `json:"tanggal"` // YYYY-MM-DD
	RitaseTotal     int64  `json:"ritase_total"`
	RitaseSelesai   int64  `json:"ritase_selesai"`
	RitaseBatal     int64  `json:"ritase_batal"`
	TotalAWB        int64  `json:"total_awb"`
	TotalKoli       int64  `json:"total_koli"`
	SellerTerlayani int64  `json:"seller_terlayani"`
	Outgoing        int64  `json:"outgoing"`
	Incoming        int64  `json:"incoming"`
}

// DriverPerf adalah performa satu driver dalam periode (durasi dalam detik, NULL = belum ada data).
type DriverPerf struct {
	IDDriver        int64    `json:"id_driver"`
	NamaDriver      string   `json:"nama_driver"`
	RitaseTotal     int64    `json:"ritase_total"`
	RitaseSelesai   int64    `json:"ritase_selesai"`
	TotalAWB        int64    `json:"total_awb"`
	TotalKoli       int64    `json:"total_koli"`
	PaketTertinggal int64    `json:"paket_tertinggal"`
	Outgoing        int64    `json:"outgoing"`
	Incoming        int64    `json:"incoming"`
	RataLoading     *float64 `json:"rata_loading,omitempty"`
	RataPerjalanan  *float64 `json:"rata_perjalanan,omitempty"`
	RataUnloading   *float64 `json:"rata_unloading,omitempty"`
}

// SellerAnalytics adalah analitik satu seller dalam periode.
type SellerAnalytics struct {
	IDSeller     int64    `json:"id_seller"`
	KodeSeller   string   `json:"kode_seller"`
	NamaSeller   string   `json:"nama_seller"`
	Kota         string   `json:"kota"`
	JarakTempuhKm *float64 `json:"jarak_tempuh_km,omitempty"`
	JarakDcKm     *float64 `json:"jarak_dc_km,omitempty"`
	Kunjungan    int64    `json:"kunjungan"`
	RitaseSelesai int64   `json:"ritase_selesai"`
	TotalAWB     int64    `json:"total_awb"`
	TotalKoli    int64    `json:"total_koli"`
	// RataBongkar = rata-rata durasi di lokasi (sampai_seller → berangkat_seller), detik.
	RataBongkar *float64 `json:"rata_bongkar,omitempty"`
}
