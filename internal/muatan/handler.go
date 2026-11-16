package muatan

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
	"backend/internal/pkg/timeutil"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

func (h *Handler) RegisterRoutes(g *echo.Group, authMW, roleMW echo.MiddlewareFunc) {
	mg := g.Group("/muatan", authMW, roleMW)
	mg.GET("/summary", h.GetSummary)
}

type RitSummary struct {
	RitaseKe            int `json:"ritase_ke"`
	ImplanAWB           int `json:"implan_awb"`
	ImplanHV            int `json:"implan_hv"`
	ImplanEntries       int `json:"implan_entries"`
	DriverPickupAWB     int `json:"driver_pickup_awb"`
	DriverPickupHV      int `json:"driver_pickup_hv"`
	DriverPickupEntries int `json:"driver_pickup_entries"`
	DropperAWB          int `json:"dropper_awb"`
	DropperHV           int `json:"dropper_hv"`
	DropperEntries      int `json:"dropper_entries"`
	TotalAWB            int `json:"total_awb"`
	TotalHV             int `json:"total_hv"`
	TotalEntries        int `json:"total_entries"`
}

type SummaryResponse struct {
	Tanggal      string       `json:"tanggal"`
	Rits         []RitSummary `json:"rits"`
	TotalAWB     int          `json:"total_awb"`
	TotalHV      int          `json:"total_hv"`
	TotalEntries int          `json:"total_entries"`
}

func (h *Handler) GetSummary(c echo.Context) error {
	tanggal := strings.TrimSpace(c.QueryParam("tanggal"))
	if tanggal == "" {
		tanggal = timeutil.NowWIB().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", tanggal); err != nil {
		return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	query := `
		WITH implan AS (
			SELECT t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(
			           COALESCE(t.koli_hv_jkt, 0) + COALESCE(t.koli_hv_seg, 0) + COALESCE(t.koli_hv_btn, 0) +
			           COALESCE(t.ecer_hv_jkt, 0) + COALESCE(t.ecer_hv_seg, 0) + COALESCE(t.ecer_hv_btn, 0) +
			           COALESCE(t.awb_hv_jkt, 0) + COALESCE(t.awb_hv_seg, 0) + COALESCE(t.awb_hv_btn, 0)
			       ), 0) AS hv,
			       COUNT(*) AS entries
			FROM input_kapten t
			WHERE (t.created_at + interval '7 hours')::date = $1::date
			  AND COALESCE(t.jenis_ritase, 'outgoing') = 'outgoing'
			  AND t.id_gudang IS NULL
			  AND (t.id_implant IS NOT NULL OR t.id_seller IS NOT NULL)
			  AND t.ritase_ke BETWEEN 1 AND 3
			GROUP BY t.ritase_ke
		),
		pickup AS (
			SELECT CASE
			           WHEN COALESCE(t.created_at, t.updated_at) IS NULL THEN 1
			           WHEN EXTRACT(HOUR FROM (COALESCE(t.created_at, t.updated_at) + interval '7 hours')) < 16 THEN 1
			           WHEN EXTRACT(HOUR FROM (COALESCE(t.created_at, t.updated_at) + interval '7 hours')) < 20 THEN 2
			           ELSE 3
			       END AS ritase_ke,
			       COALESCE(SUM(t.jumlah_barang), 0) AS awb,
			       COALESCE(SUM(t.high_value), 0) AS hv,
			       COUNT(*) AS entries
			FROM driver_pickup_log t
			WHERE t.tanggal = $1::date
			GROUP BY 1
		),
		dropper AS (
			SELECT t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.jumlah_awb_hv), 0) AS hv,
			       COUNT(*) AS entries
			FROM dropper_input t
			WHERE t.tanggal = $1::date
			  AND COALESCE(t.jenis_ritase, 'outgoing') = 'outgoing'
			  AND t.ritase_ke BETWEEN 1 AND 3
			GROUP BY t.ritase_ke
		)
		SELECT r.ritase_ke,
		       COALESCE(i.awb, 0), COALESCE(i.hv, 0), COALESCE(i.entries, 0),
		       COALESCE(p.awb, 0), COALESCE(p.hv, 0), COALESCE(p.entries, 0),
		       COALESCE(d.awb, 0), COALESCE(d.hv, 0), COALESCE(d.entries, 0)
		FROM generate_series(1, 3) AS r(ritase_ke)
		LEFT JOIN implan i ON i.ritase_ke = r.ritase_ke
		LEFT JOIN pickup p ON p.ritase_ke = r.ritase_ke
		LEFT JOIN dropper d ON d.ritase_ke = r.ritase_ke
		ORDER BY r.ritase_ke
	`

	rows, err := h.DB.Query(ctx, query, tanggal)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil rekap total muatan")
	}
	defer rows.Close()

	result := SummaryResponse{
		Tanggal: tanggal,
		Rits:    make([]RitSummary, 0, 3),
	}
	for rows.Next() {
		var r RitSummary
		if err := rows.Scan(
			&r.RitaseKe,
			&r.ImplanAWB, &r.ImplanHV, &r.ImplanEntries,
			&r.DriverPickupAWB, &r.DriverPickupHV, &r.DriverPickupEntries,
			&r.DropperAWB, &r.DropperHV, &r.DropperEntries,
		); err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal membaca rekap total muatan")
		}
		r.TotalAWB = r.ImplanAWB + r.DriverPickupAWB + r.DropperAWB
		r.TotalHV = r.ImplanHV + r.DriverPickupHV + r.DropperHV
		r.TotalEntries = r.ImplanEntries + r.DriverPickupEntries + r.DropperEntries
		result.TotalAWB += r.TotalAWB
		result.TotalHV += r.TotalHV
		result.TotalEntries += r.TotalEntries
		result.Rits = append(result.Rits, r)
	}
	if err := rows.Err(); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membaca rekap total muatan")
	}

	return response.OK(c, result)
}
