package realtime

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"backend/internal/eventbus"
)

// Provider menghasilkan snapshot data live untuk di-broadcast ke semua client SSE.
// Diimplementasikan di main.go (menggabungkan dashboard + armada service).
type Provider interface {
	GetSnapshot(ctx context.Context) (map[string]any, error)
}

// Hub adalah engine broadcast: 1 goroutine ticker query DB sekali, lalu push
// payload yang sama ke semua subscriber (biar gak dobel query per browser).
type Hub struct {
	mu       sync.RWMutex
	subs     map[chan []byte]struct{}
	provider Provider
	interval time.Duration
	bus      *eventbus.Bus

	lastMu      sync.RWMutex
	lastPayload []byte

	broadcasting int32
}

func NewHub(provider Provider, interval time.Duration, bus *eventbus.Bus) *Hub {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &Hub{
		subs:     make(map[chan []byte]struct{}),
		provider: provider,
		interval: interval,
		bus:      bus,
	}
}

// Run menjalankan loop broadcast sampai ctx di-cancel.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Mulai listener untuk trigger manual dari modul lain
	go h.listen(ctx)

	// Kirim snapshot pertama segera (tanpa nunggu tick pertama).
	h.broadcast(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.broadcast(ctx)
		}
	}
}

// listen "mendengarkan" event dari bus dan mentrigger broadcast.
func (h *Hub) listen(ctx context.Context) {
	forceRefresh := h.bus.Subscribe("force_refresh")
	log.Println("[REALTIME] listening for force_refresh events")

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-forceRefresh:
			log.Printf("[REALTIME] received force_refresh event: %s", msg)
			h.broadcast(ctx)
		}
	}
}

func (h *Hub) broadcast(ctx context.Context) {
	// Kalau ada broadcast lain yang masih jalan, skip — jangan numpuk query DB.
	if !atomic.CompareAndSwapInt32(&h.broadcasting, 0, 1) {
		log.Println("[REALTIME] broadcast sebelumnya masih jalan, skip tick ini")
		return
	}
	defer atomic.StoreInt32(&h.broadcasting, 0)

	snapshot, err := h.provider.GetSnapshot(ctx)
	if err != nil {
		log.Printf("[REALTIME] gagal ambil snapshot: %v", err)
		return
	}

	payload, err := json.Marshal(map[string]any{
		"type": "live",
		"ts":   time.Now().Format(time.RFC3339),
		"data": snapshot,
	})
	if err != nil {
		log.Printf("[REALTIME] gagal marshal payload: %v", err)
		return
	}

	h.lastMu.Lock()
	h.lastPayload = payload
	h.lastMu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- payload:
		default:
			// Channel penuh — client lambat; drop tick ini, client akan dapat
			// tick berikutnya (atau reconnect). Hindari blokir broadcast.
		}
	}
}

// Last mengembalikan payload snapshot terakhir (buat client yang baru connect).
func (h *Hub) Last() []byte {
	h.lastMu.RLock()
	defer h.lastMu.RUnlock()
	return h.lastPayload
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 4)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}
