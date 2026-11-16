package dropper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
	"backend/internal/pkg/timeutil"
)

// Handler menangani input muatan dropper (seller yang drop langsung ke gudang).
type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

// RegisterRoutes memasang route dropper (butuh JWT + role).
func (h *Handler) RegisterRoutes(g *echo.Group, authMW, roleMW echo.MiddlewareFunc) {
	dg := g.Group("/dropper", authMW, roleMW)
	dg.POST("/input", h.PostInput)
	dg.GET("/entries", h.GetEntries)
}

type InputItem struct {
	IDSeller    int64  `json:"id_seller"`
	RitaseKe    int    `json:"ritase_ke"`
	JumlahAWB   int    `json:"jumlah_awb"`
	JumlahAWBHV int    `json:"jumlah_awb_hv"`
	Catatan     string `json:"catatan"`
}

type InputRequest struct {
	JenisRitase string      `json:"jenis_ritase"`
	Items       []InputItem `json:"items"`
}

type Entry struct {
	ID          int64     `json:"id"`
	IDSeller    int64     `json:"id_seller"`
	KodeSeller  string    `json:"kode_seller"`
	NamaSeller  string    `json:"nama_seller"`
	JenisRitase string    `json:"jenis_ritase"`
	RitaseKe    int       `json:"ritase_ke"`
	JumlahAWB   int       `json:"jumlah_awb"`
	JumlahAWBHV int       `json:"jumlah_awb_hv"`
	Catatan     string    `json:"catatan"`
	Tanggal     string    `json:"tanggal"`
	CreatedAt   time.Time `json:"created_at"`
}

// PostInput menangani POST /dropper/input.
func (h *Handler) PostInput(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}

	var req InputRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if len(req.Items) == 0 {
		return response.Error(c, http.StatusBadRequest, "minimal satu seller dropper harus diisi")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi")
	}
	defer tx.Rollback(ctx)

	createdIDs := make([]int64, 0, len(req.Items))
	for i, item := range req.Items {
		if item.IDSeller <= 0 {
			return response.Error(c, http.StatusBadRequest, fmt.Sprintf("seller baris %d tidak valid", i+1))
		}
		if item.RitaseKe < 1 || item.RitaseKe > 3 {
			return response.Error(c, http.StatusBadRequest, fmt.Sprintf("ritase_ke baris %d harus 1-3", i+1))
		}
		if item.JumlahAWB < 0 || item.JumlahAWBHV < 0 {
			return response.Error(c, http.StatusBadRequest, fmt.Sprintf("jumlah AWB baris %d tidak boleh negatif", i+1))
		}
		if item.JumlahAWB+item.JumlahAWBHV <= 0 {
			return response.Error(c, http.StatusBadRequest, fmt.Sprintf("jumlah AWB baris %d harus lebih dari 0", i+1))
		}

		var kode, nama string
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(kode_seller, ''), nama_seller
			FROM seller
			WHERE id_seller = $1
			  AND status = 'aktif'
			  AND COALESCE(is_dropper, FALSE) = TRUE
		`, item.IDSeller).Scan(&kode, &nama)
		if err != nil {
			if err == pgx.ErrNoRows {
				return response.Error(c, http.StatusBadRequest, fmt.Sprintf("seller baris %d bukan dropper aktif", i+1))
			}
			return response.Error(c, http.StatusInternalServerError, "gagal memvalidasi seller dropper")
		}

		catatan := strings.TrimSpace(item.Catatan)
		var id int64
		err = tx.QueryRow(ctx, `
			INSERT INTO dropper_input
				(id_seller, nama_seller, jenis_ritase, ritase_ke, jumlah_awb, jumlah_awb_hv, catatan, tanggal, id_user)
			VALUES
				($1, $2, $3, $4, $5, $6, NULLIF($7, ''), (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE, $8)
			RETURNING id
		`, item.IDSeller, nama, req.JenisRitase, item.RitaseKe, item.JumlahAWB, item.JumlahAWBHV, catatan, userID).Scan(&id)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal menyimpan input dropper")
		}
		createdIDs = append(createdIDs, id)
	}

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan input dropper")
	}

	return response.Created(c, map[string]any{
		"count": len(createdIDs),
		"ids":   createdIDs,
	})
}

// GetEntries menangani GET /dropper/entries?tanggal=YYYY-MM-DD.
func (h *Handler) GetEntries(c echo.Context) error {
	tanggal := strings.TrimSpace(c.QueryParam("tanggal"))
	if tanggal == "" {
		tanggal = timeutil.NowWIB().Format("2006-01-02")
	}
	if tanggal != "all" {
		if _, err := time.Parse("2006-01-02", tanggal); err != nil {
			return response.Error(c, http.StatusBadRequest, "format tanggal harus YYYY-MM-DD")
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	query := `
		SELECT d.id, d.id_seller, COALESCE(s.kode_seller, ''), d.nama_seller,
		       d.jenis_ritase, d.ritase_ke, d.jumlah_awb, d.jumlah_awb_hv,
		       COALESCE(d.catatan, ''), TO_CHAR(d.tanggal, 'YYYY-MM-DD'), d.created_at
		FROM dropper_input d
		LEFT JOIN seller s ON s.id_seller = d.id_seller
		WHERE 1=1`
	args := make([]any, 0, 1)
	if tanggal != "all" {
		args = append(args, tanggal)
		query += fmt.Sprintf(" AND d.tanggal = $%d", len(args))
	}
	query += " ORDER BY d.created_at DESC, d.id DESC"

	rows, err := h.DB.Query(ctx, query, args...)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data dropper")
	}
	defer rows.Close()

	items := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.IDSeller, &e.KodeSeller, &e.NamaSeller,
			&e.JenisRitase, &e.RitaseKe, &e.JumlahAWB, &e.JumlahAWBHV,
			&e.Catatan, &e.Tanggal, &e.CreatedAt); err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal membaca data dropper")
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membaca data dropper")
	}

	return response.OK(c, items)
}
