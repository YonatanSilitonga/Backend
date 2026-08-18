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
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
}

// RitaseEvent adalah satu baris timeline status perjalanan (10 status tombol driver).
type RitaseEvent struct {
	ID        int64     `json:"id_event"`
	IDRitase  int64     `json:"id_ritase"`
	Status    string    `json:"status"`
	Catatan   *string   `json:"catatan,omitempty"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	DurasiDetik *int      `json:"durasi_detik,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RitaseStop merepresentasikan satu titik perhentian dalam rute penugasan ritase.
type RitaseStop struct {
	ID            int64    `json:"id_stop"`
	IDRitase      int64    `json:"id_ritase"`
	Urutan        int      `json:"urutan"`
	JenisStop     string   `json:"jenis_stop"`
	IDGudang      *int64   `json:"id_gudang,omitempty"`
	NamaGudang    *string  `json:"nama_gudang,omitempty"`
	TipeGudang    *string  `json:"tipe_gudang,omitempty"`
	IDSeller      *int64   `json:"id_seller,omitempty"`
	NamaSeller    *string  `json:"nama_seller,omitempty"`
	IDDropPoint   *int64   `json:"id_drop_point,omitempty"`
	NamaDropPoint *string  `json:"nama_drop_point,omitempty"`
	Keterangan    *string  `json:"keterangan,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
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

// Request body
type CreateRitaseRequest struct {
	KodeRitase  string `json:"kode_ritase"`
	Tanggal     string `json:"tanggal"`
	IDDriver    int64  `json:"id_driver"`
	IDKendaraan int64  `json:"id_kendaraan"`
	IDSeller    int64  `json:"id_seller"`
	IDDropPoint int64  `json:"id_drop_point"`
	RitaseKe    *int   `json:"ritase_ke"`
	TotalAWB    *int   `json:"total_awb"`
	TotalKoli   *int   `json:"total_koli"`
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
