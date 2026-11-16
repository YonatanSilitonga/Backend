package utilisasi

import (
	"testing"
	"time"

	"backend/internal/pkg/timeutil"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := parseDate(s)
	if err != nil {
		t.Fatalf("parseDate(%q): %v", s, err)
	}
	return d
}

func TestDefaultCategory(t *testing.T) {
	cases := []struct {
		tanggal string
		want    Category
	}{
		{"2026-01-05", CategoryPeak},   // Senin
		{"2026-01-06", CategoryNormal}, // Selasa
		{"2026-01-10", CategoryNormal}, // Sabtu
		{"2026-01-11", CategoryLow},    // Minggu
		{"2026-03-03", CategoryPeak},   // tanggal kembar (dd == mm)
		{"2026-02-04", CategoryNormal}, // Rabu, bukan kembar
	}
	for _, tc := range cases {
		if got := defaultCategory(mustDate(t, tc.tanggal)); got != tc.want {
			t.Errorf("defaultCategory(%s) = %s, want %s", tc.tanggal, got, tc.want)
		}
	}
}

func TestEffectiveCategoryOverrideWins(t *testing.T) {
	d := mustDate(t, "2026-01-05") // default Peak
	ov := &Override{Tanggal: "2026-01-05", Kategori: "low"}

	got, source := effectiveCategory(d, ov)
	if got != CategoryLow {
		t.Fatalf("override category = %s, want low", got)
	}
	if source != "override" {
		t.Fatalf("source = %s, want override", source)
	}

	got, source = effectiveCategory(d, nil)
	if got != CategoryPeak || source != "default" {
		t.Fatalf("default = (%s,%s), want (peak,default)", got, source)
	}
}

func TestWeekStart(t *testing.T) {
	cases := []struct {
		tanggal string
		want    string
	}{
		{"2026-01-05", "2026-01-05"}, // Senin
		{"2026-01-07", "2026-01-05"}, // Rabu
		{"2026-01-11", "2026-01-05"}, // Minggu
		{"2026-02-01", "2026-01-26"}, // Minggu lintas bulan
	}
	for _, tc := range cases {
		got := formatDate(weekStart(mustDate(t, tc.tanggal)))
		if got != tc.want {
			t.Errorf("weekStart(%s) = %s, want %s", tc.tanggal, got, tc.want)
		}
	}
}

func TestMonthBounds(t *testing.T) {
	d := mustDate(t, "2026-02-15")
	if got := formatDate(monthStart(d)); got != "2026-02-01" {
		t.Errorf("monthStart = %s, want 2026-02-01", got)
	}
	if got := formatDate(monthEnd(d)); got != "2026-02-28" {
		t.Errorf("monthEnd = %s, want 2026-02-28", got)
	}
}

func TestDateRange(t *testing.T) {
	got := dateRange(mustDate(t, "2026-01-01"), mustDate(t, "2026-01-03"))
	if len(got) != 3 {
		t.Fatalf("dateRange len = %d, want 3", len(got))
	}
	if formatDate(got[0]) != "2026-01-01" || formatDate(got[2]) != "2026-01-03" {
		t.Fatalf("dateRange bounds = %s..%s", formatDate(got[0]), formatDate(got[2]))
	}
}

func TestStatusFor(t *testing.T) {
	cases := []struct {
		pct  float64
		min  int
		max  int
		want Status
	}{
		{39, 80, 100, StatusUnder},
		{40, 80, 100, StatusOptimal},
		{100, 80, 100, StatusOptimal},
		{101, 80, 100, StatusOver},
		{39, 1200, 1500, StatusUnder},
		{40, 1200, 1500, StatusOptimal},
		{101, 1200, 1500, StatusOver},
	}
	for _, tc := range cases {
		if got := statusFor(tc.pct, tc.min, tc.max); got != tc.want {
			t.Errorf("statusFor(%.0f,%d,%d) = %s, want %s", tc.pct, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestRuleFor(t *testing.T) {
	cases := []struct {
		jenis   string
		koli    int
		kg      int
		wantMet Metric
		wantMin int
		wantMax int
	}{
		{"CDDL", 0, 0, MetricKoli, 80, 100},
		{"cdd", 0, 0, MetricKoli, 69, 86},
		{"TRAGA", 0, 0, MetricAWB, 3000, 3000},
		{"BV", 0, 0, MetricAWB, 1200, 1500},
		{"CDDL", 90, 0, MetricKoli, 80, 90},
		{"TRAGA", 0, 3200, MetricAWB, 3000, 3200},
		{"UNKNOWN", 50, 0, MetricKoli, 0, 50},
		{"UNKNOWN", 0, 4000, MetricAWB, 0, 4000},
	}
	for _, tc := range cases {
		_, metric, min, max := ruleFor(tc.jenis, tc.koli, tc.kg)
		if metric != tc.wantMet || min != tc.wantMin || max != tc.wantMax {
			t.Errorf("ruleFor(%s,%d,%d) = (%s,%d,%d), want (%s,%d,%d)",
				tc.jenis, tc.koli, tc.kg, metric, min, max, tc.wantMet, tc.wantMin, tc.wantMax)
		}
	}
}

func TestCellFor(t *testing.T) {
	noData := cellFor(0, 1, 0, 80, 100)
	if noData.Status != StatusNoData || noData.Pct != nil {
		t.Fatalf("cell tanpa data = %+v", noData)
	}

	optimal := cellFor(90, 1, 1, 80, 100)
	if optimal.Pct == nil || *optimal.Pct != 90 || optimal.Status != StatusOptimal {
		t.Fatalf("cell optimal = %+v", optimal)
	}

	aggregate := cellFor(180, 2, 2, 80, 100)
	if aggregate.Pct == nil || *aggregate.Pct != 90 {
		t.Fatalf("cell agregat pct = %v", aggregate.Pct)
	}

	over := cellFor(210, 2, 2, 80, 100)
	if over.Status != StatusOver {
		t.Fatalf("cell over = %+v", over)
	}
}

func TestP95(t *testing.T) {
	if got := p95(nil); got != 0 {
		t.Errorf("p95(nil) = %d, want 0", got)
	}
	values := []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if got := p95(values); got != 100 {
		t.Errorf("p95 = %d, want 100", got)
	}
	if got := p95([]int{5}); got != 5 {
		t.Errorf("p95 single = %d, want 5", got)
	}
}

func TestParseDateUsesWIB(t *testing.T) {
	d := mustDate(t, "2026-01-01")
	if _, offset := d.Zone(); offset != 7*3600 {
		t.Fatalf("offset = %d, want WIB (+7h)", offset)
	}
	if d.Location().String() != timeutil.WIB().String() {
		t.Fatalf("location = %v, want WIB", d.Location())
	}
}
