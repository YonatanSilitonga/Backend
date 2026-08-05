package mobile_api

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/pkg/response"
)

type APIHandler struct {
	DB *pgxpool.Pool
}

func NewAPIHandler(db *pgxpool.Pool) *APIHandler {
	return &APIHandler{DB: db}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
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
	if err != nil && dbPassword != req.Password {
		return response.Error(c, http.StatusUnauthorized, "Password salah")
	}

	// Token dummy/JWT sederhana
	token := "token_driver_" + dbUsername

	return response.OK(c, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id_user":  idUser,
			"username": dbUsername,
			"role":     role,
		},
	})
}

func (h *APIHandler) GetSellers(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT id_seller, COALESCE(kode_seller, ''), nama_seller, COALESCE(alamat, ''), COALESCE(kota, ''), COALESCE(pic, ''), COALESCE(no_hp, '')
		FROM seller
		ORDER BY id_seller ASC
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

type CreateTrackingRequest struct {
	IDRitase    int64   `json:"id_ritase"`
	IDKendaraan int64   `json:"id_kendaraan"`
	IDDriver    int64   `json:"id_driver"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Kecepatan   *int    `json:"kecepatan"`
	Arah        *int    `json:"arah"`
	Status      *string `json:"status"`
	JumlahKoli  int     `json:"jumlah_koli"`
}

func (h *APIHandler) PostTracking(c echo.Context) error {
	var req CreateTrackingRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid: "+err.Error())
	}

	var ritaseID interface{}
	if req.IDRitase != 0 {
		ritaseID = req.IDRitase
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	_, err := h.DB.Exec(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		ON CONFLICT (id_kendaraan)
		DO UPDATE SET id_driver   = EXCLUDED.id_driver,
		              latitude    = EXCLUDED.latitude,
		              longitude   = EXCLUDED.longitude,
		              kecepatan   = EXCLUDED.kecepatan,
		              arah        = EXCLUDED.arah,
		              status      = EXCLUDED.status,
		              last_update = now()
	`, ritaseID, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude, req.Kecepatan, req.Arah, req.Status)

	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan tracking: "+err.Error())
	}

	return response.OK(c, "success")
}
