package utilisasi

import "sort"

type EffectivenessVehicle struct {
	IDKendaraan    int64    `json:"id_kendaraan"`
	PlatNomor      string   `json:"plat_nomor"`
	JenisKendaraan string   `json:"jenis_kendaraan"`
	Metric         Metric   `json:"metric"`
	Trips          int      `json:"trips"`
	TripsWithData  int      `json:"trips_with_data"`
	Load           int      `json:"load"`
	CapacityMax    int      `json:"capacity_max"`
	Pct            *float64 `json:"pct"`
	Status         Status   `json:"status"`
}

type EffectivenessSummary struct {
	StartDate        string                 `json:"start_date"`
	EndDate          string                 `json:"end_date"`
	OverallPct       *float64               `json:"overall_pct"`
	TotalTrips       int                    `json:"total_trips"`
	TripsWithData    int                    `json:"trips_with_data"`
	TotalVehicles    int                    `json:"total_vehicles"`
	VehiclesWithData int                    `json:"vehicles_with_data"`
	StatusCount      map[Status]int         `json:"status_count"`
	Vehicles         []EffectivenessVehicle `json:"vehicles"`
}

type effectivenessTrip struct {
	Meta    vehicleMeta
	Load    int
	HasData bool
}

type effectivenessVehicleAgg struct {
	Vehicle EffectivenessVehicle
	Min     int
}

// buildEffectiveness menghitung ringkasan per kendaraan. Kendaraan tanpa
// muatan berdata tidak dimasukkan ke status maupun daftar kendaraan.
func buildEffectiveness(
	startDate, endDate string,
	totalTrips int,
	totalVehicles int,
	trips []effectivenessTrip,
) EffectivenessSummary {
	statusCount := map[Status]int{
		StatusUnder:   0,
		StatusOptimal: 0,
		StatusOver:    0,
		StatusNoData:  0,
	}
	vehicles := make(map[int64]*effectivenessVehicleAgg, len(trips))
	tripsWithData := 0

	for _, trip := range trips {
		vehicle := vehicles[trip.Meta.ID]
		if vehicle == nil {
			vehicle = &effectivenessVehicleAgg{
				Vehicle: EffectivenessVehicle{
					IDKendaraan:    trip.Meta.ID,
					PlatNomor:      trip.Meta.Plat,
					JenisKendaraan: trip.Meta.Jenis,
					Metric:         trip.Meta.Metric,
					CapacityMax:    trip.Meta.Max,
					Status:         StatusNoData,
				},
				Min: trip.Meta.Min,
			}
			vehicles[trip.Meta.ID] = vehicle
		}
		vehicle.Vehicle.Trips++

		if !trip.HasData || trip.Meta.Max <= 0 {
			continue
		}

		tripsWithData++
		vehicle.Vehicle.TripsWithData++
		vehicle.Vehicle.Load += trip.Load
	}

	items := make([]EffectivenessVehicle, 0, len(vehicles))
	var pctSum float64
	for _, vehicle := range vehicles {
		if vehicle.Vehicle.TripsWithData <= 0 || vehicle.Vehicle.CapacityMax <= 0 {
			continue
		}
		pct := float64(vehicle.Vehicle.Load) /
			float64(vehicle.Vehicle.CapacityMax*vehicle.Vehicle.TripsWithData) * 100
		status := statusFor(pct, vehicle.Min, vehicle.Vehicle.CapacityMax)
		vehicle.Vehicle.Pct = &pct
		vehicle.Vehicle.Status = status
		statusCount[status]++
		pctSum += pct
		items = append(items, vehicle.Vehicle)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Pct == nil || items[j].Pct == nil {
			return items[i].Pct != nil
		}
		if *items[i].Pct != *items[j].Pct {
			return *items[i].Pct < *items[j].Pct
		}
		return items[i].PlatNomor < items[j].PlatNomor
	})

	var overallPct *float64
	if len(items) > 0 {
		avg := pctSum / float64(len(items))
		overallPct = &avg
	}
	if totalVehicles < len(vehicles) {
		totalVehicles = len(vehicles)
	}

	return EffectivenessSummary{
		StartDate:        startDate,
		EndDate:          endDate,
		OverallPct:       overallPct,
		TotalTrips:       totalTrips,
		TripsWithData:    tripsWithData,
		TotalVehicles:    totalVehicles,
		VehiclesWithData: len(items),
		StatusCount:      statusCount,
		Vehicles:         items,
	}
}
