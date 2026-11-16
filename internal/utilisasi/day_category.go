package utilisasi

import (
	"time"

	"backend/internal/pkg/timeutil"
)

const dateLayout = "2006-01-02"

func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, s, timeutil.WIB())
}

func formatDate(t time.Time) string {
	return t.Format(dateLayout)
}

// defaultCategory aturan default: Senin Peak, Selasa-Sabtu Normal,
// Minggu Low, dan tanggal kembar (dd == mm) Peak.
func defaultCategory(t time.Time) Category {
	if t.Day() == int(t.Month()) {
		return CategoryPeak
	}
	switch t.Weekday() {
	case time.Monday:
		return CategoryPeak
	case time.Sunday:
		return CategoryLow
	default:
		return CategoryNormal
	}
}

// effectiveCategory: override manual menang atas tanggal kembar & default.
func effectiveCategory(t time.Time, override *Override) (Category, string) {
	if override != nil {
		return Category(override.Kategori), "override"
	}
	return defaultCategory(t), "default"
}

// weekStart mengembalikan Senin pada minggu yang sama (WIB).
func weekStart(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7
	y, m, d := t.Date()
	return time.Date(y, m, d-offset, 0, 0, 0, 0, t.Location())
}

func monthStart(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func monthEnd(t time.Time) time.Time {
	return monthStart(t).AddDate(0, 1, -1)
}

func dateRange(start, end time.Time) []time.Time {
	var out []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}
