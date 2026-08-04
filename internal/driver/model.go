package driver

// Driver merepresentasikan tabel driver.
type Driver struct {
	ID           int64   `json:"id_driver"`
	NamaDriver   string  `json:"nama_driver"`
	NoHP         *string `json:"no_hp,omitempty"`
	StatusDriver string  `json:"status_driver"`
}
