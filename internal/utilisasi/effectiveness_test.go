package utilisasi

import (
	"math"
	"testing"
)

func TestBuildEffectivenessMixedMetrics(t *testing.T) {
	trips := []effectivenessTrip{
		{
			Meta:    vehicleMeta{ID: 1, Plat: "B 1 CDDL", Jenis: "CDDL", Metric: MetricKoli, Min: 80, Max: 100},
			Load:    39,
			HasData: true,
		},
		{
			Meta:    vehicleMeta{ID: 2, Plat: "B 2 TRAGA", Jenis: "TRAGA", Metric: MetricAWB, Min: 3000, Max: 3000},
			Load:    1500,
			HasData: true,
		},
		{
			Meta:    vehicleMeta{ID: 3, Plat: "B 3 BV", Jenis: "BV", Metric: MetricAWB, Min: 1200, Max: 1500},
			Load:    1800,
			HasData: true,
		},
		{
			Meta: vehicleMeta{ID: 4, Plat: "B 4 CDD", Jenis: "CDD", Metric: MetricKoli, Min: 69, Max: 86},
		},
	}

	got := buildEffectiveness("2026-01-01", "2026-01-01", len(trips), len(trips), trips)

	if got.OverallPct == nil {
		t.Fatal("overall pct nil")
	}
	wantOverall := (39.0 + 50.0 + 120.0) / 3.0
	if math.Abs(*got.OverallPct-wantOverall) > 0.001 {
		t.Fatalf("overall pct = %.4f, want %.4f", *got.OverallPct, wantOverall)
	}
	if got.TotalTrips != 4 || got.TripsWithData != 3 {
		t.Fatalf("trips = %d/%d, want 4/3", got.TotalTrips, got.TripsWithData)
	}
	if got.TotalVehicles != 4 || got.VehiclesWithData != 3 {
		t.Fatalf("vehicles = %d/%d, want 4/3", got.TotalVehicles, got.VehiclesWithData)
	}
	if got.StatusCount[StatusUnder] != 1 ||
		got.StatusCount[StatusOptimal] != 1 ||
		got.StatusCount[StatusOver] != 1 ||
		got.StatusCount[StatusNoData] != 0 {
		t.Fatalf("status count = %+v", got.StatusCount)
	}
	if len(got.Vehicles) != 3 {
		t.Fatalf("vehicles len = %d, want 3", len(got.Vehicles))
	}
	if got.Vehicles[0].PlatNomor != "B 1 CDDL" ||
		got.Vehicles[1].PlatNomor != "B 2 TRAGA" ||
		got.Vehicles[2].PlatNomor != "B 3 BV" {
		t.Fatalf("vehicle order = %+v", got.Vehicles)
	}
}

func TestBuildEffectivenessAggregatesVehicleRits(t *testing.T) {
	meta := vehicleMeta{ID: 1, Plat: "B 1 CDDL", Jenis: "CDDL", Metric: MetricKoli, Min: 80, Max: 100}
	trips := []effectivenessTrip{
		{Meta: meta, Load: 40, HasData: true},
		{Meta: meta, Load: 80, HasData: true},
		{
			Meta:    vehicleMeta{ID: 2, Plat: "B 2 CDDL", Jenis: "CDDL", Metric: MetricKoli, Min: 80, Max: 100},
			Load:    30,
			HasData: true,
		},
	}

	got := buildEffectiveness("2026-01-01", "2026-01-01", len(trips), 2, trips)

	if got.VehiclesWithData != 2 || len(got.Vehicles) != 2 {
		t.Fatalf("vehicles = %d/%d, want 2/2", got.VehiclesWithData, len(got.Vehicles))
	}
	if got.Vehicles[0].IDKendaraan != 2 || got.Vehicles[1].IDKendaraan != 1 {
		t.Fatalf("vehicle order = %+v", got.Vehicles)
	}
	aggregated := got.Vehicles[1]
	if aggregated.Trips != 2 || aggregated.TripsWithData != 2 || aggregated.Load != 120 {
		t.Fatalf("aggregated vehicle = %+v", aggregated)
	}
	if aggregated.Pct == nil || math.Abs(*aggregated.Pct-60) > 0.001 {
		t.Fatalf("aggregated pct = %v, want 60", aggregated.Pct)
	}
	if aggregated.Status != StatusOptimal {
		t.Fatalf("aggregated status = %s, want optimal", aggregated.Status)
	}
	if got.StatusCount[StatusUnder] != 1 || got.StatusCount[StatusOptimal] != 1 {
		t.Fatalf("status count = %+v", got.StatusCount)
	}
	if got.OverallPct == nil || math.Abs(*got.OverallPct-45) > 0.001 {
		t.Fatalf("overall pct = %v, want 45", got.OverallPct)
	}
}

func TestBuildEffectivenessWithoutData(t *testing.T) {
	trips := []effectivenessTrip{
		{
			Meta: vehicleMeta{ID: 1, Plat: "B 1 CDDL", Jenis: "CDDL", Metric: MetricKoli, Min: 80, Max: 100},
		},
		{
			Meta: vehicleMeta{ID: 2, Plat: "B 2 CDD", Jenis: "CDD", Metric: MetricKoli, Min: 69, Max: 86},
		},
	}

	got := buildEffectiveness("2026-01-01", "2026-01-01", len(trips), len(trips), trips)

	if got.OverallPct != nil {
		t.Fatalf("overall pct = %v, want nil", *got.OverallPct)
	}
	if got.TripsWithData != 0 || got.VehiclesWithData != 0 {
		t.Fatalf("data counts = %d/%d, want 0/0", got.TripsWithData, got.VehiclesWithData)
	}
	if got.StatusCount[StatusNoData] != 0 {
		t.Fatalf("no_data count = %d, want 0", got.StatusCount[StatusNoData])
	}
	if len(got.Vehicles) != 0 {
		t.Fatalf("vehicles len = %d, want 0", len(got.Vehicles))
	}
}

func TestBuildEffectivenessReturnsAllDataVehiclesSorted(t *testing.T) {
	trips := make([]effectivenessTrip, 0, 7)
	for i := 1; i <= 7; i++ {
		trips = append(trips, effectivenessTrip{
			Meta:    vehicleMeta{ID: int64(i), Plat: string(rune('A' + i - 1)), Jenis: "CDDL", Metric: MetricKoli, Min: 80, Max: 100},
			Load:    i * 10,
			HasData: true,
		})
	}

	got := buildEffectiveness("2026-01-01", "2026-01-01", len(trips), len(trips), trips)

	if len(got.Vehicles) != len(trips) {
		t.Fatalf("vehicles len = %d, want %d", len(got.Vehicles), len(trips))
	}
	for i := 1; i < len(got.Vehicles); i++ {
		prev := got.Vehicles[i-1].Pct
		curr := got.Vehicles[i].Pct
		if prev == nil || curr == nil || *prev > *curr {
			t.Fatalf("vehicles not sorted at %d: %+v", i, got.Vehicles)
		}
	}
}
