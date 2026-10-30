package kapten

import (
	"testing"
	"time"
)

func TestRitDeadline(t *testing.T) {
	loc := time.FixedZone("WIB", 7*60*60)
	endOfDay := func(day, hour, minute int) time.Time {
		return time.Date(2026, 10, day, hour, minute, 59, int(999*time.Millisecond), loc)
	}
	tests := []struct {
		name    string
		created time.Time
		window  ritWindow
		want    time.Time
	}{
		{
			name:    "outgoing rit 1 same day",
			created: time.Date(2026, 10, 22, 18, 0, 0, 0, loc),
			window:  defaultRitWindows["outgoing|1"],
			want:    endOfDay(22, 19, 59),
		},
		{
			name:    "outgoing rit 2 same day",
			created: time.Date(2026, 10, 22, 17, 0, 0, 0, loc),
			window:  defaultRitWindows["outgoing|2"],
			want:    endOfDay(22, 23, 59),
		},
		{
			name:    "outgoing rit 3 before midnight",
			created: time.Date(2026, 10, 22, 23, 0, 0, 0, loc),
			window:  defaultRitWindows["outgoing|3"],
			want:    endOfDay(23, 3, 0),
		},
		{
			name:    "outgoing rit 3 after midnight",
			created: time.Date(2026, 10, 23, 1, 0, 0, 0, loc),
			window:  defaultRitWindows["outgoing|3"],
			want:    endOfDay(23, 3, 0),
		},
		{
			name:    "incoming rit 4 same day",
			created: time.Date(2026, 10, 22, 20, 0, 0, 0, loc),
			window:  defaultRitWindows["incoming|4"],
			want:    endOfDay(22, 22, 30),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ritDeadline(tt.created, tt.window, loc); !got.Equal(tt.want) {
				t.Fatalf("ritDeadline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRitOverdue(t *testing.T) {
	loc := time.FixedZone("WIB", 7*60*60)
	created := time.Date(2026, 10, 22, 18, 0, 0, 0, loc)
	window := defaultRitWindows["outgoing|1"]

	if ritOverdue(created, window, time.Date(2026, 10, 22, 19, 59, 59, 999*int(time.Millisecond), loc)) {
		t.Fatal("outgoing rit 1 should not be overdue before the deadline")
	}
	if !ritOverdue(created, window, time.Date(2026, 10, 22, 20, 0, 0, 0, loc)) {
		t.Fatal("outgoing rit 1 should be overdue at 20:00 WIB")
	}
}

func TestPendingOverdue(t *testing.T) {
	loc := time.FixedZone("WIB", 7*60*60)
	windows := map[string]ritWindow{
		"outgoing|1": defaultRitWindows["outgoing|1"],
		"outgoing|3": defaultRitWindows["outgoing|3"],
	}
	tests := []struct {
		name string
		p    map[string]interface{}
		now  time.Time
		want bool
	}{
		{
			name: "expired rit 1",
			p: map[string]interface{}{
				"jenis_ritase": "outgoing", "ritase_ke": 1,
				"created_at": "2026-10-22T18:00:00+07:00",
			},
			now:  time.Date(2026, 10, 22, 22, 19, 0, 0, loc),
			want: true,
		},
		{
			name: "active rit 1 before deadline",
			p: map[string]interface{}{
				"jenis_ritase": "outgoing", "ritase_ke": 1,
				"created_at": "2026-10-22T18:00:00+07:00",
			},
			now:  time.Date(2026, 10, 22, 18, 30, 0, 0, loc),
			want: false,
		},
		{
			name: "rit 3 still within cross-midnight window",
			p: map[string]interface{}{
				"jenis_ritase": "outgoing", "ritase_ke": 3,
				"created_at": "2026-10-22T23:00:00+07:00",
			},
			now:  time.Date(2026, 10, 23, 2, 59, 0, 0, loc),
			want: false,
		},
		{
			name: "rit 3 past cross-midnight deadline",
			p: map[string]interface{}{
				"jenis_ritase": "outgoing", "ritase_ke": 3,
				"created_at": "2026-10-22T23:00:00+07:00",
			},
			now:  time.Date(2026, 10, 23, 3, 1, 0, 0, loc),
			want: true,
		},
		{
			name: "invalid created_at stays normal",
			p: map[string]interface{}{
				"jenis_ritase": "outgoing", "ritase_ke": 1,
				"created_at": "invalid",
			},
			now:  time.Date(2026, 10, 22, 22, 19, 0, 0, loc),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pendingOverdue(tt.p, windows, tt.now); got != tt.want {
				t.Fatalf("pendingOverdue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRitDateOverdue(t *testing.T) {
	loc := time.FixedZone("WIB", 7*60*60)
	windows := defaultRitWindows
	tests := []struct {
		name    string
		tanggal string
		jenis   string
		ritKe   int
		now     time.Time
		want    bool
	}{
		{
			name:    "rit 1 aktif sebelum 19:59",
			tanggal: "2026-10-22", jenis: "outgoing", ritKe: 1,
			now:  time.Date(2026, 10, 22, 19, 58, 0, 0, loc),
			want: false,
		},
		{
			name:    "rit 1 terlewat setelah 19:59",
			tanggal: "2026-10-22", jenis: "outgoing", ritKe: 1,
			now:  time.Date(2026, 10, 22, 20, 1, 0, 0, loc),
			want: true,
		},
		{
			name:    "rit 3 lintas tengah malam masih aktif",
			tanggal: "2026-10-22", jenis: "outgoing", ritKe: 3,
			now:  time.Date(2026, 10, 23, 2, 59, 0, 0, loc),
			want: false,
		},
		{
			name:    "rit 3 terlewat setelah 03:00",
			tanggal: "2026-10-22", jenis: "outgoing", ritKe: 3,
			now:  time.Date(2026, 10, 23, 3, 1, 0, 0, loc),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ritDateOverdue(tt.tanggal, tt.jenis, tt.ritKe, windows, tt.now); got != tt.want {
				t.Fatalf("ritDateOverdue() = %v, want %v", got, tt.want)
			}
		})
	}
}
