package timeutil

import (
	"testing"
	"time"
)

func TestWithinMinutes(t *testing.T) {
	loc := WIB()
	tests := []struct {
		name     string
		now      time.Time
		startMin int
		endMin   int
		want     bool
	}{
		{
			name:     "20:44 WIB di dalam 16:00-23:59",
			now:      time.Date(2026, 11, 6, 20, 44, 0, 0, loc),
			startMin: 16 * 60,
			endMin:   23*60 + 59,
			want:     true,
		},
		{
			name:     "13:44 UTC dikonversi ke WIB di dalam 16:00-23:59",
			now:      time.Date(2026, 11, 6, 13, 44, 0, 0, time.UTC).In(loc),
			startMin: 16 * 60,
			endMin:   23*60 + 59,
			want:     true,
		},
		{
			name:     "15:59 WIB sebelum window",
			now:      time.Date(2026, 11, 6, 15, 59, 0, 0, loc),
			startMin: 16 * 60,
			endMin:   23*60 + 59,
			want:     false,
		},
		{
			name:     "16:00 WIB batas awal window",
			now:      time.Date(2026, 11, 6, 16, 0, 0, 0, loc),
			startMin: 16 * 60,
			endMin:   23*60 + 59,
			want:     true,
		},
		{
			name:     "23:59 WIB batas akhir window",
			now:      time.Date(2026, 11, 6, 23, 59, 0, 0, loc),
			startMin: 16 * 60,
			endMin:   23*60 + 59,
			want:     true,
		},
		{
			name:     "00:00 WIB setelah window",
			now:      time.Date(2026, 11, 7, 0, 0, 0, 0, loc),
			startMin: 16 * 60,
			endMin:   23*60 + 59,
			want:     false,
		},
		{
			name:     "01:00 WIB di dalam window lintas tengah malam",
			now:      time.Date(2026, 11, 7, 1, 0, 0, 0, loc),
			startMin: 20 * 60,
			endMin:   3 * 60,
			want:     true,
		},
		{
			name:     "03:00 WIB batas akhir window lintas tengah malam",
			now:      time.Date(2026, 11, 7, 3, 0, 0, 0, loc),
			startMin: 20 * 60,
			endMin:   3 * 60,
			want:     true,
		},
		{
			name:     "03:01 WIB di luar window lintas tengah malam",
			now:      time.Date(2026, 11, 7, 3, 1, 0, 0, loc),
			startMin: 20 * 60,
			endMin:   3 * 60,
			want:     false,
		},
		{
			name:     "19:59 WIB sebelum window lintas tengah malam",
			now:      time.Date(2026, 11, 6, 19, 59, 0, 0, loc),
			startMin: 20 * 60,
			endMin:   3 * 60,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithinMinutes(tt.now, tt.startMin, tt.endMin); got != tt.want {
				t.Fatalf("WithinMinutes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWIBOffset(t *testing.T) {
	_, offset := time.Date(2026, 11, 6, 12, 0, 0, 0, WIB()).Zone()
	if offset != 7*60*60 {
		t.Fatalf("offset WIB = %d, want %d", offset, 7*60*60)
	}
}
