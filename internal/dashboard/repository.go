package dashboard

import (
	"context"
	"database/sql"
	"fmt"
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
func (r *Repository) GetSummary(ctx context.Context) (*Summary, error) {
	s := &Summary{}
	today := time.Now().Format("2006-01-02")

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
		       count(*) FILTER (WHERE LOWER(status_driver) IN ('libur','off','cuti'))
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
		       COALESCE(sum(total_koli),0),
		       COALESCE(sum(paket_tertinggal),0)
		FROM ritase
	`, today).Scan(&s.TotalRitase, &s.RitaseAktif, &s.RitaseSelesai, &s.RitaseToday,
		&s.TotalAWB, &s.TotalAWBToday, &s.TotalKoli, &s.PaketTertinggal); err != nil {
		return nil, err
	}

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
// Pakai pasangan status: loading = mulai_loading -> selesai_loading,
// perjalanan = berangkat_gudang -> sampai_gudang, unloading = mulai_unloading -> selesai_unloading.
func (r *Repository) GetDurasiAnalisis(ctx context.Context) (*DurasiAnalisis, error) {
	d := &DurasiAnalisis{}

	pairs := []struct {
		start string
		end   string
		dst   *string
	}{
		{"mulai_loading", "selesai_loading", &d.RataRataLoading},
		{"berangkat_gudang", "sampai_gudang", &d.RataRataPerjalanan},
		{"mulai_unloading", "selesai_unloading", &d.RataRataUnloading},
	}

	for _, p := range pairs {
		var avgSeconds *float64
		err := r.db.QueryRow(ctx, `
			SELECT avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)))
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = $2
			WHERE e1.status = $1 AND e2.created_at > e1.created_at
		`, p.start, p.end).Scan(&avgSeconds)
		if err != nil {
			return nil, err
		}
		if avgSeconds != nil {
			*p.dst = formatDuration(*avgSeconds)
		} else {
			*p.dst = "belum ada data"
		}
	}

	if err := r.db.QueryRow(ctx, `
		SELECT count(DISTINCT id_ritase) FROM ritase_event
		WHERE status IN ('mulai_loading','berangkat_gudang','mulai_unloading')
	`).Scan(&d.TotalRitaseDihitung); err != nil {
		return nil, err
	}

	return d, nil
}

// GetBottleneck mendeteksi titik-titik hambatan dari data existing.
func (r *Repository) GetBottleneck(ctx context.Context) ([]Bottleneck, error) {
	var items []Bottleneck

	// seller dengan ritase paling banyak tapi lama tidak selesai
	// (relasi seller via ritase_stop — skema baru, ritase gak punya id_seller lagi)
	rows, err := r.db.Query(ctx, `
		SELECT 'seller', s.nama_seller, 'ritase terbanyak', count(DISTINCT rs.id_ritase)::float8
		FROM ritase_stop rs
		JOIN ritase r ON r.id_ritase = rs.id_ritase
		JOIN seller s ON s.id_seller = rs.id_seller
		WHERE rs.jenis_stop = 'seller' AND r.status NOT IN ('selesai','completed','done')
		GROUP BY s.nama_seller
		ORDER BY count(DISTINCT rs.id_ritase) DESC
		LIMIT 3
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b Bottleneck
		if err := rows.Scan(&b.Kategori, &b.Label, &b.Indikator, &b.Nilai); err != nil {
			rows.Close()
			return nil, err
		}
		b.Deskripsi = "Seller ini paling sering disinggahi ritase yang belum selesai — indikasi antrean loading atau kapasitas bongkar yang lambat."
		b.Rekomendasi = "Cek antrean & jadwal kunjungan seller, pertimbangkan tambah slot atau relokasi ritase ke driver lain."
		items = append(items, b)
	}
	rows.Close()

	// driver dengan jumlah paket tertinggal terbesar
	rows, err = r.db.Query(ctx, `
		SELECT 'driver', d.nama_driver, 'paket tertinggal', COALESCE(sum(r.paket_tertinggal),0)::float8
		FROM ritase r JOIN driver d ON d.id_driver = r.id_driver
		GROUP BY d.nama_driver
		ORDER BY COALESCE(sum(r.paket_tertinggal),0) DESC
		LIMIT 3
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b Bottleneck
		if err := rows.Scan(&b.Kategori, &b.Label, &b.Indikator, &b.Nilai); err != nil {
			rows.Close()
			return nil, err
		}
		b.Deskripsi = "Driver dengan total paket tertinggal terbesar — indikasi pengecekan muatan kurang ketat saat loading."
		b.Rekomendasi = "Evaluasi SOP pengecekan paket, pantau daftar tertinggal, dan konfirmasi ke driver sebelum berangkat."
		items = append(items, b)
	}
	rows.Close()

	return items, rows.Err()
}

// GetAlerts mendeteksi anomali yang perlu notifikasi.
func (r *Repository) GetAlerts(ctx context.Context) ([]AlertAnomali, error) {
	var items []AlertAnomali

	// ritase berjalan lama tanpa update (kendaraan berhenti terlalu lama)
	rows, err := r.db.Query(ctx, `
		SELECT r.kode_ritase, now() - max(e.created_at)
		FROM ritase r
		JOIN ritase_event e ON e.id_ritase = r.id_ritase
		WHERE r.status NOT IN ('selesai','completed','done','batal','cancelled')
		GROUP BY r.kode_ritase
		HAVING now() - max(e.created_at) > interval '3 hours'
		ORDER BY now() - max(e.created_at) DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kode string
		var dur time.Duration
		if err := rows.Scan(&kode, &dur); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, AlertAnomali{
			Tingkat:     "warning",
			Kategori:    "kendaraan_berhenti",
			Pesan:       fmt.Sprintf("Ritase %s berhenti lebih dari %s tanpa update", kode, dur.Round(time.Minute)),
			Waktu:       time.Now(),
			Deskripsi:   "Kendaraan tidak mengirim update GPS lebih dari 3 jam padahal ritase belum selesai — berpotensi berhenti di jalan, HP mati, atau kendala armada.",
			Rekomendasi: "Hubungi driver segera, cek posisi terakhir, dan pastikan kondisi armada serta keselamatan muatan.",
		})
	}
	rows.Close()

	// ritase yang sudah melewati batas wajar (jeda berangkat->tiba > 8 jam)
	rows, err = r.db.Query(ctx, `
		SELECT r.kode_ritase
		FROM ritase r
		JOIN ritase_event e1 ON e1.id_ritase = r.id_ritase AND e1.status = 'berangkat_gudang'
		JOIN ritase_event e2 ON e2.id_ritase = r.id_ritase AND e2.status = 'sampai_gudang'
		WHERE r.status NOT IN ('selesai','completed','done','batal','cancelled')
		  AND EXTRACT(EPOCH FROM (e2.created_at - e1.created_at)) > 8*3600
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kode string
		if err := rows.Scan(&kode); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, AlertAnomali{
			Tingkat:     "critical",
			Kategori:    "perjalanan_terlalu_lama",
			Pesan:       fmt.Sprintf("Ritase %s perjalanan melebihi 8 jam", kode),
			Waktu:       time.Now(),
			Deskripsi:   "Durasi berangkat gudang → tiba melebihi 8 jam — di luar batas wajar rute operasional, bisa menandakan kemacetan parah, menyimpang dari rute, atau kendala di jalan.",
			Rekomendasi: "Telaah ulang rute/jadwal ritase, konfirmasi ke driver penyebab keterlambatan, dan catat untuk evaluasi performa.",
		})
	}
	rows.Close()

	return items, nil
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%.0f detik", seconds)
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0f menit", d.Minutes())
	}
	return fmt.Sprintf("%.1f jam", d.Hours())
}

// defaultRange mengisi from/to (YYYY-MM-DD): kosong → 30 hari terakhir (to = hari ini).
func defaultRange(from, to string) (string, string) {
	if from == "" {
		from = time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
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
		)
		SELECT r.tanggal::text,
		       count(*),
		       count(*) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       count(*) FILTER (WHERE LOWER(r.status) IN ('batal','cancelled')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(r.total_koli),0),
		       COALESCE(sd.n,0),
		       count(*) FILTER (WHERE `+arahSQL+` = 'outgoing'),
		       count(*) FILTER (WHERE `+arahSQL+` = 'incoming')
		FROM ritase r
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN seller_day sd ON sd.tanggal = r.tanggal
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
			&t.TotalAWB, &t.TotalKoli, &t.SellerTerlayani, &t.Outgoing, &t.Incoming); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// GetAnalyticsDrivers menghitung performa per driver dalam periode.
// Durasi (detik) dihitung per ritase via CTE pairing event, lalu dirata-rata per driver.
func (r *Repository) GetAnalyticsDrivers(ctx context.Context, from, to string) ([]DriverPerf, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH loading AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'selesai_loading'
			WHERE e1.status = 'mulai_loading' AND e2.created_at > e1.created_at
			GROUP BY e1.id_ritase
		), perjalanan AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'sampai_gudang'
			WHERE e1.status = 'berangkat_gudang' AND e2.created_at > e1.created_at
			GROUP BY e1.id_ritase
		), unloading AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'selesai_unloading'
			WHERE e1.status = 'mulai_unloading' AND e2.created_at > e1.created_at
			GROUP BY e1.id_ritase
		)
		SELECT d.id_driver, d.nama_driver,
		       count(DISTINCT r.id_ritase),
		       count(DISTINCT r.id_ritase) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(r.total_koli),0),
		       COALESCE(sum(r.paket_tertinggal),0),
		       count(*) FILTER (WHERE `+arahSQL+` = 'outgoing'),
		       count(*) FILTER (WHERE `+arahSQL+` = 'incoming'),
		       avg(l.dur), avg(p.dur), avg(u.dur)
		FROM ritase r
		JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN loading l ON l.id_ritase = r.id_ritase
		LEFT JOIN perjalanan p ON p.id_ritase = r.id_ritase
		LEFT JOIN unloading u ON u.id_ritase = r.id_ritase
		WHERE r.tanggal BETWEEN $1 AND $2
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
			&p.RitaseTotal, &p.RitaseSelesai, &p.TotalAWB, &p.TotalKoli, &p.PaketTertinggal,
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
// RataBongkar = rata-rata durasi di lokasi (sampai_seller → berangkat_seller) per ritase.
func (r *Repository) GetAnalyticsSellers(ctx context.Context, from, to string) ([]SellerAnalytics, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH loc_dur AS (
			SELECT e1.id_ritase, avg(EXTRACT(EPOCH FROM (e2.created_at - e1.created_at))) AS dur
			FROM ritase_event e1
			JOIN ritase_event e2 ON e2.id_ritase = e1.id_ritase AND e2.status = 'berangkat_seller'
			WHERE e1.status = 'sampai_seller' AND e2.created_at > e1.created_at
			GROUP BY e1.id_ritase
		)
		SELECT s.id_seller, COALESCE(s.kode_seller,''), COALESCE(s.nama_seller,''), COALESCE(s.kota,''),
		       s.jarak_tempuh_km, s.jarak_dc_km,
		       count(DISTINCT r.id_ritase),
		       count(DISTINCT r.id_ritase) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(r.total_koli),0),
		       avg(ld.dur)
		FROM seller s
		JOIN ritase_stop rs ON rs.id_seller = s.id_seller AND rs.jenis_stop = 'seller'
		JOIN ritase r ON r.id_ritase = rs.id_ritase
		LEFT JOIN loc_dur ld ON ld.id_ritase = r.id_ritase
		WHERE r.tanggal BETWEEN $1 AND $2
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
			&s.Kunjungan, &s.RitaseSelesai, &s.TotalAWB, &s.TotalKoli, &bongkar); err != nil {
			return nil, err
		}
		if bongkar.Valid {
			s.RataBongkar = &bongkar.Float64
		}
		items = append(items, s)
	}
	return items, rows.Err()
}
