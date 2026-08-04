package kendaraan

// Kendaraan merepresentasikan tabel kendaraan.
type Kendaraan struct {
	ID              int64   `json:"id_kendaraan"`
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKg     *int    `json:"kapasitas_kg,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}
