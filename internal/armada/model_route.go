package armada

// File khusus tipe MILIK WEB (rute/gudang + seller detail).
// Dipisah dari model.go biar perubahan tim di model.go TIDAK menimpa tipe ini.
// JANGAN hapus file ini tanpa koordinasi — dipakai backend & frontend web.

// RitaseStop adalah satu titik dalam rute ritase (gudang -> seller(s) -> drop_point/GTW).
type RitaseStop struct {
	IDStop         int64   `json:"id_stop"`
	IDRitase       int64   `json:"id_ritase"`
	Urutan         int     `json:"urutan"`
	JenisStop      string  `json:"jenis_stop"` // gudang | seller | drop_point
	IDGudang       *int64  `json:"id_gudang,omitempty"`
	NamaGudang     *string `json:"nama_gudang,omitempty"`
	TipeGudang     *string `json:"tipe_gudang,omitempty"`
	IDSeller       *int64  `json:"id_seller,omitempty"`
	IDDropPoint    *int64  `json:"id_drop_point,omitempty"`
	NamaSeller     *string `json:"nama_seller,omitempty"`
	NamaDropPoint  *string `json:"nama_drop_point,omitempty"`
	Keterangan     *string `json:"keterangan,omitempty"`
}

// RitaseStopRequest adalah satu titik rute saat membuat ritase.
type RitaseStopRequest struct {
	Urutan      int     `json:"urutan"`
	JenisStop   string  `json:"jenis_stop"`
	IDGudang    *int64  `json:"id_gudang"`
	IDSeller    *int64  `json:"id_seller"`
	IDDropPoint *int64  `json:"id_drop_point"`
	Keterangan  *string `json:"keterangan"`
}

// GudangPoint posisi gudang untuk peta (Outgoing/Incoming=DC), dibaca dari tabel gudang.
type GudangPoint struct {
	IDGudang   int64   `json:"id_gudang"`
	NamaGudang string  `json:"nama_gudang"`
	Tipe       string  `json:"tipe"` // outgoing | incoming
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

// DropPointPoi posisi drop_point (Gateway JKT/SEG) untuk peta.
type DropPointPoi struct {
	IDDropPoint  int64    `json:"id_drop_point"`
	KodeDP       string   `json:"kode_dp,omitempty"`
	NamaDP       string   `json:"nama_drop_point,omitempty"`
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	// Jarak dari OUTGOING & dari DC, dihitung sekali via tools/fill_jarak.
	JarakTempuhKm *float64 `json:"jarak_tempuh_km,omitempty"` // dari Gudang Outgoing
	JarakDcKm     *float64 `json:"jarak_dc_km,omitempty"`     // dari Gudang DC
}

// SellerLocation lokasi toko seller untuk peta (termasuk kontak PIC/NoHP).
type SellerLocation struct {
	IDSeller        int64    `json:"id_seller"`
	KodeSeller      string   `json:"kode_seller,omitempty"`
	NamaSeller      string   `json:"nama_seller"`
	Alamat          string   `json:"alamat"`
	Kota            string   `json:"kota"`
	PIC             string   `json:"pic,omitempty"`
	NoHP            string   `json:"no_hp,omitempty"`
Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	// Jarak dari OUTGOING & dari DC (Buaran Indah), dihitung sekali via tools/fill_jarak.
	JarakTempuhKm  *float64 `json:"jarak_tempuh_km,omitempty"` // dari Gudang Outgoing
	JarakDcKm      *float64 `json:"jarak_dc_km,omitempty"`     // dari Gudang DC
}