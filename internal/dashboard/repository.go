package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository mengakses data agregat untuk dashboard.
type Repository struct {
	db *pgxpool.Pool
	// Ambang offline (menit tanpa GPS) — default 15.
	offlineMin int
	// Ambang session (jam sejak login) — default 12.
	sessionHours int
	// Wajib session aktif buat dihitung "online" (GPS fresh + login).
	sessionRequired bool
}

func NewRepository(db *pgxpool.Pool, offlineMin int, sessionHours int, sessionRequired bool) *Repository {
	if offlineMin <= 0 {
		offlineMin = 15
	}
	if sessionHours <= 0 {
		sessionHours = 12
	}
	return &Repository{db: db, offlineMin: offlineMin, sessionHours: sessionHours, sessionRequired: sessionRequired}
}

// GetSummary menghitung ringkasan KPI dari tabel-tabel existing.
// Query DIKONSOLIDASI (GROUP BY / FILTER) biar gak 24x roundtrip ke DB.
// GetSummary menghitung ringkasan KPI dari tabel-tabel existing.
// Query DIKONSOLIDASI (GROUP BY / FILTER) biar gak 24x roundtrip ke DB.
func (r *Repository) GetSummary(ctx context.Context) (*Summary, error) {
	s := &Summary{}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	today := time.Now().In(loc).Format("2006-01-02")

	// Kendaraan — 1 query (dulu 4)
	if err := r.db.QueryRow(ctx, `
        SELECT count(*),
               count(*) FILTER (WHERE LOWER(status_kendaraan) IN ('aktif','berjalan','bertugas','tersedia')),
               count(*) FILTER (WHERE LOWER(status_kendaraan) IN ('selesai','istirahat')),
               count(*) FILTER (WHERE LOWER(status_kendaraan) IN ('tersedia','idle','off'))
        FROM kendaraan
    `).Scan(&s.TotalKendaraan, &s.ArmadaAktif, &s.ArmadaSelesai, &s.ArmadaIdle); err != nil {
		return nil, err
	}

	// Driver — 1 query (dulu 3)
	if err := r.db.QueryRow(ctx, `
        SELECT count(*),
               count(*) FILTER (WHERE LOWER(status_driver) IN ('aktif','bertugas','on_duty')),
               count(*) FILTER (WHERE LOWER(status_driver) IN ('libur','off','cuti','nonaktif'))
        FROM driver
    `).Scan(&s.TotalDriver, &s.DriverAktif, &s.DriverLibur); err != nil {
		return nil, err
	}

	// Ritase + muatan — 1 query (dulu ~8)
	if err := r.db.QueryRow(ctx, `
        SELECT count(*),
               count(*) FILTER (WHERE LOWER(status) NOT IN ('selesai','completed','done','batal','cancelled')),
               count(*) FILTER (WHERE LOWER(status) IN ('selesai','completed','done')),
               count(*) FILTER (WHERE tanggal = $1),
               COALESCE(sum(total_awb),0),
               COALESCE(sum(total_awb) FILTER (WHERE tanggal = $1),0),
               COALESCE(sum(total_koli),0)
        FROM ritase
    `, today).Scan(&s.TotalRitase, &s.RitaseAktif, &s.RitaseSelesai, &s.RitaseToday,
		&s.TotalAWB, &s.TotalAWBToday, &s.TotalKoli); err != nil {
		return nil, err
	}

	// Muatan hari ini — SUM dari event "Bongkar Muat Barang" per ritase.
	t := time.Now()
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(sum(koli),0), COALESCE(sum(hv),0), COALESCE(sum(ecer),0)
		FROM (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE (ev.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date = $1 AND ev.status = 'Bongkar Muat Barang'
			GROUP BY ev.id_ritase
		) latest
	`, today).Scan(&s.TotalKoliToday, &s.TotalHighValueToday, &s.TotalEceranToday); err != nil {
		return nil, err
	}
	log.Printf("[TIMING] muatan_hari_ini: %v", time.Since(t))

	// Seller terlayani — 1 query (relasi via ritase_stop)
	if err := r.db.QueryRow(ctx, `
        SELECT count(DISTINCT rs.id_seller)
        FROM ritase_stop rs
        JOIN ritase r ON r.id_ritase = rs.id_ritase
        WHERE rs.jenis_stop = 'seller' AND LOWER(r.status) IN ('selesai','completed','done')
    `).Scan(&s.SellerTerlayani); err != nil {
		return nil, err
	}

	// Master lainnya — 1 query (scalar subquery)
	if err := r.db.QueryRow(ctx, `
        SELECT (SELECT count(*) FROM seller),
               (SELECT count(*) FROM drop_point),
               (SELECT count(*) FROM karyawan),
               (SELECT COALESCE(sum(jumlah_manpower),0) FROM implant),
               (SELECT count(*) FROM absensi),
               (SELECT count(*) FROM implant),
               (SELECT count(*) FROM armada_tracking)
    `).Scan(&s.TotalSeller, &s.TotalDropPoint, &s.TotalKaryawan,
		&s.TotalManpower, &s.TotalAbsensi, &s.TotalImplant, &s.TotalTracking); err != nil {
		return nil, err
	}

	// Armada online = ada posisi terbaru ≤ ambang offline (menit).
	// Saat sessionRequired: wajib juga punya session login aktif (anti ghost online).
	onlineSQL := fmt.Sprintf(`
        SELECT count(*) FROM armada_tracking
        WHERE last_update > now() - make_interval(mins => %d)
    `, r.offlineMin)
	if r.sessionRequired {
		onlineSQL = fmt.Sprintf(`
            SELECT count(*)
            FROM armada_tracking t
            JOIN driver d ON d.id_driver = t.id_driver
            JOIN users u ON u.id_driver = d.id_driver
            WHERE t.last_update > now() - make_interval(mins => %d)
              AND u.last_login IS NOT NULL
              AND u.last_login > now() - make_interval(hours => %d)
        `, r.offlineMin, r.sessionHours)
	}
	if err := r.db.QueryRow(ctx, onlineSQL).Scan(&s.ArmadaOnline); err != nil {
		return nil, err
	}

	// driver telat: ritase berjalan yang umurnya > 6 jam sejak event pertama
	if err := r.db.QueryRow(ctx, `
        SELECT count(DISTINCT r.id_driver)
        FROM ritase r
        JOIN ritase_event e ON e.id_ritase = r.id_ritase
        WHERE LOWER(r.status) NOT IN ('selesai','completed','done','batal','cancelled')
          AND e.created_at < now() - interval '6 hours'
    `).Scan(&s.DriverTelat); err != nil {
		return nil, err
	}

	return s, nil
}

// GetDurasiAnalisis menghitung rata-rata durasi proses dari timeline event.
// Hanya menghitung ritase yang benar-benar dijalankan (status selesai/berjalan).
// Pakai pasangan status stored di DB:
// loading (bongkar muat) = Tiba -> Sedang Menuju (waktu di lokasi)
// perjalanan = Sedang Menuju -> Tiba (waktu perjalanan)
func (r *Repository) GetDurasiAnalisis(ctx context.Context) (*DurasiAnalisis, error) {
	d := &DurasiAnalisis{}

	pairs := []struct {
		start string
		end   string
		dst   *string
	}{
		{"Tiba", "Sedang Menuju", &d.RataRataLoading},
		{"Sedang Menuju", "Tiba", &d.RataRataPerjalanan},
	}

	for _, p := range pairs {
		var avgSeconds *float64
		err := r.db.QueryRow(ctx, `
			SELECT avg(seg.dur)
			FROM (
				SELECT e1.id_ritase,
				       SUM(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
				FROM ritase_event e1
				JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = $2
				JOIN ritase r ON r.id_ritase = e1.id_ritase
				WHERE e1.status = $1 AND e2.created_at > e1.created_at
				  AND r.status IN ('selesai', 'berjalan')
				GROUP BY e1.id_ritase
			) seg
		`, p.start, p.end).Scan(&avgSeconds)
		if err != nil {
			return nil, err
		}
		if avgSeconds != nil {
			*p.dst = formatJamMenit(time.Duration(*avgSeconds) * time.Second)
		} else {
			*p.dst = "belum ada data"
		}
	}

	if err := r.db.QueryRow(ctx, `
		SELECT count(DISTINCT e.id_ritase) FROM ritase_event e
		JOIN ritase r ON r.id_ritase = e.id_ritase
		WHERE e.status IN ('Tiba','Sedang Menuju')
		  AND r.status IN ('selesai', 'berjalan')
	`).Scan(&d.TotalRitaseDihitung); err != nil {
		return nil, err
	}

	return d, nil
}

// GetBottleneck mendeteksi titik-titik hambatan dari data existing.
// Saat ini belum ada bottleneck yang cukup signifikan untuk ditampilkan.
// Alert (kendaraan berhenti lama, perjalanan terlalu lama) sudah cover fungsi ini.
func (r *Repository) GetBottleneck(ctx context.Context) ([]Bottleneck, error) {
	return nil, nil
}

// GetAlerts mendeteksi anomali yang perlu notifikasi.
func (r *Repository) GetAlerts(ctx context.Context) ([]AlertAnomali, error) {
	var items []AlertAnomali

	// ritase berjalan lama tanpa update (kendaraan berhenti terlalu lama)
	rows, err := r.db.Query(ctx, `
		SELECT r.kode_ritase, d.nama_driver, now() - max(e.created_at), max(e.created_at)
		FROM ritase r
		JOIN ritase_event e ON e.id_ritase = r.id_ritase
		JOIN driver d ON d.id_driver = r.id_driver
		WHERE r.status NOT IN ('selesai','completed','done','batal','cancelled')
		GROUP BY r.kode_ritase, d.nama_driver
		HAVING now() - max(e.created_at) > interval '3 hours'
		ORDER BY now() - max(e.created_at) DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kode, namaDriver string
		var dur time.Duration
		var lastEvent time.Time
		if err := rows.Scan(&kode, &namaDriver, &dur, &lastEvent); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, AlertAnomali{
			Tingkat:     "warning",
			Kategori:    "kendaraan_berhenti",
			Pesan:       fmt.Sprintf("Driver %s berhenti lebih dari %s tanpa update", namaDriver, formatJamMenit(dur)),
			Waktu:       lastEvent, // waktu kejadian asli (update GPS terakhir), bukan waktu query
			Deskripsi:   "Kendaraan tidak mengirim update GPS lebih dari 3 jam padahal ritase belum selesai berpotensi berhenti di jalan, HP mati, atau kendala armada.",
			Rekomendasi: "Hubungi driver segera, cek posisi terakhir, dan pastikan kondisi armada serta keselamatan muatan.",
		})
	}
	rows.Close()

	// ritase yang sudah melewati batas wajar (jeda berangkat->tiba > 8 jam)
	rows, err = r.db.Query(ctx, `
		SELECT r.kode_ritase, max(e2.created_at)
		FROM ritase r
		JOIN ritase_event e1 ON e1.id_ritase = r.id_ritase AND e1.status = 'Sedang Menuju'
		JOIN ritase_event e2 ON e2.id_ritase = r.id_ritase AND e2.status = 'Tiba'
		WHERE r.status NOT IN ('selesai','completed','done','batal','cancelled')
		  AND EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)) > 8*3600
		GROUP BY r.kode_ritase
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kode string
		var lastEvent time.Time
		if err := rows.Scan(&kode, &lastEvent); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, AlertAnomali{
			Tingkat:     "critical",
			Kategori:    "perjalanan_terlalu_lama",
			Pesan:       fmt.Sprintf("Ritase %s perjalanan melebihi 8 jam", kode),
			Waktu:       lastEvent, // waktu kejadian asli (tiba gudang yang telat), bukan waktu query
			Deskripsi:   "Durasi berangkat gudang → tiba melebihi 8 jam — di luar batas wajar rute operasional, bisa menandakan kemacetan parah, menyimpang dari rute, atau kendala di jalan.",
			Rekomendasi: "Telaah ulang rute/jadwal ritase, konfirmasi ke driver penyebab keterlambatan, dan catat untuk evaluasi performa.",
		})
	}
	rows.Close()

	return items, nil
}

// formatJamMenit mengubah time.Duration jadi "18 jam 18 menit" (tanpa detik).
func formatJamMenit(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) - h*60
	if h == 0 {
		return fmt.Sprintf("%d menit", m)
	}
	return fmt.Sprintf("%d jam %d menit", h, m)
}

// defaultRange mengisi from/to (YYYY-MM-DD): kosong → 30 hari terakhir (to = hari ini).
func defaultRange(from, to string) (string, string) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(loc)
	if from == "" {
		from = now.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	return from, to
}

// arahSQL adalah ekspresi CASE untuk klasifikasi arah ritase dari drop point (gateway).
const arahSQL = `CASE
	WHEN LOWER(COALESCE(dp.kode_dp,'')) LIKE '%jkt%' OR LOWER(COALESCE(dp.nama_drop_point,'')) LIKE '%jkt%' THEN 'outgoing'
	WHEN LOWER(COALESCE(dp.kode_dp,'')) LIKE '%seg%' OR LOWER(COALESCE(dp.nama_drop_point,'')) LIKE '%seg%' THEN 'incoming'
	ELSE 'lainnya'
END`

// GetAnalyticsTrend menghitung trend harian (GROUP BY ritase.tanggal — tanggal jadwal,
// bukan created_at, biar shift yang nyebrang tengah malam gak kepotong 2 hari).
func (r *Repository) GetAnalyticsTrend(ctx context.Context, from, to string) ([]TrendPoint, error) {
	from, to = defaultRange(from, to)

	// Seller terlayani per tanggal (CTE terpisah biar JOIN stop gak menduplikasi baris ritase).
	rows, err := r.db.Query(ctx, `
		WITH seller_day AS (
			SELECT r.tanggal, count(DISTINCT rs.id_seller) AS n
			FROM ritase r
			JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
			WHERE rs.jenis_stop = 'seller'
			  AND LOWER(r.status) IN ('selesai','completed','done')
			  AND r.tanggal BETWEEN $1 AND $2
			GROUP BY r.tanggal
		), muatan AS (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.status = 'Bongkar Muat Barang'
			  AND (ev.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date BETWEEN $1 AND $2
			GROUP BY ev.id_ritase
		)
		SELECT r.tanggal::text,
		       count(*),
		       count(*) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       count(*) FILTER (WHERE LOWER(r.status) IN ('batal','cancelled')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(m.koli),0),
		       COALESCE(sum(m.hv),0),
		       COALESCE(sum(m.ecer),0),
		       COALESCE(sd.n,0),
		       count(*) FILTER (WHERE `+arahSQL+` = 'outgoing'),
		       count(*) FILTER (WHERE `+arahSQL+` = 'incoming')
		FROM ritase r
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN seller_day sd ON sd.tanggal = r.tanggal
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
		WHERE r.tanggal BETWEEN $1 AND $2
		GROUP BY r.tanggal, sd.n
		ORDER BY r.tanggal
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TrendPoint
	for rows.Next() {
		var t TrendPoint
		if err := rows.Scan(&t.Tanggal, &t.RitaseTotal, &t.RitaseSelesai, &t.RitaseBatal,
			&t.TotalAWB, &t.TotalKoli, &t.TotalHighValue, &t.TotalEceran,
			&t.SellerTerlayani, &t.Outgoing, &t.Incoming); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// GetAnalyticsDrivers menghitung performa per driver dalam periode.
// Durasi (detik) dihitung per ritase via CTE pairing event, lalu dirata-rata per driver.
// Hanya ritase yang benar-benar dijalankan (status selesai/berjalan) yang dihitung.
func (r *Repository) GetAnalyticsDrivers(ctx context.Context, from, to string) ([]DriverPerf, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH active_ritase AS (
			SELECT id_ritase FROM ritase
			WHERE tanggal BETWEEN $1 AND $2
			  AND status IN ('selesai', 'berjalan')
		), loading AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'Sedang Menuju'
			WHERE e1.status = 'Tiba' AND e2.created_at > e1.created_at
			  AND e1.id_ritase IN (SELECT id_ritase FROM active_ritase)
			GROUP BY e1.id_ritase
		), perjalanan AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'Tiba'
			WHERE e1.status = 'Sedang Menuju' AND e2.created_at > e1.created_at
			  AND e1.id_ritase IN (SELECT id_ritase FROM active_ritase)
			GROUP BY e1.id_ritase
		), unloading AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'Selesai'
			WHERE e1.status = 'Bongkar Muat Barang' AND e2.created_at > e1.created_at
			  AND e1.id_ritase IN (SELECT id_ritase FROM active_ritase)
			GROUP BY e1.id_ritase
		), muatan AS (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.status = 'Bongkar Muat Barang'
			  AND (ev.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date BETWEEN $1 AND $2
			GROUP BY ev.id_ritase
		)
		SELECT d.id_driver, d.nama_driver,
		       count(DISTINCT r.id_ritase),
		       count(DISTINCT r.id_ritase) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(m.koli),0), COALESCE(sum(m.hv),0), COALESCE(sum(m.ecer),0),
		       count(*) FILTER (WHERE `+arahSQL+` = 'outgoing'),
		       count(*) FILTER (WHERE `+arahSQL+` = 'incoming'),
		       avg(l.dur), avg(p.dur), avg(u.dur)
		FROM ritase r
		JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN loading l ON l.id_ritase = r.id_ritase
		LEFT JOIN perjalanan p ON p.id_ritase = r.id_ritase
		LEFT JOIN unloading u ON u.id_ritase = r.id_ritase
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
		WHERE r.tanggal BETWEEN $1 AND $2
		  AND r.status IN ('selesai', 'berjalan')
		GROUP BY d.id_driver, d.nama_driver
		ORDER BY count(DISTINCT r.id_ritase) DESC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DriverPerf
	for rows.Next() {
		var p DriverPerf
		var loading, perjalanan, unloading sql.NullFloat64
		if err := rows.Scan(&p.IDDriver, &p.NamaDriver,
			&p.RitaseTotal, &p.RitaseSelesai, &p.TotalAWB, &p.TotalKoli,
			&p.TotalHighValue, &p.TotalEceran,
			&p.Outgoing, &p.Incoming, &loading, &perjalanan, &unloading); err != nil {
			return nil, err
		}
		if loading.Valid {
			p.RataLoading = &loading.Float64
		}
		if perjalanan.Valid {
			p.RataPerjalanan = &perjalanan.Float64
		}
		if unloading.Valid {
			p.RataUnloading = &unloading.Float64
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// GetAnalyticsSellers menghitung analitik per seller dalam periode.
// RataBongkar = rata-rata durasi di lokasi (Tiba -> Sedang Menuju) per ritase yang dijalankan.
func (r *Repository) GetAnalyticsSellers(ctx context.Context, from, to string) ([]SellerAnalytics, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH loc_dur AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'Sedang Menuju'
			JOIN ritase r ON r.id_ritase = e1.id_ritase
			WHERE e1.status = 'Tiba' AND e2.created_at > e1.created_at
			  AND r.status IN ('selesai', 'berjalan')
			GROUP BY e1.id_ritase
		), muatan AS (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.status = 'Bongkar Muat Barang'
			  AND (ev.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date BETWEEN $1 AND $2
			GROUP BY ev.id_ritase
		)
		SELECT s.id_seller, COALESCE(s.kode_seller,''), COALESCE(s.nama_seller,''), COALESCE(s.kota,''),
		       s.jarak_tempuh_km, s.jarak_dc_km,
		       count(DISTINCT r.id_ritase),
		       count(DISTINCT r.id_ritase) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(m.koli),0),
		       COALESCE(sum(m.hv),0),
		       COALESCE(sum(m.ecer),0),
		       avg(ld.dur)
		FROM seller s
		JOIN ritase_stop rs ON rs.id_seller = s.id_seller AND rs.jenis_stop = 'seller'
		JOIN ritase r ON r.id_ritase = rs.id_ritase
		LEFT JOIN loc_dur ld ON ld.id_ritase = r.id_ritase
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
		WHERE r.tanggal BETWEEN $1 AND $2
		  AND r.status IN ('selesai', 'berjalan')
		GROUP BY s.id_seller, s.kode_seller, s.nama_seller, s.kota, s.jarak_tempuh_km, s.jarak_dc_km
		ORDER BY count(DISTINCT r.id_ritase) DESC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SellerAnalytics
	for rows.Next() {
		var s SellerAnalytics
		var bongkar sql.NullFloat64
		if err := rows.Scan(&s.IDSeller, &s.KodeSeller, &s.NamaSeller, &s.Kota,
			&s.JarakTempuhKm, &s.JarakDcKm,
			&s.Kunjungan, &s.RitaseSelesai, &s.TotalAWB, &s.TotalKoli,
			&s.TotalHighValue, &s.TotalEceran, &bongkar); err != nil {
			return nil, err
		}
		if bongkar.Valid {
			s.RataBongkar = &bongkar.Float64
		}
		items = append(items, s)
	}
	return items, rows.Err()
}
