package admin

// Driver — admin CRUD.
type Driver struct {
	ID             int64   `json:"id_driver"`
	NamaDriver     string  `json:"nama_driver"`
	NoHP           *string `json:"no_hp,omitempty"`
	NoSIM          *string `json:"no_sim,omitempty"`
	JenisSIM       *string `json:"jenis_sim,omitempty"`
	StatusDriver   string  `json:"status_driver"`
	CreatedAt      *string `json:"created_at,omitempty"`
	CreatedBy      *int64  `json:"created_by,omitempty"`
	CreatedByName  string  `json:"created_by_name,omitempty"`
	UpdatedAt      *string `json:"updated_at,omitempty"`
	UpdatedBy      *int64  `json:"updated_by,omitempty"`
	UpdatedByName  string  `json:"updated_by_name,omitempty"`
}

type DriverRequest struct {
	NamaDriver   string  `json:"nama_driver"`
	NoHP         *string `json:"no_hp,omitempty"`
	NoSIM        *string `json:"no_sim,omitempty"`
	JenisSIM     *string `json:"jenis_sim,omitempty"`
	Jabatan      *string `json:"jabatan,omitempty"`
	StatusDriver string  `json:"status_driver"`
}

// Kendaraan — admin CRUD.
type Kendaraan struct {
	ID              int64   `json:"id_kendaraan"`
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKg     *int64  `json:"kapasitas_kg,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
	CreatedAt       *string `json:"created_at,omitempty"`
	CreatedBy       *int64  `json:"created_by,omitempty"`
	CreatedByName   string  `json:"created_by_name,omitempty"`
	UpdatedAt       *string `json:"updated_at,omitempty"`
	UpdatedBy       *int64  `json:"updated_by,omitempty"`
	UpdatedByName   string  `json:"updated_by_name,omitempty"`
}

type KendaraanRequest struct {
	PlatNomor       string  `json:"plat_nomor"`
	JenisKendaraan  *string `json:"jenis_kendaraan,omitempty"`
	KapasitasKg     *int64  `json:"kapasitas_kg,omitempty"`
	StatusKendaraan string  `json:"status_kendaraan"`
}

// Seller — admin CRUD.
type Seller struct {
	ID             int64    `json:"id_seller"`
	KodeSeller     string   `json:"kode_seller"`
	NamaSeller     string   `json:"nama_seller"`
	Alamat         *string  `json:"alamat,omitempty"`
	Kota           *string  `json:"kota,omitempty"`
	Area           *string  `json:"area,omitempty"`
	Pic            *string  `json:"pic,omitempty"`
	NoHP           *string  `json:"no_hp,omitempty"`
	ForecastHarian *int64   `json:"forecast_harian,omitempty"`
	Status         string   `json:"status"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	CreatedAt      *string  `json:"created_at,omitempty"`
	CreatedBy      *int64   `json:"created_by,omitempty"`
	CreatedByName  string   `json:"created_by_name,omitempty"`
	UpdatedAt      *string  `json:"updated_at,omitempty"`
	UpdatedBy      *int64   `json:"updated_by,omitempty"`
	UpdatedByName  string   `json:"updated_by_name,omitempty"`
}

type SellerRequest struct {
	KodeSeller     string   `json:"kode_seller"`
	NamaSeller     string   `json:"nama_seller"`
	Alamat         *string  `json:"alamat,omitempty"`
	Kota           *string  `json:"kota,omitempty"`
	Area           *string  `json:"area,omitempty"`
	Pic            *string  `json:"pic,omitempty"`
	NoHP           *string  `json:"no_hp,omitempty"`
	ForecastHarian *int64   `json:"forecast_harian,omitempty"`
	Status         string   `json:"status"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
}

// Implant — admin CRUD (master lokasi implant, terpisah dari seller).
// PIC implant = kapten via kapten_implant_map (bukan kolom teks).
type Implant struct {
	ID               int64    `json:"id_implant"`
	KodeImplant      string   `json:"kode_implant"`
	NamaImplant      string   `json:"nama_implant"`
	Alamat           *string  `json:"alamat,omitempty"`
	Kota             *string  `json:"kota,omitempty"`
	Area             *string  `json:"area,omitempty"`
	NoHP             *string  `json:"no_hp,omitempty"`
	JamMulaiPickup   *string  `json:"jam_mulai_pickup,omitempty"`
	JamSelesaiPickup *string  `json:"jam_selesai_pickup,omitempty"`
	ForecastHarian   *int64   `json:"forecast_harian,omitempty"`
	Status           string   `json:"status"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	JarakTempuhKm    *float64 `json:"jarak_tempuh_km,omitempty"`
	JarakDcKm        *float64 `json:"jarak_dc_km,omitempty"`
	JumlahManpower   int64    `json:"jumlah_manpower"`
	CreatedAt        *string  `json:"created_at,omitempty"`
	UpdatedAt        *string  `json:"updated_at,omitempty"`
}

type ImplantRequest struct {
	KodeImplant      string   `json:"kode_implant"`
	NamaImplant      string   `json:"nama_implant"`
	Alamat           *string  `json:"alamat,omitempty"`
	Kota             *string  `json:"kota,omitempty"`
	Area             *string  `json:"area,omitempty"`
	NoHP             *string  `json:"no_hp,omitempty"`
	JamMulaiPickup   *string  `json:"jam_mulai_pickup,omitempty"`
	JamSelesaiPickup *string  `json:"jam_selesai_pickup,omitempty"`
	ForecastHarian   *int64   `json:"forecast_harian,omitempty"`
	Status           string   `json:"status"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	JarakTempuhKm    *float64 `json:"jarak_tempuh_km,omitempty"`
	JarakDcKm        *float64 `json:"jarak_dc_km,omitempty"`
	JumlahManpower   *int64   `json:"jumlah_manpower,omitempty"`
}

// Gudang — admin CRUD.
type Gudang struct {
	ID            int64    `json:"id_gudang"`
	NamaGudang    string   `json:"nama_gudang"`
	Alamat        *string  `json:"alamat,omitempty"`
	Kota          *string  `json:"kota,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	Status        string   `json:"status"`
	CreatedAt     *string  `json:"created_at,omitempty"`
	CreatedBy     *int64   `json:"created_by,omitempty"`
	CreatedByName string   `json:"created_by_name,omitempty"`
	UpdatedAt     *string  `json:"updated_at,omitempty"`
	UpdatedBy     *int64   `json:"updated_by,omitempty"`
	UpdatedByName string   `json:"updated_by_name,omitempty"`
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
	ID            int64   `json:"id_user"`
	Username      string  `json:"username"`
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	IDDriver      *int64  `json:"id_driver,omitempty"`
	IsActive      bool    `json:"is_active"`
	Status        string  `json:"status"`
	CreatedAt     *string `json:"created_at,omitempty"`
	CreatedBy     *int64  `json:"created_by,omitempty"`
	CreatedByName string  `json:"created_by_name,omitempty"`
	UpdatedAt     *string `json:"updated_at,omitempty"`
	UpdatedBy     *int64  `json:"updated_by,omitempty"`
	UpdatedByName string  `json:"updated_by_name,omitempty"`
}

type UserRequest struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	IDDriver *int64 `json:"id_driver,omitempty"`
	Status   string `json:"status,omitempty"`
}

type UserStatusRequest struct {
	Status string `json:"status"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// Kapten — admin CRUD (profil kapten implant + akun login).
// Identitas akun di users; data diri di kapten;
// cakupan lama di kapten_seller_map, cakupan baru di kapten_implant_map.
type Kapten struct {
	ID            int64           `json:"id_kapten"`
	IDUser        *int64          `json:"id_user,omitempty"`
	Username      string          `json:"username,omitempty"`
	Nama          string          `json:"nama"`
	NoHP          *string         `json:"no_hp,omitempty"`
	Status        string          `json:"status"`
	Sellers       []KaptenSeller  `json:"sellers,omitempty"`
	Implants      []KaptenImplant `json:"implants,omitempty"`
	CreatedAt     *string         `json:"created_at,omitempty"`
	CreatedBy     *int64          `json:"created_by,omitempty"`
	CreatedByName string          `json:"created_by_name,omitempty"`
	UpdatedAt     *string         `json:"updated_at,omitempty"`
	UpdatedBy     *int64          `json:"updated_by,omitempty"`
	UpdatedByName string          `json:"updated_by_name,omitempty"`
}

type KaptenSeller struct {
	IDSeller   int64  `json:"id_seller"`
	KodeSeller string `json:"kode_seller,omitempty"`
	NamaSeller string `json:"nama_seller"`
}

type KaptenImplant struct {
	IDImplant   int64  `json:"id_implant"`
	KodeImplant string `json:"kode_implant,omitempty"`
	NamaImplant string `json:"nama_implant"`
	Peran       string `json:"peran,omitempty"`
}

type KaptenRequest struct {
	Nama       string  `json:"nama"`
	NoHP       *string `json:"no_hp,omitempty"`
	Status     string  `json:"status,omitempty"`   // default aktif
	Username   *string `json:"username,omitempty"` // auto "kapten <nama>" bila kosong (create)
	Password   *string `json:"password,omitempty"` // default kapten123 (hanya saat create)
	SellerIDs  []int64 `json:"seller_ids,omitempty"`
	ImplantIDs []int64 `json:"implant_ids,omitempty"`
	// Peran per implant (kunci = id_implant, nilai utama|cadangan).
	// Kosong = 'utama'. Aturan longgar: tidak ada constraint jumlah utama.
	ImplantPeran map[int64]string `json:"implant_peran,omitempty"`
}

type KaptenCreated struct {
	IDKapten      int64  `json:"id_kapten"`
	IDUser        int64  `json:"id_user"`
	Username      string `json:"username"`
	PasswordAwal  string `json:"password_awal"`
	SellerCount   int    `json:"seller_count"`
	ImplantCount  int    `json:"implant_count"`
}
