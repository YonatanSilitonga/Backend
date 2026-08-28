package mobile_api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

// ScheduleItem merepresentasikan satu ritase dalam jadwal driver hari ini.
type ScheduleItem struct {
	IDRitase       int64   `json:"id_ritase"`
	KodeRitase     string  `json:"kode_ritase"`
	Status         string  `json:"status"`
	RitaseKe       int     `json:"ritase_ke"`
	JenisRitase    *string `json:"jenis_ritase"`
	JamMulai       *string `json:"jam_mulai"`
	JamSelesai     *string `json:"jam_selesai"`
	PlatNomor      string  `json:"plat_nomor"`
	JenisKendaraan string  `json:"jenis_kendaraan"`
	TotalStop      int     `json:"total_stop"`
	StopSelesai    int     `json:"stop_selesai"`
	TotalKoli      int     `json:"total_koli"`
	TotalEcer      int     `json:"total_ecer"`
	TotalHV        int     `json:"total_high_value"`
	DurasiDetik    int     `json:"durasi_detik"`
	StartedAt      *string `json:"started_at"`
	FinishedAt     *string `json:"finished_at"`
	IsLate         bool    `json:"is_late"`
	LateMinutes    int     `json:"late_minutes"`
}

// GetMySchedules mengembalikan daftar semua ritase driver hari ini (aktif + selesai + upcoming).
// GET /api/v1/driver/my-schedules
func (h *APIHandler) GetMySchedules(c echo.Context) error {
	driverID, ok := c.Get(middleware.CtxDriverID).(int64)
	if !ok || driverID == 0 {
		return response.Error(c, http.StatusUnauthorized, "token tidak memuat id_driver yang valid")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 8*time.Second)
	defer cancel()

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	hariIni := time.Now().In(loc).Format("2006-01-02")

	// Ambil semua ritase driver hari ini
	rows, err := h.DB.Query(ctx, `
		SELECT
			r.id_ritase, r.kode_ritase, r.status, r.ritase_ke,
			r.jenis_ritase,
			TO_CHAR(r.jam_mulai, 'HH24:MI:SS') AS jam_mulai,
			TO_CHAR(r.jam_selesai, 'HH24:MI:SS') AS jam_selesai,
			COALESCE(k.plat_nomor, '') AS plat_nomor,
			COALESCE(k.jenis_kendaraan, '') AS jenis_kendaraan,
			(SELECT COUNT(*) FROM ritase_stop WHERE id_ritase = r.id_ritase) AS total_stop,
			(SELECT COUNT(*) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Tiba') AS stop_selesai,
			COALESCE((SELECT SUM(jumlah_koli) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Bongkar Muat Barang'), 0) AS total_koli,
			COALESCE((SELECT SUM(jumlah_ecer) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Bongkar Muat Barang'), 0) AS total_ecer,
			COALESCE((SELECT SUM(jumlah_high_value) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Bongkar Muat Barang'), 0) AS total_hv,
			COALESCE((SELECT SUM(durasi_detik) FROM ritase_event WHERE id_ritase = r.id_ritase), 0) AS durasi_detik,
			(SELECT MIN(created_at) FROM ritase_event WHERE id_ritase = r.id_ritase) AS started_at,
			CASE WHEN r.status = 'selesai'
				THEN (SELECT MAX(created_at) FROM ritase_event WHERE id_ritase = r.id_ritase)
				ELSE NULL
			END AS finished_at
		FROM ritase r
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.id_driver = $1 AND r.tanggal = $2
		ORDER BY r.ritase_ke ASC, r.id_ritase ASC
	`, driverID, hariIni)

	if err != nil {
		log.Printf("[DRIVER SCHEDULES] query error: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil jadwal")
	}
	defer rows.Close()

	var schedules []ScheduleItem
	now := time.Now().In(loc)
	for rows.Next() {
		var s ScheduleItem
		if err := rows.Scan(
			&s.IDRitase, &s.KodeRitase, &s.Status, &s.RitaseKe,
			&s.JenisRitase, &s.JamMulai, &s.JamSelesai,
			&s.PlatNomor, &s.JenisKendaraan,
			&s.TotalStop, &s.StopSelesai,
			&s.TotalKoli, &s.TotalEcer, &s.TotalHV,
			&s.DurasiDetik, &s.StartedAt, &s.FinishedAt,
		); err == nil {
			// Hitung keterlambatan: kalau status bukan selesai dan jam_mulai sudah lewat
			if s.Status != "selesai" && s.JamMulai != nil {
				parts := splitTime(*s.JamMulai)
				if len(parts) >= 2 {
					hour, _ := strconv.Atoi(parts[0])
					min, _ := strconv.Atoi(parts[1])
					scheduleTime := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
					if now.After(scheduleTime) {
						s.IsLate = true
						s.LateMinutes = int(now.Sub(scheduleTime).Minutes())
					}
				}
			}
			schedules = append(schedules, s)
		}
	}

	if schedules == nil {
		schedules = []ScheduleItem{}
	}

	return response.OK(c, map[string]interface{}{
		"date":    hariIni,
		"driver":  driverID,
		"total":   len(schedules),
		"schedules": schedules,
	})
}

// splitTime memecah string waktu "HH:MM:SS" atau "HH:MM" menjadi slice.
func splitTime(t string) []string {
	return strings.Split(t, ":")
}

// HistoryItem merepresentasikan satu ritase dari riwayat driver.
type HistoryItem struct {
	IDRitase       int64   `json:"id_ritase"`
	KodeRitase     string  `json:"kode_ritase"`
	Status         string  `json:"status"`
	RitaseKe       int     `json:"ritase_ke"`
	JenisRitase    *string `json:"jenis_ritase"`
	Tanggal        *string `json:"tanggal"`
	JamMulai       *string `json:"jam_mulai"`
	JamSelesai     *string `json:"jam_selesai"`
	PlatNomor      string  `json:"plat_nomor"`
	JenisKendaraan string  `json:"jenis_kendaraan"`
	TotalStop      int     `json:"total_stop"`
	StopSelesai    int     `json:"stop_selesai"`
	TotalKoli      int     `json:"total_koli"`
	TotalEcer      int     `json:"total_ecer"`
	TotalHV        int     `json:"total_high_value"`
	DurasiDetik    int     `json:"durasi_detik"`
	StartedAt      *string `json:"started_at"`
	FinishedAt     *string `json:"finished_at"`
	Events         []EventItem `json:"events,omitempty"`
}

// EventItem satu event status dalam ritase.
type EventItem struct {
	IDEvent     int64   `json:"id_event"`
	Status      string  `json:"status"`
	NamaLokasi  *string `json:"nama_lokasi"`
	DurasiDetik int     `json:"durasi_detik"`
	Koli        int     `json:"jumlah_koli"`
	Ecer        int     `json:"jumlah_ecer"`
	HV          int     `json:"jumlah_high_value"`
	CreatedAt   string  `json:"created_at"`
}

// GetHistory mengembalikan riwayat ritase driver (7 hari terakhir atau date range).
// GET /api/v1/driver/history?days=7
func (h *APIHandler) GetHistory(c echo.Context) error {
	driverID, ok := c.Get(middleware.CtxDriverID).(int64)
	if !ok || driverID == 0 {
		return response.Error(c, http.StatusUnauthorized, "token tidak memuat id_driver yang valid")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 8*time.Second)
	defer cancel()

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}

	// Default 7 hari terakhir, bisa override via ?days=N
	days := 7
	if d := c.QueryParam("days"); d != "" {
		parsed := 0
		for _, ch := range d {
			if ch >= '0' && ch <= '9' {
				parsed = parsed*10 + int(ch-'0')
			}
		}
		if parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	since := time.Now().In(loc).AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := h.DB.Query(ctx, `
		SELECT
			r.id_ritase, r.kode_ritase, r.status, r.ritase_ke,
			r.jenis_ritase,
			TO_CHAR(r.tanggal, 'YYYY-MM-DD') AS tanggal,
			TO_CHAR(r.jam_mulai, 'HH24:MI:SS') AS jam_mulai,
			TO_CHAR(r.jam_selesai, 'HH24:MI:SS') AS jam_selesai,
			COALESCE(k.plat_nomor, '') AS plat_nomor,
			COALESCE(k.jenis_kendaraan, '') AS jenis_kendaraan,
			(SELECT COUNT(*) FROM ritase_stop WHERE id_ritase = r.id_ritase) AS total_stop,
			(SELECT COUNT(*) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Tiba') AS stop_selesai,
			COALESCE((SELECT SUM(jumlah_koli) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Bongkar Muat Barang'), 0) AS total_koli,
			COALESCE((SELECT SUM(jumlah_ecer) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Bongkar Muat Barang'), 0) AS total_ecer,
			COALESCE((SELECT SUM(jumlah_high_value) FROM ritase_event WHERE id_ritase = r.id_ritase AND status = 'Bongkar Muat Barang'), 0) AS total_hv,
			COALESCE((SELECT SUM(durasi_detik) FROM ritase_event WHERE id_ritase = r.id_ritase), 0) AS durasi_detik,
			(SELECT MIN(created_at) FROM ritase_event WHERE id_ritase = r.id_ritase) AS started_at,
			CASE WHEN r.status = 'selesai'
				THEN (SELECT MAX(created_at) FROM ritase_event WHERE id_ritase = r.id_ritase)
				ELSE NULL
			END AS finished_at
		FROM ritase r
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.id_driver = $1
		  AND ((r.tanggal >= $2 AND r.tanggal <= (now() AT TIME ZONE 'Asia/Jakarta')::date)
		       OR (r.tanggal IS NULL AND r.created_at >= $2::timestamp))
		ORDER BY r.tanggal DESC, r.ritase_ke DESC
	`, driverID, since)

	if err != nil {
		log.Printf("[DRIVER HISTORY] query error: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil riwayat")
	}
	defer rows.Close()

	var history []HistoryItem
	for rows.Next() {
		var item HistoryItem
		if err := rows.Scan(
			&item.IDRitase, &item.KodeRitase, &item.Status, &item.RitaseKe,
			&item.JenisRitase, &item.Tanggal,
			&item.JamMulai, &item.JamSelesai,
			&item.PlatNomor, &item.JenisKendaraan,
			&item.TotalStop, &item.StopSelesai,
			&item.TotalKoli, &item.TotalEcer, &item.TotalHV,
			&item.DurasiDetik, &item.StartedAt, &item.FinishedAt,
		); err == nil {
			history = append(history, item)
		}
	}

	if history == nil {
		history = []HistoryItem{}
	}

	// Ambil events untuk setiap ritase (detail timeline)
	for i := range history {
		evRows, err := h.DB.Query(ctx, `
			SELECT
				re.id_event, re.status, re.nama_lokasi, re.durasi_detik,
				COALESCE(re.jumlah_koli, 0), COALESCE(re.jumlah_ecer, 0), COALESCE(re.jumlah_high_value, 0),
				TO_CHAR(re.created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at
			FROM ritase_event re
			WHERE re.id_ritase = $1
			ORDER BY re.created_at ASC, re.id_event ASC
		`, history[i].IDRitase)
		if err != nil {
			continue
		}
		for evRows.Next() {
			var ev EventItem
			if err := evRows.Scan(
				&ev.IDEvent, &ev.Status, &ev.NamaLokasi, &ev.DurasiDetik,
				&ev.Koli, &ev.Ecer, &ev.HV, &ev.CreatedAt,
			); err == nil {
				history[i].Events = append(history[i].Events, ev)
			}
		}
		evRows.Close()
	}

	return response.OK(c, map[string]interface{}{
		"driver":  driverID,
		"days":    days,
		"total":   len(history),
		"history": history,
	})
}
