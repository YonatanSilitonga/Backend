package armada

import "time"

// Kendaraan merepresentasikan tabel kendaraan.
type Kendaraan struct {
	ID              int64   `json:"id_kendaraan"`
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKoli   *int    `json:"kapasitas_koli,omitempty"`
	KapasitasKg     *int    `json:"kapasitas_kg,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}

// Driver merepresentasikan tabel driver.
type Driver struct {
	ID            int64   `json:"id_driver"`
	NamaDriver    string  `json:"nama_driver"`
	NoHP          *string `json:"no_hp,omitempty"`
	NoSIM         *string `json:"no_sim,omitempty"`
	JenisSIM      *string `json:"jenis_sim,omitempty"`
	StatusDriver  string  `json:"status_driver"`
	PlatNomor     *string `json:"plat_nomor,omitempty"`
	IDKendaraan   *int64  `json:"id_kendaraan,omitempty"`
	TrackingFresh bool    `json:"tracking_fresh"`
}

// Ritase merepresentasikan tabel ritase (penugasan perjalanan angkut).
type Ritase struct {
	ID               int64     `json:"id_ritase"`
	KodeRitase       string    `json:"kode_ritase"`
	Tanggal          string    `json:"tanggal"`
	IDDriver         int64     `json:"id_driver"`
	NamaDriver       string    `json:"nama_driver,omitempty"`
	IDKendaraan      int64     `json:"id_kendaraan"`
	PlatNomor        string    `json:"plat_nomor,omitempty"`
	IDSeller         int64     `json:"id_seller,omitempty"`
	NamaSeller       string    `json:"nama_seller,omitempty"`
	IDDropPoint      int64     `json:"id_drop_point"`
	NamaDropPoint    string    `json:"nama_drop_point,omitempty"`
	RitaseKe         *int      `json:"ritase_ke,omitempty"`
	TotalAWB         *int      `json:"total_awb,omitempty"`
	TotalKoli        *int      `json:"total_koli,omitempty"`
	TotalHighValue   *int      `json:"total_high_value,omitempty"`
	TotalEceran      *int      `json:"total_eceran,omitempty"`
	PaketTertinggal  *int      `json:"paket_tertinggal,omitempty"`
	AlasanTertinggal *string   `json:"alasan_tertinggal,omitempty"`
	JamBerangkat     *string   `json:"jam_berangkat,omitempty"`
	JamTiba          *string   `json:"jam_tiba,omitempty"`
	JamMulai         *string   `json:"jam_mulai,omitempty"`
	JamSelesai       *string   `json:"jam_selesai,omitempty"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
}

// RitaseEvent adalah satu baris timeline status perjalanan (10 status tombol driver).
type RitaseEvent struct {
	ID              int64     `json:"id_event"`
	IDRitase        int64     `json:"id_ritase"`
	Status          string    `json:"status"`
	Catatan         *string   `json:"catatan,omitempty"`
	Latitude        *float64  `json:"latitude,omitempty"`
	Longitude       *float64  `json:"longitude,omitempty"`
	NamaLokasi      *string   `json:"nama_lokasi,omitempty"`
	DurasiDetik     *int      `json:"durasi_detik,omitempty"`
	JumlahKoli      *int      `json:"jumlah_koli,omitempty"`
	JumlahEcer      *int      `json:"jumlah_ecer,omitempty"`
	JumlahHighValue *int      `json:"jumlah_high_value,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// RitaseDetail adalah ritase + seluruh rute stops + timeline event-nya.
type RitaseDetail struct {
	Ritase
	Stops  []RitaseStop  `json:"stops"`
	Events []RitaseEvent `json:"events"`
}

// Tracking merepresentasikan tabel armada_tracking (posisi realtime).
type Tracking struct {
	ID          int64     `json:"id_tracking"`
	IDRitase    *int64    `json:"id_ritase,omitempty"`
	IDKendaraan int64     `json:"id_kendaraan"`
	IDDriver    int64     `json:"id_driver"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Kecepatan   *int      `json:"kecepatan,omitempty"`
	Arah        *int      `json:"arah,omitempty"`
	Status      *string   `json:"status,omitempty"`
	LastUpdate  time.Time `json:"last_update"`
}

// TrackingLive adalah posisi TERBARU satu kendaraan (1 baris per kendaraan) — untuk map.
type TrackingLive struct {
	ID            int64      `json:"id_tracking"`
	IDRitase      *int64     `json:"id_ritase,omitempty"`
	IDKendaraan   int64      `json:"id_kendaraan"`
	PlatNomor     string     `json:"plat_nomor"`
	IDDriver      int64      `json:"id_driver"`
	NamaDriver    string     `json:"nama_driver"`
	Latitude      float64    `json:"latitude"`
	Longitude     float64    `json:"longitude"`
	Kecepatan     *int       `json:"kecepatan,omitempty"`
	Arah          *int       `json:"arah,omitempty"`
	Status        *string    `json:"status,omitempty"`
	NamaLokasi      *string    `json:"nama_lokasi,omitempty"`
	JumlahKoli      *int       `json:"total_koli,omitempty"`
	JumlahEcer      *int       `json:"total_eceran,omitempty"`
	JumlahHighValue *int       `json:"total_high_value,omitempty"`
	LastUpdate      time.Time  `json:"last_update"`
	Offline         bool       `json:"offline"`
	SessionOnline   bool       `json:"session_online"`
	LastLogin       *time.Time `json:"last_login,omitempty"`
	LastOpen        *time.Time `json:"last_open,omitempty"`
}

// MapTracking gabungan posisi live kendaraan + titik seller + gudang + drop_point (data peta).
type MapTracking struct {
	Vehicles   []TrackingLive   `json:"vehicles"`
	Sellers    []SellerLocation `json:"sellers"`
	Gudang     []GudangPoint    `json:"gudang"`
	DropPoints []DropPointPoi   `json:"drop_points"`
}

// TrackingCheckpoint satu baris riwayat status dari ritase_event.
type TrackingCheckpoint struct {
	IDEvent         int64     `json:"id_event"`
	IDRitase        int64     `json:"id_ritase"`
	KodeRitase      string    `json:"kode_ritase"`
	Status          string    `json:"status"`
	Catatan         *string   `json:"catatan,omitempty"`
	Latitude        *float64  `json:"latitude,omitempty"`
	Longitude       *float64  `json:"longitude,omitempty"`
	NamaLokasi      *string   `json:"nama_lokasi,omitempty"`
	DurasiDetik     *int      `json:"durasi_detik,omitempty"`
	JumlahKoli      *int      `json:"jumlah_koli,omitempty"`
	JumlahEcer      *int      `json:"jumlah_ecer,omitempty"`
	JumlahHighValue *int      `json:"jumlah_high_value,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

/* ---------- Request bodies ---------- */

type CreateRitaseRequest struct {
	KodeRitase  string              `json:"kode_ritase"`
	Tanggal     string              `json:"tanggal"`
	IDDriver    int64               `json:"id_driver"`
	IDKendaraan int64               `json:"id_kendaraan"`
	IDSeller    int64               `json:"id_seller"`
	IDDropPoint int64               `json:"id_drop_point"`
	RitaseKe    *int                `json:"ritase_ke"`
	TotalAWB    *int                `json:"total_awb"`
	TotalKoli   *int                `json:"total_koli"`
	Stops       []RitaseStopRequest `json:"stops"`
}

type UpdateStatusRequest struct {
	Status          string   `json:"status"`
	Catatan         *string  `json:"catatan"`
	Latitude        *float64 `json:"latitude"`
	Longitude       *float64 `json:"longitude"`
	NamaLokasi      *string  `json:"nama_lokasi"`
	DurasiDetik     *int     `json:"durasi_detik"`
	JumlahKoli      *int     `json:"jumlah_koli"`
	JumlahEcer      *int     `json:"jumlah_ecer"`
	JumlahHighValue *int     `json:"jumlah_high_value"`
}

type UpdateMuatanRequest struct {
	TotalAWB         *int    `json:"total_awb"`
	TotalKoli        *int    `json:"total_koli"`
	PaketTertinggal  *int    `json:"paket_tertinggal"`
	AlasanTertinggal *string `json:"alasan_tertinggal"`
}

type CreateTrackingRequest struct {
	IDRitase        int64   `json:"id_ritase"`
	IDKendaraan     int64   `json:"id_kendaraan"`
	IDDriver        int64   `json:"id_driver"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Kecepatan       *int    `json:"kecepatan"`
	Arah            *int    `json:"arah"`
	Status          *string `json:"status"`
	JumlahKoli      int     `json:"jumlah_koli"`
	JumlahEcer      int     `json:"jumlah_ecer"`
	JumlahHighValue int     `json:"jumlah_high_value"`
	DurasiDetik     *int    `json:"durasi_detik"`
}
