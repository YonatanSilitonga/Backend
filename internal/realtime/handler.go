package realtime

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler menyediakan endpoint SSE untuk web dashboard.
type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// Stream menangani GET /realtime/live — Server-Sent Events.
// Setiap tick hub, client menerima `data: {json}\n\n`. Client baru langsung
// dapat snapshot terakhir (Last) tanpa nunggu tick berikutnya.
func (h *Handler) Stream(c echo.Context) error {
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no") // matikan buffering reverse-proxy

	flush := func() {
		if f, ok := res.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Snapshot terakhir dulu biar client langsung punya data.
	if last := h.hub.Last(); len(last) > 0 {
		if _, err := res.Write([]byte("data: " + string(last) + "\n\n")); err != nil {
			return nil
		}
		flush()
	}

	ch := h.hub.Subscribe()
	defer h.hub.Unsubscribe(ch)

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case payload := <-ch:
			if _, err := res.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
				return nil
			}
			flush()
		}
	}
}
