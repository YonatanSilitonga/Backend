package admin

// Driver — admin CRUD.
type Driver struct {
	ID           int64   `json:"id_driver"`
	NamaDriver   string  `json:"nama_driver"`
	NoHP         *string `json:"no_hp,omitempty"`
	NoSIM        *string `json:"no_sim,omitempty"`
	JenisSIM     *string `json:"jenis_sim,omitempty"`
	StatusDriver string  `json:"status_driver"`
}

type DriverRequest struct {
	NamaDriver   string  `json:"nama_driver"`
	NoHP         *string `json:"no_hp,omitempty"`
	NoSIM        *string `json:"no_sim,omitempty"`
	JenisSIM     *string `json:"jenis_sim,omitempty"`
	StatusDriver string  `json:"status_driver"`
}

// Kendaraan — admin CRUD.
type Kendaraan struct {
	ID              int64   `json:"id_kendaraan"`
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKg     *int64  `json:"kapasitas_kg,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}

type KendaraanRequest struct {
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKg     *int64  `json:"kapasitas_kg,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}

// Seller — admin CRUD.
type Seller struct {
	ID              int64   `json:"id_seller"`
	KodeSeller      string  `json:"kode_seller"`
	NamaSeller      string  `json:"nama_seller"`
	Alamat          *string `json:"alamat,omitempty"`
	Kota            *string `json:"kota,omitempty"`
	Area            *string `json:"area,omitempty"`
	Pic             *string `json:"pic,omitempty"`
	NoHP            *string `json:"no_hp,omitempty"`
	ForecastHarian  *int64  `json:"forecast_harian,omitempty"`
	Status          string  `json:"status"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
}

type SellerRequest struct {
	KodeSeller      string   `json:"kode_seller"`
	NamaSeller      string   `json:"nama_seller"`
	Alamat          *string  `json:"alamat,omitempty"`
	Kota            *string  `json:"kota,omitempty"`
	Area            *string  `json:"area,omitempty"`
	Pic             *string  `json:"pic,omitempty"`
	NoHP            *string  `json:"no_hp,omitempty"`
	ForecastHarian  *int64   `json:"forecast_harian,omitempty"`
	Status          string   `json:"status"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
}

// Gudang — admin CRUD.
type Gudang struct {
	ID         int64   `json:"id_gudang"`
	NamaGudang string  `json:"nama_gudang"`
	Alamat     *string `json:"alamat,omitempty"`
	Kota       *string `json:"kota,omitempty"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	Status     string  `json:"status"`
}

type GudangRequest struct {
	NamaGudang string   `json:"nama_gudang"`
	Alamat     *string  `json:"alamat,omitempty"`
	Kota       *string  `json:"kota,omitempty"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	Status     string   `json:"status"`
}

// User — admin CRUD.
type User struct {
	ID         int64   `json:"id_user"`
	Username   string  `json:"username"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	KaryawanID *int64  `json:"karyawan_id,omitempty"`
	IsActive   bool    `json:"is_active"`
}

type UserRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	KaryawanID *int64 `json:"karyawan_id,omitempty"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}
