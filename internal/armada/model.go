package armada

import "time"

// Kendaraan merepresentasikan tabel kendaraan.
type Kendaraan struct {
	ID              int64   `json:"id_kendaraan"`
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKoli   *int    `json:"kapasitas_koli,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}

// Driver merepresentasikan tabel driver.
type Driver struct {
	ID           int64   `json:"id_driver"`
	NamaDriver   string  `json:"nama_driver"`
	NoHP         *string `json:"no_hp,omitempty"`
	NoSIM        *string `json:"no_sim,omitempty"`
	JenisSIM     *string `json:"jenis_sim,omitempty"`
	StatusDriver string  `json:"status_driver"`
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
	IDSeller         int64     `json:"id_seller"`
	NamaSeller       string    `json:"nama_seller,omitempty"`
	IDDropPoint      int64     `json:"id_drop_point"`
	NamaDropPoint    string    `json:"nama_drop_point,omitempty"`
	RitaseKe         *int      `json:"ritase_ke,omitempty"`
	TotalAWB         *int      `json:"total_awb,omitempty"`
	TotalKoli        *int      `json:"total_koli,omitempty"`
	PaketTertinggal  *int      `json:"paket_tertinggal,omitempty"`
	AlasanTertinggal *string   `json:"alasan_tertinggal,omitempty"`
	JamBerangkat     *string   `json:"jam_berangkat,omitempty"`
	JamTiba          *string   `json:"jam_tiba,omitempty"`
	JamMulai         *string   `json:"jam_mulai,omitempty"`
	JamSelesai       *string   `json:"jam_selesai,omitempty"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
}

// RitaseStop adalah satu titik rute dalam sebuah ritase (implant/seller atau gateway/drop point).
type RitaseStop struct {
	IDStop      int64   `json:"id_stop"`
	IDRitase    int64   `json:"id_ritase"`
	Urutan      int     `json:"urutan"`
	JenisStop   string  `json:"jenis_stop"` // gudang | seller | drop_point
	IDSeller    *int64  `json:"id_seller,omitempty"`
	IDDropPoint *int64  `json:"id_drop_point,omitempty"`
	Keterangan  *string `json:"keterangan,omitempty"`
}

// RitaseEvent adalah satu baris timeline status perjalanan (10 status tombol driver).
type RitaseEvent struct {
	ID          int64     `json:"id_event"`
	IDRitase    int64     `json:"id_ritase"`
	Status      string    `json:"status"`
	Catatan     *string   `json:"catatan,omitempty"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	DurasiDetik *int      `json:"durasi_detik,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RitaseStopInput dipakai saat membuat ritase baru (bagian dari CreateRitaseRequest).
type RitaseStopInput struct {
	Urutan      int     `json:"urutan"`
	JenisStop   string  `json:"jenis_stop"`
	IDSeller    *int64  `json:"id_seller"`
	IDDropPoint *int64  `json:"id_drop_point"`
	Keterangan  *string `json:"keterangan"`
}

// RitaseDetail adalah ritase + seluruh timeline event + rute (stops)-nya.
type RitaseDetail struct {
	Ritase
	Events []RitaseEvent `json:"events"`
	Stops  []RitaseStop  `json:"stops"`
}

// Tracking merepresentasikan tabel armada_tracking (posisi realtime, histori mentah).
type Tracking struct {
	ID          int64     `json:"id_tracking"`
	IDRitase    int64     `json:"id_ritase"`
	IDKendaraan int64     `json:"id_kendaraan"`
	IDDriver    int64     `json:"id_driver"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Kecepatan   *int      `json:"kecepatan,omitempty"`
	Arah        *int      `json:"arah,omitempty"`
	Status      *string   `json:"status,omitempty"`
	LastUpdate  time.Time `json:"last_update"`
}

// TrackingLive adalah posisi TERBARU satu kendaraan (1 baris per kendaraan) — dipakai untuk map.
type TrackingLive struct {
	ID          int64     `json:"id_tracking"`
	IDKendaraan int64     `json:"id_kendaraan"`
	PlatNomor   string    `json:"plat_nomor"`
	IDDriver    int64     `json:"id_driver"`
	NamaDriver  string    `json:"nama_driver"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Kecepatan   *int      `json:"kecepatan,omitempty"`
	Arah        *int      `json:"arah,omitempty"`
	Status      *string   `json:"status,omitempty"`
	LastUpdate  time.Time `json:"last_update"`
}

// SellerLocation adalah titik koordinat implant/gudang seller — dipakai untuk marker statis di map.
type SellerLocation struct {
	IDSeller   int64   `json:"id_seller"`
	NamaSeller string  `json:"nama_seller"`
	Alamat     string  `json:"alamat"`
	Kota       string  `json:"kota"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

// MapTracking gabungan data untuk render map: posisi truk + titik implant/seller.
type MapTracking struct {
	Vehicles []TrackingLive   `json:"vehicles"`
	Sellers  []SellerLocation `json:"sellers"`
}

// TrackingCheckpoint adalah satu baris riwayat status (dari ritase_event) untuk sebuah kendaraan.
type TrackingCheckpoint struct {
	IDEvent     int64     `json:"id_event"`
	IDRitase    int64     `json:"id_ritase"`
	KodeRitase  string    `json:"kode_ritase"`
	Status      string    `json:"status"`
	Catatan     *string   `json:"catatan,omitempty"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	DurasiDetik *int      `json:"durasi_detik,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

/* ---------- Request bodies ---------- */

type CreateRitaseRequest struct {
	KodeRitase  string            `json:"kode_ritase"`
	Tanggal     string            `json:"tanggal"`
	IDDriver    int64             `json:"id_driver"`
	IDKendaraan int64             `json:"id_kendaraan"`
	IDSeller    int64             `json:"id_seller"`
	IDDropPoint int64             `json:"id_drop_point"`
	RitaseKe    *int              `json:"ritase_ke"`
	TotalAWB    *int              `json:"total_awb"`
	TotalKoli   *int              `json:"total_koli"`
	Stops       []RitaseStopInput `json:"stops"`
}

type UpdateStatusRequest struct {
	Status      string   `json:"status"`
	Catatan     *string  `json:"catatan"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	DurasiDetik *int     `json:"durasi_detik"`
}

type UpdateMuatanRequest struct {
	TotalAWB         *int    `json:"total_awb"`
	TotalKoli        *int    `json:"total_koli"`
	PaketTertinggal  *int    `json:"paket_tertinggal"`
	AlasanTertinggal *string `json:"alasan_tertinggal"`
}

type CreateTrackingRequest struct {
	IDRitase    int64   `json:"id_ritase"`
	IDKendaraan int64   `json:"id_kendaraan"`
	IDDriver    int64   `json:"id_driver"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Kecepatan   *int    `json:"kecepatan"`
	Arah        *int    `json:"arah"`
	Status      *string `json:"status"`
}
