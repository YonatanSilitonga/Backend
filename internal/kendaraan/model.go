package kendaraan

// Kendaraan merepresentasikan tabel kendaraan.
type Kendaraan struct {
	ID              int64   `json:"id_kendaraan"`
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKoli   *int    `json:"kapasitas_koli,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}
