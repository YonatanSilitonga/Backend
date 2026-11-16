package utilisasi

import (
	"math"
	"sort"
	"strings"
)

// capacityRule acuan kapasitas per tipe kendaraan.
// Max adalah batas atas; Min dipakai untuk ambang Under.
type capacityRule struct {
	Metric  Metric
	MinKoli int
	MaxKoli int
	MinAWB  int
	MaxAWB  int
}

var capacityRules = map[string]capacityRule{
	"CDDL":  {Metric: MetricKoli, MinKoli: 80, MaxKoli: 100, MaxAWB: 7000},
	"CDD":   {Metric: MetricKoli, MinKoli: 69, MaxKoli: 86, MaxAWB: 6000},
	"CDEL":  {Metric: MetricKoli, MinKoli: 57, MaxKoli: 71, MaxAWB: 5000},
	"CDE":   {Metric: MetricKoli, MinKoli: 51, MaxKoli: 64, MaxAWB: 4500},
	"TRAGA": {Metric: MetricAWB, MinAWB: 3000, MaxAWB: 3000},
	"BV":    {Metric: MetricAWB, MinAWB: 1200, MaxAWB: 1500},
}

func normalizeJenis(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// ruleFor mengembalikan aturan kapasitas + metrik + min/max efektif.
// Nilai kapasitas dari DB (jika > 0) menjadi batas atas; jika kosong,
// fallback ke tabel tipe.
func ruleFor(jenis string, kapasitasKoli, kapasitasKg int) (capacityRule, Metric, int, int) {
	rule, ok := capacityRules[normalizeJenis(jenis)]
	if !ok {
		switch {
		case kapasitasKoli > 0:
			rule = capacityRule{Metric: MetricKoli, MaxKoli: kapasitasKoli, MaxAWB: kapasitasKg}
		case kapasitasKg > 0:
			rule = capacityRule{Metric: MetricAWB, MaxAWB: kapasitasKg}
		default:
			rule = capacityRule{Metric: MetricKoli}
		}
	}

	metric := rule.Metric
	if kapasitasKoli > 0 {
		metric = MetricKoli
	}

	maxKoli := rule.MaxKoli
	if kapasitasKoli > 0 {
		maxKoli = kapasitasKoli
	}
	maxAWB := rule.MaxAWB
	if kapasitasKg > 0 {
		maxAWB = kapasitasKg
	}

	if metric == MetricAWB {
		if maxAWB <= 0 {
			return rule, MetricAWB, 0, 0
		}
		min := rule.MinAWB
		if min > maxAWB {
			min = maxAWB
		}
		return rule, MetricAWB, min, maxAWB
	}

	if maxKoli <= 0 {
		return rule, MetricKoli, 0, 0
	}
	min := rule.MinKoli
	if min > maxKoli {
		min = maxKoli
	}
	return rule, MetricKoli, min, maxKoli
}

// statusFor menentukan status dari persentase terhadap batas atas.
// Under jika < 50% dari rasio min/max, Over jika > 100%.
func statusFor(pct float64, min, max int) Status {
	if max <= 0 {
		return StatusNoData
	}
	under := 50.0 * float64(min) / float64(max)
	switch {
	case pct < under:
		return StatusUnder
	case pct > 100:
		return StatusOver
	default:
		return StatusOptimal
	}
}

func cellFor(load, trips, tripsWithData, capacityMin, capacityMax int) Cell {
	c := Cell{Trips: trips, TripsWithData: tripsWithData, Load: load}
	if tripsWithData <= 0 || capacityMax <= 0 {
		c.Status = StatusNoData
		return c
	}
	pct := float64(load) / float64(capacityMax*tripsWithData) * 100
	c.Pct = &pct
	c.Status = statusFor(pct, capacityMin, capacityMax)
	return c
}

func pctFor(load, tripsWithData, capacityMax int) *float64 {
	if tripsWithData <= 0 || capacityMax <= 0 {
		return nil
	}
	v := float64(load) / float64(capacityMax*tripsWithData) * 100
	return &v
}

func p95(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sort.Ints(values)
	idx := int(math.Ceil(0.95*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}
