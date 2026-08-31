package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Driver
	g.GET("/drivers", h.ListDriver)
	g.POST("/drivers", h.CreateDriver)
	g.PUT("/drivers/:id", h.UpdateDriver)
	g.DELETE("/drivers/:id", h.DeleteDriver)

	// Kendaraan
	g.GET("/vehicles", h.ListKendaraan)
	g.POST("/vehicles", h.CreateKendaraan)
	g.PUT("/vehicles/:id", h.UpdateKendaraan)
	g.DELETE("/vehicles/:id", h.DeleteKendaraan)

	// Seller
	g.GET("/sellers", h.ListSeller)
	g.POST("/sellers", h.CreateSeller)
	g.PUT("/sellers/:id", h.UpdateSeller)
	g.DELETE("/sellers/:id", h.DeleteSeller)

	// Gudang
	g.GET("/gudang", h.ListGudang)
	g.POST("/gudang", h.CreateGudang)
	g.PUT("/gudang/:id", h.UpdateGudang)
	g.DELETE("/gudang/:id", h.DeleteGudang)

	// Drop Point
	g.GET("/drop-points", h.ListDropPoint)
	g.POST("/drop-points", h.CreateDropPoint)
	g.PUT("/drop-points/:id", h.UpdateDropPoint)
	g.DELETE("/drop-points/:id", h.DeleteDropPoint)

	// User
	g.GET("/users", h.ListUser)
	g.POST("/users", h.CreateUser)
	g.PUT("/users/:id/role", h.UpdateUserRole)
	g.PUT("/users/:id/status", h.UpdateUserStatus)
	g.POST("/users/:id/reset-password", h.ResetPassword)
	g.DELETE("/users/:id", h.DeleteUser)
}

// ──────── Driver ────────

func (h *Handler) ListDriver(c echo.Context) error {
	data, err := h.svc.ListDriver(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data driver")
	}
	return response.OK(c, data)
}

func (h *Handler) CreateDriver(c echo.Context) error {
	var req DriverRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.NamaDriver == "" || req.StatusDriver == "" {
		return response.Error(c, http.StatusBadRequest, "nama_driver dan status_driver wajib diisi")
	}
	id, err := h.svc.CreateDriver(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat driver")
	}
	return response.Created(c, map[string]any{"id_driver": id})
}

func (h *Handler) UpdateDriver(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req DriverRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if err := h.svc.UpdateDriver(c.Request().Context(), id, req); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update driver")
	}
	return response.OK(c, map[string]string{"message": "driver diperbarui"})
}

func (h *Handler) DeleteDriver(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.DeleteDriver(c.Request().Context(), id); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menonaktifkan driver")
	}
	return response.OK(c, map[string]string{"message": "driver dinonaktifkan"})
}

// ──────── Kendaraan ────────

func (h *Handler) ListKendaraan(c echo.Context) error {
	data, err := h.svc.ListKendaraan(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data kendaraan")
	}
	return response.OK(c, data)
}

func (h *Handler) CreateKendaraan(c echo.Context) error {
	var req KendaraanRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.PlatNomor == "" || req.StatusKendaraan == "" {
		return response.Error(c, http.StatusBadRequest, "plat_nomor dan status_kendaraan wajib diisi")
	}
	id, err := h.svc.CreateKendaraan(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat kendaraan")
	}
	return response.Created(c, map[string]any{"id_kendaraan": id})
}

func (h *Handler) UpdateKendaraan(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req KendaraanRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if err := h.svc.UpdateKendaraan(c.Request().Context(), id, req); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update kendaraan")
	}
	return response.OK(c, map[string]string{"message": "kendaraan diperbarui"})
}

func (h *Handler) DeleteKendaraan(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.DeleteKendaraan(c.Request().Context(), id); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menonaktifkan kendaraan")
	}
	return response.OK(c, map[string]string{"message": "kendaraan dinonaktifkan"})
}

// ──────── Seller ────────

func (h *Handler) ListSeller(c echo.Context) error {
	data, err := h.svc.ListSeller(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data seller")
	}
	return response.OK(c, data)
}

func (h *Handler) CreateSeller(c echo.Context) error {
	var req SellerRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.KodeSeller == "" || req.NamaSeller == "" {
		return response.Error(c, http.StatusBadRequest, "kode_seller dan nama_seller wajib diisi")
	}
	if req.Status == "" {
		req.Status = "aktif"
	}
	id, err := h.svc.CreateSeller(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat seller")
	}
	return response.Created(c, map[string]any{"id_seller": id})
}

func (h *Handler) UpdateSeller(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req SellerRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if err := h.svc.UpdateSeller(c.Request().Context(), id, req); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update seller")
	}
	return response.OK(c, map[string]string{"message": "seller diperbarui"})
}

func (h *Handler) DeleteSeller(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.DeleteSeller(c.Request().Context(), id); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menonaktifkan seller")
	}
	return response.OK(c, map[string]string{"message": "seller dinonaktifkan"})
}

// ──────── Gudang ────────

func (h *Handler) ListGudang(c echo.Context) error {
	data, err := h.svc.ListGudang(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data gudang")
	}
	return response.OK(c, data)
}

func (h *Handler) CreateGudang(c echo.Context) error {
	var req GudangRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.NamaGudang == "" {
		return response.Error(c, http.StatusBadRequest, "nama_gudang wajib diisi")
	}
	if req.Status == "" {
		req.Status = "aktif"
	}
	id, err := h.svc.CreateGudang(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat gudang")
	}
	return response.Created(c, map[string]any{"id_gudang": id})
}

func (h *Handler) UpdateGudang(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req GudangRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if err := h.svc.UpdateGudang(c.Request().Context(), id, req); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update gudang")
	}
	return response.OK(c, map[string]string{"message": "gudang diperbarui"})
}

func (h *Handler) DeleteGudang(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.DeleteGudang(c.Request().Context(), id); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menonaktifkan gudang")
	}
	return response.OK(c, map[string]string{"message": "gudang dinonaktifkan"})
}

// ──────── DropPoint ────────

func (h *Handler) ListDropPoint(c echo.Context) error {
	data, err := h.svc.ListDropPoint(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data drop point")
	}
	return response.OK(c, data)
}

func (h *Handler) CreateDropPoint(c echo.Context) error {
	var req DropPointRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.NamaDropPoint == "" {
		return response.Error(c, http.StatusBadRequest, "nama_drop_point wajib diisi")
	}
	if req.Status == "" {
		req.Status = "aktif"
	}
	id, err := h.svc.CreateDropPoint(c.Request().Context(), req)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat drop point")
	}
	return response.Created(c, map[string]any{"id_drop_point": id})
}

func (h *Handler) UpdateDropPoint(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req DropPointRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if err := h.svc.UpdateDropPoint(c.Request().Context(), id, req); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update drop point")
	}
	return response.OK(c, map[string]string{"message": "drop point diperbarui"})
}

func (h *Handler) DeleteDropPoint(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.DeleteDropPoint(c.Request().Context(), id); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menonaktifkan drop point")
	}
	return response.OK(c, map[string]string{"message": "drop point dinonaktifkan"})
}

// ──────── User ────────

func (h *Handler) ListUser(c echo.Context) error {
	data, err := h.svc.ListUser(c.Request().Context())
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data user")
	}
	return response.OK(c, data)
}

func (h *Handler) CreateUser(c echo.Context) error {
	var req UserRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.Username == "" || req.Password == "" || req.Role == "" {
		return response.Error(c, http.StatusBadRequest, "username, password, dan role wajib diisi")
	}
	id, err := h.svc.CreateUser(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, ErrUsernameExists) {
			return response.Error(c, http.StatusConflict, "username sudah digunakan")
		}
		return response.Error(c, http.StatusInternalServerError, "gagal membuat user")
	}
	return response.Created(c, map[string]any{"id_user": id})
}

func (h *Handler) UpdateUserRole(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil || req.Role == "" {
		return response.Error(c, http.StatusBadRequest, "role wajib diisi")
	}
	if err := h.svc.UpdateUserRole(c.Request().Context(), id, req.Role); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update role")
	}
	return response.OK(c, map[string]string{"message": "role diperbarui"})
}

func (h *Handler) UpdateUserStatus(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil || req.Status == "" {
		return response.Error(c, http.StatusBadRequest, "status wajib diisi")
	}
	if req.Status != "aktif" && req.Status != "nonaktif" {
		return response.Error(c, http.StatusBadRequest, "status harus 'aktif' atau 'nonaktif'")
	}
	if err := h.svc.UpdateUserStatus(c.Request().Context(), id, req.Status); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal update status")
	}
	return response.OK(c, map[string]string{"message": "status diperbarui"})
}

func (h *Handler) ResetPassword(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil || req.NewPassword == "" {
		return response.Error(c, http.StatusBadRequest, "new_password wajib diisi")
	}
	if len(req.NewPassword) < 6 {
		return response.Error(c, http.StatusBadRequest, "password minimal 6 karakter")
	}
	if err := h.svc.ResetPassword(c.Request().Context(), id, req.NewPassword); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal reset password")
	}
	return response.OK(c, map[string]string{"message": "password berhasil direset"})
}

func (h *Handler) DeleteUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "id tidak valid")
	}
	if err := h.svc.DeleteUser(c.Request().Context(), id); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menghapus user")
	}
	return response.OK(c, map[string]string{"message": "user dihapus"})
}
