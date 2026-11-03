package kapten

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"backend/internal/eventbus"
	appJWT "backend/internal/pkg/jwt"
)

type Handler struct {
	DB  *pgxpool.Pool
	bus *eventbus.Bus
	jwt *appJWT.Manager
}

func NewHandler(db *pgxpool.Pool, bus *eventbus.Bus, jwtMgr *appJWT.Manager) *Handler {
	return &Handler{DB: db, bus: bus, jwt: jwtMgr}
}

// RegisterRoutes memasang route kapten di grup /api/v1.
func (h *Handler) RegisterRoutes(g *echo.Group, authMW echo.MiddlewareFunc) {
	kapten := g.Group("/kapten", authMW)
	kapten.GET("/my-seller-ritase", h.GetMySellerRitase)
	kapten.POST("/cargo-input", h.PostCargoInput)
	kapten.GET("/seller-info", h.GetSellerInfo)
	kapten.GET("/implant-info", h.GetImplantInfo)
	kapten.GET("/today-cargo", h.GetTodayCargo)
	kapten.GET("/my-sellers", h.GetMySellers)
	kapten.POST("/select-seller", h.SelectSeller)
	kapten.GET("/my-implants", h.GetMyImplants)
	kapten.POST("/select-implant", h.SelectImplant)
	kapten.POST("/confirm-pickup", h.ConfirmPickup)
	kapten.GET("/pending-confirmations", h.GetPendingConfirmations)
	kapten.GET("/konfirmasi-penjemputan", h.GetKonfirmasiPenjemputan)
	kapten.GET("/batch-overview", h.GetBatchOverview)
	kapten.GET("/batch-ritase", h.GetBatchRitase)
	kapten.POST("/confirm-pickup-batch", h.PostBatchConfirmPickup)
	kapten.GET("/konfirmasi-history", h.GetKonfirmasiHistory)
}
