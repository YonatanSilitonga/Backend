package utilisasi

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
	"backend/internal/pkg/timeutil"
)

type Handler struct {
	repo *Repository
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{repo: NewRepository(db)}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMW, readMW, writeMW echo.MiddlewareFunc) {
	ug := g.Group("/utilisasi-muatan", authMW, readMW)
	ug.GET("", h.GetData)
	ug.GET("/summary", h.GetEffectiveness)

	dg := g.Group("/day-categories", authMW, writeMW)
	dg.GET("", h.ListDayCategories)
	dg.PUT("/:tanggal", h.UpsertDayCategory)
	dg.DELETE("/:tanggal", h.DeleteDayCategory)
}

func defaultRange() (time.Time, time.Time) {
	end := timeutil.NowWIB()
	start := end.AddDate(0, 0, -29)
	return start, end
}

func parseRange(c echo.Context) (time.Time, time.Time, error) {
	startStr := strings.TrimSpace(c.QueryParam("start_date"))
	endStr := strings.TrimSpace(c.QueryParam("end_date"))
	if startStr == "" && endStr == "" {
		start, end := defaultRange()
		return start, end, nil
	}

	var start, end time.Time
	var err error
	if startStr == "" || endStr == "" {
		return start, end, errInvalidRange
	}
	if start, err = parseDate(startStr); err != nil {
		return start, end, errInvalidRange
	}
	if end, err = parseDate(endStr); err != nil {
		return start, end, errInvalidRange
	}
	if end.Before(start) {
		return start, end, errInvalidRange
	}
	return start, end, nil
}

var errInvalidRange = echo.NewHTTPError(http.StatusBadRequest, "rentang tanggal tidak valid")

func (h *Handler) GetData(c echo.Context) error {
	start, end, err := parseRange(c)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD dan end >= start")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	data, err := h.repo.GetData(ctx, start, end)
	if err != nil {
		log.Printf("[UTILISASI] gagal mengambil data: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data utilisasi muatan")
	}
	return response.OK(c, data)
}

func (h *Handler) GetEffectiveness(c echo.Context) error {
	start, end, err := parseRange(c)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD dan end >= start")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()

	data, err := h.repo.GetEffectiveness(ctx, start, end)
	if err != nil {
		log.Printf("[UTILISASI] gagal mengambil ringkasan efektivitas: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil ringkasan efektivitas armada")
	}
	return response.OK(c, data)
}

func (h *Handler) ListDayCategories(c echo.Context) error {
	start, end, err := parseRange(c)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD dan end >= start")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	items, err := h.repo.EffectiveCategories(ctx, start, end)
	if err != nil {
		log.Printf("[UTILISASI] gagal mengambil kategori hari: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil kategori hari")
	}
	return response.OK(c, map[string]any{
		"start_date": formatDate(start),
		"end_date":   formatDate(end),
		"items":      items,
	})
}

type dayCategoryRequest struct {
	Kategori   string `json:"kategori"`
	Keterangan string `json:"keterangan"`
}

func (h *Handler) UpsertDayCategory(c echo.Context) error {
	tanggal := strings.TrimSpace(c.Param("tanggal"))
	if _, err := parseDate(tanggal); err != nil {
		return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD")
	}

	var req dayCategoryRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "body tidak valid")
	}
	req.Kategori = strings.ToLower(strings.TrimSpace(req.Kategori))
	switch Category(req.Kategori) {
	case CategoryPeak, CategoryNormal, CategoryLow:
	default:
		return response.Error(c, http.StatusBadRequest, "kategori harus peak, normal, atau low")
	}

	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	if err := h.repo.UpsertOverride(ctx, tanggal, req.Kategori, strings.TrimSpace(req.Keterangan), userID); err != nil {
		log.Printf("[UTILISASI] gagal menyimpan kategori hari: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan kategori hari")
	}
	return response.OK(c, map[string]any{
		"tanggal":  tanggal,
		"kategori": req.Kategori,
	})
}

func (h *Handler) DeleteDayCategory(c echo.Context) error {
	tanggal := strings.TrimSpace(c.Param("tanggal"))
	if _, err := parseDate(tanggal); err != nil {
		return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	if err := h.repo.DeleteOverride(ctx, tanggal); err != nil {
		log.Printf("[UTILISASI] gagal menghapus kategori hari: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menghapus kategori hari")
	}
	return response.OK(c, map[string]any{"tanggal": tanggal, "deleted": true})
}
