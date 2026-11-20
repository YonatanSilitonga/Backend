package utilisasi

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type Override struct {
	Tanggal    string
	Kategori   string
	Keterangan string
}

type vehicleRow struct {
	IDKendaraan    int64
	PlatNomor      string
	JenisKendaraan string
	KapasitasKoli  int
	KapasitasKg    int
	Status         string
}

type tripRow struct {
	IDRitase          int64
	Tanggal           string
	RitaseKe          int
	IDKendaraan       int64
	PlatNomor         string
	JenisKendaraan    string
	NamaDriver        string
	KapasitasKoli     int
	KapasitasKg       int
	EventKoli         int
	EventKoliJKT      int
	EventKoliSEG      int
	KonfirmasiKoli    int
	KonfirmasiKoliJKT int
	KonfirmasiKoliSEG int
	KonfirmasiAWB     int
	RitaseKoli        int
	RitaseAWB         int
	GatewayAkhir      string
}

type agg struct {
	Trips         int
	TripsWithData int
	Load          int
	Koli          int
	AWB           int
}

func (a *agg) add(hasData bool, load, koli, awb int) {
	a.Trips++
	if !hasData {
		return
	}
	a.TripsWithData++
	a.Load += load
	a.Koli += koli
	a.AWB += awb
}

type vehicleMeta struct {
	ID     int64
	Plat   string
	Jenis  string
	Metric Metric
	Min    int
	Max    int
	AWBMax int
	Active bool
}

type resolvedTrip struct {
	Detail    TripDetail
	HasData   bool
	Load      int
	Koli      int
	AWB       int
	Date      time.Time
	WeekKey   string
	DayCat    Category
	VehicleID int64
	RitaseKe  int
}

type dailyAgg struct {
	Rits  [4]agg
	Total agg
}

type weeklyAgg struct {
	Categories map[Category]*agg
	Total      agg
}

func (r *Repository) ListVehicles(ctx context.Context) ([]vehicleRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, COALESCE(jenis_kendaraan,''),
		       COALESCE(kapasitas_koli,0), COALESCE(kapasitas_kg,0),
		       COALESCE(status_kendaraan,'aktif')
		FROM kendaraan
		ORDER BY plat_nomor`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []vehicleRow
	for rows.Next() {
		var v vehicleRow
		if err := rows.Scan(&v.IDKendaraan, &v.PlatNomor, &v.JenisKendaraan,
			&v.KapasitasKoli, &v.KapasitasKg, &v.Status); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *Repository) ListTrips(ctx context.Context, start, end time.Time) ([]tripRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id_ritase, r.tanggal::text, COALESCE(r.ritase_ke,1),
		       r.id_kendaraan, COALESCE(k.plat_nomor,''), COALESCE(k.jenis_kendaraan,''),
		       COALESCE(d.nama_driver,''),
		       COALESCE(k.kapasitas_koli,0), COALESCE(k.kapasitas_kg,0),
		       COALESCE(ev.koli,0), COALESCE(ev.koli_jkt,0), COALESCE(ev.koli_seg,0) + COALESCE(ev.koli_btn,0),
		       COALESCE(kp.koli,0), COALESCE(kp.koli_jkt,0), COALESCE(kp.koli_seg,0) + COALESCE(kp.koli_btn,0),
		       COALESCE(kp.awb,0),
		       COALESCE(r.total_koli,0), COALESCE(r.total_awb,0),
		       COALESCE(gl.gateway, '')
		FROM ritase r
		JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN (
			SELECT id_ritase, SUM(jumlah_koli) AS koli,
			       SUM(COALESCE(koli_jkt,0)) AS koli_jkt,
			       SUM(COALESCE(koli_seg,0)) AS koli_seg,
			       SUM(COALESCE(koli_btn,0)) AS koli_btn
			FROM ritase_event
			WHERE status IN ('Bongkar Muat Barang', 'Muat Barang')
			GROUP BY id_ritase
		) ev ON ev.id_ritase = r.id_ritase
		LEFT JOIN (
			SELECT id_ritase, SUM(jumlah_awb) AS awb,
			       SUM(COALESCE(koli_jkt,0) + COALESCE(koli_seg,0) + COALESCE(koli_btn,0)) AS koli,
			       SUM(COALESCE(koli_jkt,0)) AS koli_jkt,
			       SUM(COALESCE(koli_seg,0)) AS koli_seg,
			       SUM(COALESCE(koli_btn,0)) AS koli_btn
			FROM konfirmasi_penjemputan
			GROUP BY id_ritase
		) kp ON kp.id_ritase = r.id_ritase
		LEFT JOIN LATERAL (
			SELECT COALESCE(
				r.id_drop_point,
				(SELECT rs.id_drop_point
				 FROM ritase_stop rs
				 WHERE rs.id_ritase = r.id_ritase AND rs.id_drop_point IS NOT NULL
				 ORDER BY rs.urutan DESC
				 LIMIT 1)
			) AS id_drop_point
		) gwp ON true
		LEFT JOIN drop_point dp ON dp.id_drop_point = gwp.id_drop_point
		LEFT JOIN LATERAL (
			SELECT CASE
				WHEN POSITION('jkt' IN LOWER(COALESCE(dp.kode_dp, ''))) > 0
				  OR POSITION('jkt' IN LOWER(COALESCE(dp.nama_drop_point, ''))) > 0 THEN 'JKT'
				WHEN POSITION('seg' IN LOWER(COALESCE(dp.kode_dp, ''))) > 0
				  OR POSITION('seg' IN LOWER(COALESCE(dp.nama_drop_point, ''))) > 0
				  OR POSITION('btn' IN LOWER(COALESCE(dp.kode_dp, ''))) > 0
				  OR POSITION('btn' IN LOWER(COALESCE(dp.nama_drop_point, ''))) > 0 THEN 'SEG'
				ELSE ''
			END AS gateway
		) gl ON true
		WHERE r.tanggal BETWEEN $1::date AND $2::date
		  AND COALESCE(r.jenis_ritase,'outgoing') = 'outgoing'
		  AND COALESCE(r.status,'') NOT IN ('batal','cancelled','dibatalkan')
		ORDER BY r.tanggal, COALESCE(r.ritase_ke,1), k.plat_nomor, r.id_ritase`,
		formatDate(start), formatDate(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []tripRow
	for rows.Next() {
		var t tripRow
		if err := rows.Scan(&t.IDRitase, &t.Tanggal, &t.RitaseKe,
			&t.IDKendaraan, &t.PlatNomor, &t.JenisKendaraan,
			&t.NamaDriver, &t.KapasitasKoli, &t.KapasitasKg,
			&t.EventKoli, &t.EventKoliJKT, &t.EventKoliSEG,
			&t.KonfirmasiKoli, &t.KonfirmasiKoliJKT, &t.KonfirmasiKoliSEG, &t.KonfirmasiAWB,
			&t.RitaseKoli, &t.RitaseAWB, &t.GatewayAkhir); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *Repository) ListOverrides(ctx context.Context, start, end time.Time) (map[string]Override, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tanggal::text, kategori, COALESCE(keterangan,'')
		FROM day_category_override
		WHERE tanggal BETWEEN $1::date AND $2::date`,
		formatDate(start), formatDate(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]Override{}
	for rows.Next() {
		var o Override
		if err := rows.Scan(&o.Tanggal, &o.Kategori, &o.Keterangan); err != nil {
			return nil, err
		}
		out[o.Tanggal] = o
	}
	return out, rows.Err()
}

func (r *Repository) UpsertOverride(ctx context.Context, tanggal, kategori, keterangan string, userID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO day_category_override (tanggal, kategori, keterangan, created_by, updated_by)
		VALUES ($1::date, $2, $3, $4, $4)
		ON CONFLICT (tanggal) DO UPDATE SET
			kategori = EXCLUDED.kategori,
			keterangan = EXCLUDED.keterangan,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()`,
		tanggal, kategori, keterangan, userID)
	return err
}

func (r *Repository) DeleteOverride(ctx context.Context, tanggal string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM day_category_override WHERE tanggal = $1::date`, tanggal)
	return err
}

func (r *Repository) EffectiveCategories(ctx context.Context, start, end time.Time) ([]DayCategoryItem, error) {
	overrides, err := r.ListOverrides(ctx, start, end)
	if err != nil {
		return nil, err
	}

	out := make([]DayCategoryItem, 0, len(dateRange(start, end)))
	for _, d := range dateRange(start, end) {
		key := formatDate(d)
		var ov *Override
		if o, ok := overrides[key]; ok {
			ov = &o
		}
		cat, source := effectiveCategory(d, ov)
		item := DayCategoryItem{Tanggal: key, Kategori: cat, Source: source}
		if ov != nil {
			item.Keterangan = ov.Keterangan
		}
		out = append(out, item)
	}
	return out, nil
}

// GetData mengambil seluruh ritase dalam rentang lalu mengagregasi harian,
// mingguan, dan bulanan di Go.
func (r *Repository) GetData(ctx context.Context, start, end time.Time) (*Response, error) {
	vehicles, err := r.ListVehicles(ctx)
	if err != nil {
		return nil, err
	}
	trips, err := r.ListTrips(ctx, start, end)
	if err != nil {
		return nil, err
	}
	overrides, err := r.ListOverrides(ctx, start, end)
	if err != nil {
		return nil, err
	}

	vehicleByID := make(map[int64]*vehicleMeta, len(vehicles))
	for _, v := range vehicles {
		rule, metric, min, max := ruleFor(v.JenisKendaraan, v.KapasitasKoli, v.KapasitasKg)
		vehicleByID[v.IDKendaraan] = &vehicleMeta{
			ID:     v.IDKendaraan,
			Plat:   v.PlatNomor,
			Jenis:  normalizeJenis(v.JenisKendaraan),
			Metric: metric,
			Min:    min,
			Max:    max,
			AWBMax: rule.MaxAWB,
			Active: !strings.EqualFold(strings.TrimSpace(v.Status), "nonaktif"),
		}
	}

	resolved := make([]resolvedTrip, 0, len(trips))
	hasTrip := make(map[int64]bool, len(trips))
	calibrationLoads := map[string][]int{}
	coverage := Coverage{TotalTrips: len(trips)}

	for _, t := range trips {
		date, err := parseDate(t.Tanggal)
		if err != nil {
			continue
		}

		meta := vehicleByID[t.IDKendaraan]
		if meta == nil {
			rule, metric, min, max := ruleFor(t.JenisKendaraan, t.KapasitasKoli, t.KapasitasKg)
			meta = &vehicleMeta{
				ID:     t.IDKendaraan,
				Plat:   t.PlatNomor,
				Jenis:  normalizeJenis(t.JenisKendaraan),
				Metric: metric,
				Min:    min,
				Max:    max,
				AWBMax: rule.MaxAWB,
				Active: true,
			}
			vehicleByID[t.IDKendaraan] = meta
		}

		koli, koliSource := resolveKoli(t)
		awb, awbSource := resolveAWB(t)

		var load int
		var source string
		var hasData bool
		if meta.Metric == MetricAWB {
			load, source = awb, awbSource
			hasData = awb > 0
		} else {
			load, source = koli, koliSource
			hasData = koli > 0
		}

		if koli > 0 {
			coverage.TripsWithKoli++
		}
		if awb > 0 {
			coverage.TripsWithAWB++
		}

		var pct *float64
		status := StatusNoData
		if hasData {
			pct = pctFor(load, 1, meta.Max)
			if pct != nil {
				status = statusFor(*pct, meta.Min, meta.Max)
			}
			calibrationLoads[meta.Jenis] = append(calibrationLoads[meta.Jenis], load)
		}

		ov := overrideFor(overrides, date)
		dayCat, _ := effectiveCategory(date, ov)

		ritaseKe := t.RitaseKe
		if ritaseKe < 1 || ritaseKe > 3 {
			ritaseKe = 1
		}

		detail := TripDetail{
			IDRitase:       t.IDRitase,
			Tanggal:        t.Tanggal,
			RitaseKe:       ritaseKe,
			WeekStart:      formatDate(weekStart(date)),
			DayCategory:    dayCat,
			IDKendaraan:    t.IDKendaraan,
			PlatNomor:      meta.Plat,
			JenisKendaraan: meta.Jenis,
			NamaDriver:     t.NamaDriver,
			Metric:         meta.Metric,
			CapacityMin:    meta.Min,
			CapacityMax:    meta.Max,
			Load:           load,
			Pct:            pct,
			Status:         status,
			HasData:        hasData,
			Source:         source,
			Koli:           koli,
			AWB:            awb,
		}

		hasTrip[t.IDKendaraan] = true
		resolved = append(resolved, resolvedTrip{
			Detail:    detail,
			HasData:   hasData,
			Load:      load,
			Koli:      koli,
			AWB:       awb,
			Date:      date,
			WeekKey:   detail.WeekStart,
			DayCat:    dayCat,
			VehicleID: t.IDKendaraan,
			RitaseKe:  ritaseKe,
		})
	}

	if coverage.TotalTrips > 0 {
		coverage.KoliPct = float64(coverage.TripsWithKoli) / float64(coverage.TotalTrips) * 100
		coverage.AWBPct = float64(coverage.TripsWithAWB) / float64(coverage.TotalTrips) * 100
	}

	included := make([]*vehicleMeta, 0, len(vehicleByID))
	for _, vm := range vehicleByID {
		if vm.Active || hasTrip[vm.ID] {
			included = append(included, vm)
		}
	}
	sort.Slice(included, func(i, j int) bool {
		if included[i].Plat == included[j].Plat {
			return included[i].ID < included[j].ID
		}
		return included[i].Plat < included[j].Plat
	})

	dailyMap := map[string]map[int64]*dailyAgg{}
	weeklyMap := map[string]map[int64]*weeklyAgg{}

	for _, rt := range resolved {
		dKey := formatDate(rt.Date)
		if dailyMap[dKey] == nil {
			dailyMap[dKey] = map[int64]*dailyAgg{}
		}
		da := dailyMap[dKey][rt.VehicleID]
		if da == nil {
			da = &dailyAgg{}
			dailyMap[dKey][rt.VehicleID] = da
		}
		da.Rits[rt.RitaseKe].add(rt.HasData, rt.Load, rt.Koli, rt.AWB)
		da.Total.add(rt.HasData, rt.Load, rt.Koli, rt.AWB)

		if weeklyMap[rt.WeekKey] == nil {
			weeklyMap[rt.WeekKey] = map[int64]*weeklyAgg{}
		}
		wa := weeklyMap[rt.WeekKey][rt.VehicleID]
		if wa == nil {
			wa = &weeklyAgg{Categories: map[Category]*agg{}}
			weeklyMap[rt.WeekKey][rt.VehicleID] = wa
		}
		if wa.Categories[rt.DayCat] == nil {
			wa.Categories[rt.DayCat] = &agg{}
		}
		wa.Categories[rt.DayCat].add(rt.HasData, rt.Load, rt.Koli, rt.AWB)
		wa.Total.add(rt.HasData, rt.Load, rt.Koli, rt.AWB)
	}

	dailyGroups := make([]DailyGroup, 0, len(dateRange(start, end)))
	for _, d := range dateRange(start, end) {
		key := formatDate(d)
		ov := overrideFor(overrides, d)
		cat, _ := effectiveCategory(d, ov)
		group := DailyGroup{Tanggal: key, DayCategory: cat, Vehicles: make([]DailyVehicle, 0, len(included))}

		for _, vm := range included {
			var da *dailyAgg
			if m := dailyMap[key]; m != nil {
				da = m[vm.ID]
			}
			dv := DailyVehicle{
				IDKendaraan:    vm.ID,
				PlatNomor:      vm.Plat,
				JenisKendaraan: vm.Jenis,
				Metric:         vm.Metric,
				CapacityMin:    vm.Min,
				CapacityMax:    vm.Max,
				Rits:           make([]DailyRit, 0, 3),
			}
			for rit := 1; rit <= 3; rit++ {
				var a agg
				if da != nil {
					a = da.Rits[rit]
				}
				dv.Rits = append(dv.Rits, DailyRit{
					RitaseKe: rit,
					Cell:     cellFor(a.Load, a.Trips, a.TripsWithData, vm.Min, vm.Max),
				})
			}
			var total agg
			if da != nil {
				total = da.Total
			}
			dv.Avg = cellFor(total.Load, total.Trips, total.TripsWithData, vm.Min, vm.Max)
			group.Vehicles = append(group.Vehicles, dv)
		}
		dailyGroups = append(dailyGroups, group)
	}

	weeklyGroups := make([]WeeklyGroup, 0)
	for w := weekStart(start); !w.After(weekStart(end)); w = w.AddDate(0, 0, 7) {
		key := formatDate(w)
		group := WeeklyGroup{
			WeekStart: key,
			WeekEnd:   formatDate(w.AddDate(0, 0, 6)),
			Vehicles:  make([]WeeklyVehicle, 0, len(included)),
		}
		for _, vm := range included {
			var wa *weeklyAgg
			if m := weeklyMap[key]; m != nil {
				wa = m[vm.ID]
			}
			wv := WeeklyVehicle{
				IDKendaraan:    vm.ID,
				PlatNomor:      vm.Plat,
				JenisKendaraan: vm.Jenis,
				Metric:         vm.Metric,
				CapacityMin:    vm.Min,
				CapacityMax:    vm.Max,
				Categories:     map[Category]Cell{},
			}
			for _, cat := range []Category{CategoryPeak, CategoryNormal, CategoryLow} {
				var a agg
				if wa != nil && wa.Categories[cat] != nil {
					a = *wa.Categories[cat]
				}
				wv.Categories[cat] = cellFor(a.Load, a.Trips, a.TripsWithData, vm.Min, vm.Max)
			}
			var total agg
			if wa != nil {
				total = wa.Total
			}
			wv.Total = cellFor(total.Load, total.Trips, total.TripsWithData, vm.Min, vm.Max)
			group.Vehicles = append(group.Vehicles, wv)
		}
		weeklyGroups = append(weeklyGroups, group)
	}

	monthlyGroups := make([]MonthlyGroup, 0)
	for m := monthStart(start); !m.After(monthStart(end)); m = m.AddDate(0, 1, 0) {
		mStart := m
		if mStart.Before(start) {
			mStart = start
		}
		mEnd := monthEnd(m)
		if mEnd.After(end) {
			mEnd = end
		}
		wFrom := weekStart(mStart)
		wTo := weekStart(mEnd)

		group := MonthlyGroup{Month: m.Format("2006-01"), Vehicles: make([]MonthlyVehicle, 0, len(included))}
		for _, vm := range included {
			mv := MonthlyVehicle{
				IDKendaraan:    vm.ID,
				PlatNomor:      vm.Plat,
				JenisKendaraan: vm.Jenis,
				Metric:         vm.Metric,
				CapacityMin:    vm.Min,
				CapacityMax:    vm.Max,
				Categories:     map[Category]MonthCell{},
			}
			for _, cat := range []Category{CategoryPeak, CategoryNormal, CategoryLow} {
				vals := []float64{}
				for w := wFrom; !w.After(wTo); w = w.AddDate(0, 0, 7) {
					key := formatDate(w)
					wm := weeklyMap[key]
					if wm == nil || wm[vm.ID] == nil || wm[vm.ID].Categories[cat] == nil {
						continue
					}
					a := wm[vm.ID].Categories[cat]
					if a.TripsWithData > 0 && vm.Max > 0 {
						vals = append(vals, float64(a.Load)/float64(vm.Max*a.TripsWithData)*100)
					}
				}
				cell := MonthCell{Weeks: len(vals), Status: StatusNoData}
				if len(vals) > 0 {
					sum := 0.0
					for _, v := range vals {
						sum += v
					}
					avg := sum / float64(len(vals))
					cell.AvgPct = &avg
					cell.Status = statusFor(avg, vm.Min, vm.Max)
				}
				mv.Categories[cat] = cell
			}
			group.Vehicles = append(group.Vehicles, mv)
		}
		monthlyGroups = append(monthlyGroups, group)
	}

	capacities := make([]CapacityInfo, 0, len(capacityRules))
	seen := map[string]bool{}
	for _, vm := range included {
		if seen[vm.Jenis] {
			continue
		}
		seen[vm.Jenis] = true
		capacities = append(capacities, CapacityInfo{
			Jenis:  vm.Jenis,
			Metric: vm.Metric,
			Min:    vm.Min,
			Max:    vm.Max,
			AWBMax: vm.AWBMax,
		})
	}
	sort.Slice(capacities, func(i, j int) bool { return capacities[i].Jenis < capacities[j].Jenis })

	calibration := make([]Calibration, 0, len(calibrationLoads))
	for jenis, vals := range calibrationLoads {
		if len(vals) == 0 {
			continue
		}
		_, metric, _, _ := ruleFor(jenis, 0, 0)
		calibration = append(calibration, Calibration{
			Jenis:  jenis,
			Metric: metric,
			Trips:  len(vals),
			P95:    p95(vals),
			Min:    minInt(vals),
			Max:    maxInt(vals),
		})
	}
	sort.Slice(calibration, func(i, j int) bool { return calibration[i].Jenis < calibration[j].Jenis })

	tripDetails := make([]TripDetail, 0, len(resolved))
	for _, rt := range resolved {
		tripDetails = append(tripDetails, rt.Detail)
	}

	return &Response{
		StartDate: formatDate(start),
		EndDate:   formatDate(end),
		Meta: Meta{
			Coverage:    coverage,
			Capacities:  capacities,
			Calibration: calibration,
		},
		Daily:   dailyGroups,
		Weekly:  weeklyGroups,
		Monthly: monthlyGroups,
		Trips:   tripDetails,
	}, nil
}

// GetEffectiveness menghitung ringkasan efektivitas armada tanpa membangun
// matriks harian/mingguan/bulanan seperti GetData.
func (r *Repository) GetEffectiveness(ctx context.Context, start, end time.Time) (*EffectivenessSummary, error) {
	vehicles, err := r.ListVehicles(ctx)
	if err != nil {
		return nil, err
	}
	trips, err := r.ListTrips(ctx, start, end)
	if err != nil {
		return nil, err
	}

	vehicleByID := make(map[int64]*vehicleMeta, len(vehicles))
	for _, v := range vehicles {
		rule, metric, min, max := ruleFor(v.JenisKendaraan, v.KapasitasKoli, v.KapasitasKg)
		vehicleByID[v.IDKendaraan] = &vehicleMeta{
			ID:     v.IDKendaraan,
			Plat:   v.PlatNomor,
			Jenis:  normalizeJenis(v.JenisKendaraan),
			Metric: metric,
			Min:    min,
			Max:    max,
			AWBMax: rule.MaxAWB,
			Active: !strings.EqualFold(strings.TrimSpace(v.Status), "nonaktif"),
		}
	}

	resolved := make([]effectivenessTrip, 0, len(trips))
	hasTrip := make(map[int64]bool, len(trips))
	for _, t := range trips {
		meta := vehicleByID[t.IDKendaraan]
		if meta == nil {
			rule, metric, min, max := ruleFor(t.JenisKendaraan, t.KapasitasKoli, t.KapasitasKg)
			meta = &vehicleMeta{
				ID:     t.IDKendaraan,
				Plat:   t.PlatNomor,
				Jenis:  normalizeJenis(t.JenisKendaraan),
				Metric: metric,
				Min:    min,
				Max:    max,
				AWBMax: rule.MaxAWB,
				Active: true,
			}
			vehicleByID[t.IDKendaraan] = meta
		}

		hasTrip[t.IDKendaraan] = true
		koli, _ := resolveKoli(t)
		awb, _ := resolveAWB(t)
		load := koli
		hasData := koli > 0
		if meta.Metric == MetricAWB {
			load = awb
			hasData = awb > 0
		}

		resolved = append(resolved, effectivenessTrip{
			Meta:    *meta,
			Load:    load,
			HasData: hasData,
		})
	}

	totalVehicles := 0
	for id, meta := range vehicleByID {
		if meta.Active || hasTrip[id] {
			totalVehicles++
		}
	}

	summary := buildEffectiveness(
		formatDate(start),
		formatDate(end),
		len(trips),
		totalVehicles,
		resolved,
	)
	return &summary, nil
}

func resolveKoli(t tripRow) (int, string) {
	switch t.GatewayAkhir {
	case "JKT":
		if t.EventKoliJKT+t.EventKoliSEG > 0 {
			return t.EventKoliJKT, "event"
		}
		if t.KonfirmasiKoliJKT+t.KonfirmasiKoliSEG > 0 {
			return t.KonfirmasiKoliJKT, "konfirmasi"
		}
	case "SEG":
		if t.EventKoliJKT+t.EventKoliSEG > 0 {
			return t.EventKoliSEG, "event"
		}
		if t.KonfirmasiKoliJKT+t.KonfirmasiKoliSEG > 0 {
			return t.KonfirmasiKoliSEG, "konfirmasi"
		}
	}

	switch {
	case t.EventKoli > 0:
		return t.EventKoli, "event"
	case t.KonfirmasiKoli > 0:
		return t.KonfirmasiKoli, "konfirmasi"
	case t.RitaseKoli > 0:
		return t.RitaseKoli, "ritase"
	default:
		return 0, ""
	}
}

func resolveAWB(t tripRow) (int, string) {
	switch {
	case t.KonfirmasiAWB > 0:
		return t.KonfirmasiAWB, "konfirmasi"
	case t.RitaseAWB > 0:
		return t.RitaseAWB, "ritase"
	default:
		return 0, ""
	}
}

func overrideFor(overrides map[string]Override, t time.Time) *Override {
	if o, ok := overrides[formatDate(t)]; ok {
		return &o
	}
	return nil
}

func minInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func maxInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
