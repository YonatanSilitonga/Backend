package mobile_api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/eventbus"
	"backend/internal/pkg/response"
)

type APIHandler struct {
	DB         *pgxpool.Pool
	bus        *eventbus.Bus
	trackerKey string
}

func NewAPIHandler(db *pgxpool.Pool, bus *eventbus.Bus, trackerKey string) *APIHandler {
	return &APIHandler{DB: db, bus: bus, trackerKey: trackerKey}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type AppVersionResponse struct {
	VersionCode  int    `json:"version_code"`
	VersionName  string `json:"version_name"`
	DownloadURL  string `json:"download_url"`
	ForceUpdate  bool   `json:"force_update"`
	ReleaseNotes string `json:"release_notes"`
}

// GetAppVersion mengembalikan info versi terbaru. Nilai default = production
// saat ini; override via env agar backend lokal bisa menonaktifkan
// force-update saat testing (APP_FORCE_UPDATE=false) tanpa ubah kode:
// APP_VERSION_CODE, APP_VERSION_NAME, APP_DOWNLOAD_URL,
// APP_FORCE_UPDATE ("true"/"false"), APP_RELEASE_NOTES.
func (h *APIHandler) GetAppVersion(c echo.Context) error {
	versionCode := 19
	if v, err := strconv.Atoi(os.Getenv("APP_VERSION_CODE")); err == nil && v > 0 {
		versionCode = v
	}
	versionName := os.Getenv("APP_VERSION_NAME")
	if versionName == "" {
		versionName = "1.2.8"
	}
	downloadURL := os.Getenv("APP_DOWNLOAD_URL")
	if downloadURL == "" {
		downloadURL = "https://api.controltowerslb.tech/uploads/apk/tower-control-latest.apk"
	}
	forceUpdate := true
	if v := os.Getenv("APP_FORCE_UPDATE"); v != "" {
		forceUpdate = v == "true" || v == "1"
	}
	releaseNotes := os.Getenv("APP_RELEASE_NOTES")
	if releaseNotes == "" {
		releaseNotes = "Update v1.2.8: Peningkatan akurasi live tracking armada, sinkronisasi rute aktif, dan peningkatan stabilitas aplikasi."
	}
	return c.JSON(http.StatusOK, AppVersionResponse{
		VersionCode:  versionCode,
		VersionName:  versionName,
		DownloadURL:  downloadURL,
		ForceUpdate:  forceUpdate,
		ReleaseNotes: releaseNotes,
	})
}

func (h *APIHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format request tidak valid")
	}

	identifier := req.Username
	if identifier == "" {
		identifier = req.Email
	}

	if identifier == "" || req.Password == "" {
		return response.Error(c, http.StatusBadRequest, "Username/email dan password wajib diisi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var idUser int
	var dbUsername, dbPassword, role string
	err := h.DB.QueryRow(ctx, `
		SELECT id_user, username, password, role 
		FROM users 
		WHERE LOWER(username) = LOWER($1)
		LIMIT 1
	`, identifier).Scan(&idUser, &dbUsername, &dbPassword, &role)

	if err != nil {
		return response.Error(c, http.StatusUnauthorized, "Username/email atau password salah")
	}

	// Cek password bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
	if err != nil {
		return response.Error(c, http.StatusUnauthorized, "Password salah")
	}

	// Cari id_driver yang sesuai dengan username driver
	var idDriver int64
	var namaDriver string
	_ = h.DB.QueryRow(ctx, `
		SELECT id_driver, nama_driver
		FROM driver
		WHERE LOWER(nama_driver) LIKE '%' || LOWER($1) || '%'
		ORDER BY id_driver ASC
		LIMIT 1
	`, dbUsername).Scan(&idDriver, &namaDriver)

	if namaDriver == "" {
		namaDriver = dbUsername
	}

	// Token dummy/JWT sederhana
	token := "token_driver_" + dbUsername

	return response.OK(c, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id_user":     idUser,
			"username":    dbUsername,
			"role":        role,
			"id_driver":   idDriver,
			"nama_driver": namaDriver,
			"name":        namaDriver,
		},
	})
}

func (h *APIHandler) GetSellers(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_seller, COALESCE(kode_seller, ''), nama_seller, COALESCE(alamat, ''), COALESCE(kota, ''), COALESCE(pic, ''), COALESCE(no_hp, '')
		FROM seller
		WHERE status = 'aktif'
		ORDER BY nama_seller ASC
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data seller")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id int
		var kode, nama, alamat, kota, pic, noHp string
		if err := rows.Scan(&id, &kode, &nama, &alamat, &kota, &pic, &noHp); err == nil {
			list = append(list, map[string]interface{}{
				"id":      id,
				"code":    kode,
				"name":    nama,
				"nama_seller": nama,
				"address": alamat,
				"city":    kota,
				"pic":     pic,
				"no_hp":   noHp,
			})
		}
	}

	return response.OK(c, list)
}

func (h *APIHandler) GetDrivers(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_driver, nama_driver, COALESCE(no_hp, ''), status_driver
		FROM driver
		ORDER BY id_driver ASC
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data driver")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id int
		var nama, noHp, status string
		if err := rows.Scan(&id, &nama, &noHp, &status); err == nil {
			list = append(list, map[string]interface{}{
				"id":     id,
				"name":   nama,
				"no_hp":  noHp,
				"status": status,
			})
		}
	}

	return response.OK(c, list)
}

func (h *APIHandler) GetVehicles(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, COALESCE(jenis_kendaraan, ''), COALESCE(kapasitas_kg, 0), status_kendaraan
		FROM kendaraan
		ORDER BY id_kendaraan ASC
	`)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data kendaraan")
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, kapasitasKg int
		var plat, jenis, status string
		if err := rows.Scan(&id, &plat, &jenis, &kapasitasKg, &status); err == nil {
			list = append(list, map[string]interface{}{
				"id":          id,
				"plat":        plat,
				"type":        jenis,
				"capacity_kg": kapasitasKg,
				"status":      status,
			})
		}
	}

	return response.OK(c, list)
}

