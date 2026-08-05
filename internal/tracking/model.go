package tracking

// Tracking merepresentasikan tabel armada_tracking.
type Tracking struct {
	ID          int64   `json:"id_tracking"`
	IDRitase    *int64  `json:"id_ritase,omitempty"` // nullable — GPS bisa tanpa ritase
	IDKendaraan int64   `json:"id_kendaraan"`
	IDDriver    int64   `json:"id_driver"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Kecepatan   *int    `json:"kecepatan,omitempty"`
	Arah        *int    `json:"arah,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// CreateTrackingRequest adalah body untuk menyimpan posisi GPS.
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
