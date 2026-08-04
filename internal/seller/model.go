package seller

// Seller merepresentasikan satu baris di tabel seller.
type Seller struct {
	ID              int64    `json:"id_seller"`
	KodeSeller      string   `json:"kode_seller"`
	NamaSeller      string   `json:"nama_seller"`
	Alamat          string   `json:"alamat"`
	Kota            string   `json:"kota"`
	Area            string   `json:"area,omitempty"`
	Pic             string   `json:"pic"`
	NoHP            string   `json:"no_hp"`
	JamMulaiPickup  string   `json:"jam_mulai_pickup,omitempty"`
	JamSelesaiPickup string  `json:"jam_selesai_pickup,omitempty"`
	ForecastHarian  *int     `json:"forecast_harian,omitempty"`
	Status          string   `json:"status"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
}
