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
func (r *Repository) GetSummary(ctx context.Context) (*Summary, error) {
	s := &Summary{}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	today := time.Now().In(loc).Format("2006-01-02")
	yesterday := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")

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
               count(*) FILTER (WHERE tanggal = $2),
               COALESCE(sum(total_awb),0),
               COALESCE(sum(total_awb) FILTER (WHERE tanggal = $1),0),
               COALESCE(sum(total_awb) FILTER (WHERE tanggal = $2),0),
               COALESCE(sum(total_koli),0),
               COALESCE(sum(total_koli) FILTER (WHERE tanggal = $1),0),
               COALESCE(sum(total_koli) FILTER (WHERE tanggal = $2),0)
        FROM ritase
    `, today, yesterday).Scan(
		&s.TotalRitase, &s.RitaseAktif, &s.RitaseSelesai,
		&s.RitaseToday, &s.RitaseYesterday,
		&s.TotalAWB, &s.TotalAWBToday, &s.TotalAWBYesterday,
		&s.TotalKoli, &s.TotalKoliToday, &s.TotalKoliYesterday,
	); err != nil {
		return nil, err
	}

	// Muatan hari ini & kemarin — SUM dari event "Bongkar Muat Barang" per ritase.
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
	`, yesterday).Scan(&s.TotalKoliYesterday, &s.TotalHighValueYesterday, &s.TotalEceranYesterday); err != nil {
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
// Dioptimasi menggunakan Window Function LEAD() — 1 Single Pass scan tanpa perbandingan kuadratik.
func (r *Repository) GetDurasiAnalisis(ctx context.Context) (*DurasiAnalisis, error) {
	d := &DurasiAnalisis{}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	today := time.Now().In(loc).Format("2006-01-02")
	yesterday := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")

	var avgLoading, avgJalan *float64
	var totalDihitung int64

	// Overall averages
	err := r.db.QueryRow(ctx, `
		WITH stepped AS (
			SELECT e.id_ritase, e.status, e.created_at,
			       LEAD(e.status) OVER (PARTITION BY e.id_ritase ORDER BY e.created_at) AS next_status,
			       LEAD(e.created_at) OVER (PARTITION BY e.id_ritase ORDER BY e.created_at) AS next_time
			FROM ritase_event e
			JOIN ritase r ON r.id_ritase = e.id_ritase
			WHERE r.status IN ('selesai', 'berjalan')
		)
		SELECT 
			avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Tiba' AND next_status = 'Sedang Menuju'),
			avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Sedang Menuju' AND next_status = 'Tiba'),
			count(DISTINCT id_ritase)
		FROM stepped;
	`).Scan(&avgLoading, &avgJalan, &totalDihitung)

	if err != nil {
		return nil, err
	}

	if avgLoading != nil {
		d.RataRataLoading = formatJamMenit(time.Duration(*avgLoading) * time.Second)
		d.RataRataLoadingDetik = *avgLoading
	} else {
		d.RataRataLoading = "belum ada data"
	}

	if avgJalan != nil {
		d.RataRataPerjalanan = formatJamMenit(time.Duration(*avgJalan) * time.Second)
		d.RataRataPerjalananDetik = *avgJalan
	} else {
		d.RataRataPerjalanan = "belum ada data"
	}

	d.TotalRitaseDihitung = totalDihitung

	// Today averages
	var avgLoadingToday, avgJalanToday *float64
	_ = r.db.QueryRow(ctx, `
		WITH stepped AS (
			SELECT e.id_ritase, e.status, e.created_at,
			       LEAD(e.status) OVER (PARTITION BY e.id_ritase ORDER BY e.created_at) AS next_status,
			       LEAD(e.created_at) OVER (PARTITION BY e.id_ritase ORDER BY e.created_at) AS next_time
			FROM ritase_event e
			JOIN ritase r ON r.id_ritase = e.id_ritase
			WHERE r.status IN ('selesai', 'berjalan')
			  AND r.tanggal = $1
		)
		SELECT 
			avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Tiba' AND next_status = 'Sedang Menuju'),
			avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Sedang Menuju' AND next_status = 'Tiba')
		FROM stepped;
	`, today).Scan(&avgLoadingToday, &avgJalanToday)
	if avgLoadingToday != nil {
		d.RataRataLoadingDetik = *avgLoadingToday
	}
	if avgJalanToday != nil {
		d.RataRataPerjalananDetik = *avgJalanToday
	}

	// Yesterday averages
	var avgLoadingYesterday, avgJalanYesterday *float64
	_ = r.db.QueryRow(ctx, `
		WITH stepped AS (
			SELECT e.id_ritase, e.status, e.created_at,
			       LEAD(e.status) OVER (PARTITION BY e.id_ritase ORDER BY e.created_at) AS next_status,
			       LEAD(e.created_at) OVER (PARTITION BY e.id_ritase ORDER BY e.created_at) AS next_time
			FROM ritase_event e
			JOIN ritase r ON r.id_ritase = e.id_ritase
			WHERE r.status IN ('selesai', 'berjalan')
			  AND r.tanggal = $1
		)
		SELECT 
			avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Tiba' AND next_status = 'Sedang Menuju'),
			avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Sedang Menuju' AND next_status = 'Tiba')
		FROM stepped;
	`, yesterday).Scan(&avgLoadingYesterday, &avgJalanYesterday)
	if avgLoadingYesterday != nil {
		d.RataRataLoadingKemarinDetik = *avgLoadingYesterday
	}
	if avgJalanYesterday != nil {
		d.RataRataPerjalananKemarinDetik = *avgJalanYesterday
	}

	return d, nil
}

// GetBottleneck mendeteksi titik-titik hambatan dari data existing.
func (r *Repository) GetBottleneck(ctx context.Context) ([]Bottleneck, error) {
	return nil, nil
}

// GetAlerts mendeteksi anomali yang perlu notifikasi.
func (r *Repository) GetAlerts(ctx context.Context) ([]AlertAnomali, error) {
	var items []AlertAnomali

	// Auto-resolve alert untuk ritase yang sudah selesai.
	_, _ = r.db.Exec(ctx, `
		UPDATE alert_anomali SET is_resolved = true, resolved_at = now()
		WHERE is_resolved = false
		  AND id_ritase IN (SELECT id_ritase FROM ritase WHERE status IN ('selesai','completed','done'))
	`)

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
			Waktu:       lastEvent,
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
			Waktu:       lastEvent,
			Deskripsi:   "Durasi berangkat gudang → tiba melebihi 8 jam — di luar batas wajar rute operasional, bisa menandakan kemacetan parah, menyimpang dari rute, atau kendala di jalan.",
			Rekomendasi: "Telaah ulang rute/jadwal ritase, konfirmasi ke driver penyebab keterlambatan, dan catat untuk evaluasi performa.",
		})
	}
	rows.Close()

	// menuju berhenti terlalu lama — kendaraan status "Sedang Menuju" tapi diam > 15 menit.
	detectedRows, err := r.db.Query(ctx, `
		SELECT t.id_ritase, t.id_driver, t.id_kendaraan,
		       d.nama_driver, t.latitude, t.longitude, t.nama_lokasi,
		       EXTRACT(EPOCH FROM COALESCE(now() - t.stopped_since, now() - t.last_update))::int AS durasi_detik
		FROM armada_tracking t
		JOIN ritase r ON r.id_ritase = t.id_ritase
		JOIN driver d ON d.id_driver = t.id_driver
		WHERE t.status = 'Sedang Menuju'
		  AND r.status NOT IN ('selesai','completed','done','batal','cancelled')
		  AND (
		    (t.stopped_since IS NOT NULL AND now() - t.stopped_since > interval '1 minutes')
		    OR
		    (t.stopped_since IS NULL AND now() - t.last_update > interval '1 minutes')
		  )
		ORDER BY durasi_detik DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer detectedRows.Close()

	for detectedRows.Next() {
		var idRitase, idDriver, idKendaraan int64
		var namaDriver string
		var lat, lng *float64
		var namaLokasi *string
		var durasiDetik int
		if err := detectedRows.Scan(&idRitase, &idDriver, &idKendaraan, &namaDriver, &lat, &lng, &namaLokasi, &durasiDetik); err != nil {
			return nil, err
		}

		// Jika sudah ada alert aktif untuk ritase ini, baca dari DB dan tampilkan.
		var exists bool
		_ = r.db.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM alert_anomali WHERE id_ritase = $1 AND kategori = 'menuju_berhenti_lama' AND is_resolved = false)
		`, idRitase).Scan(&exists)
		if exists {
			var a AlertAnomali
			_ = r.db.QueryRow(ctx, `
				SELECT id_alert, id_ritase, tingkat, pesan, kategori, created_at, deskripsi, rekomendasi,
				       latitude, longitude, nama_lokasi, durasi_detik, is_resolved
				FROM alert_anomali
				WHERE id_ritase = $1 AND kategori = 'menuju_berhenti_lama' AND is_resolved = false
				ORDER BY created_at DESC LIMIT 1
			`, idRitase).Scan(&a.ID, &a.IDRitase, &a.Tingkat, &a.Pesan, &a.Kategori, &a.Waktu,
				&a.Deskripsi, &a.Rekomendasi, &a.Latitude, &a.Longitude, &a.NamaLokasi, &a.DurasiDetik, &a.IsResolved)
			items = append(items, a)
			continue
		}

		// Tentukan severity berdasarkan durasi.
		tingkat := "warning"
		if durasiDetik > 1800 { // > 30 menit
			tingkat = "critical"
		}

		pesan := fmt.Sprintf("Driver %s berhenti %s saat menuju ke lokasi", namaDriver, formatJamMenit(time.Duration(durasiDetik)*time.Second))
		deskripsi := fmt.Sprintf("Kendaraan berstatus \"Sedang Menuju\" namun tidak bergerak selama %s. Perlu dipastikan kondisi armada dan keselamatan muatan.", formatJamMenit(time.Duration(durasiDetik)*time.Second))
		rekomendasi := "Hubungi driver untuk memastikan kondisi. Jika kendala di jalan, siapkan bantuan."

		// Insert ke tabel alert_anomali.
		var alertID int64
		errInsert := r.db.QueryRow(ctx, `
			INSERT INTO alert_anomali (id_ritase, id_driver, id_kendaraan, kategori, tingkat, latitude, longitude, nama_lokasi, durasi_detik, pesan, deskripsi, rekomendasi)
			VALUES ($1, $2, $3, 'menuju_berhenti_lama', $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id_alert
		`, idRitase, idDriver, idKendaraan, tingkat, lat, lng, namaLokasi, durasiDetik, pesan, deskripsi, rekomendasi).Scan(&alertID)
		if errInsert != nil {
			continue
		}

		waktu := time.Now()
		items = append(items, AlertAnomali{
			ID:          alertID,
			IDRitase:    &idRitase,
			Tingkat:     tingkat,
			Pesan:       pesan,
			Kategori:    "menuju_berhenti_lama",
			Waktu:       waktu,
			Deskripsi:   deskripsi,
			Rekomendasi: rekomendasi,
			Latitude:    lat,
			Longitude:   lng,
			NamaLokasi:  namaLokasi,
			DurasiDetik: &durasiDetik,
			IsResolved:  false,
		})
	}

	// Fallback: baca semua alert menuju_berhenti_lama aktif dari tabel
	// agar tetap tampil meskipun query deteksi tidak return (GPS masih kirim update).
	existingRows, err := r.db.Query(ctx, `
		SELECT id_alert, id_ritase, tingkat, pesan, kategori, created_at, deskripsi, rekomendasi,
		       latitude, longitude, nama_lokasi, durasi_detik, is_resolved
		FROM alert_anomali
		WHERE kategori = 'menuju_berhenti_lama' AND is_resolved = false
		ORDER BY created_at DESC
		LIMIT 10
	`)
	if err == nil {
		defer existingRows.Close()
		for existingRows.Next() {
			var a AlertAnomali
			if err := existingRows.Scan(&a.ID, &a.IDRitase, &a.Tingkat, &a.Pesan, &a.Kategori, &a.Waktu,
				&a.Deskripsi, &a.Rekomendasi, &a.Latitude, &a.Longitude, &a.NamaLokasi, &a.DurasiDetik, &a.IsResolved); err != nil {
				continue
			}
			// Skip jika sudah ada di items (duplikat dari query deteksi).
			duplicate := false
			for _, existing := range items {
				if existing.ID == a.ID {
					duplicate = true
					break
				}
			}
			if !duplicate {
				items = append(items, a)
			}
		}
	}

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

// GetAnalyticsTrend menghitung trend harian dengan scoped muatan CTE.
func (r *Repository) GetAnalyticsTrend(ctx context.Context, from, to string) ([]TrendPoint, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH active_ritase AS (
			SELECT id_ritase, tanggal, status, total_awb, id_drop_point
			FROM ritase
			WHERE tanggal BETWEEN $1 AND $2
		),
		seller_day AS (
			SELECT r.tanggal, count(DISTINCT rs.id_seller) AS n
			FROM active_ritase r
			JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
			WHERE rs.jenis_stop = 'seller'
			  AND LOWER(r.status) IN ('selesai','completed','done')
			GROUP BY r.tanggal
		), 
		muatan AS (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.status = 'Bongkar Muat Barang'
			  AND ev.id_ritase IN (SELECT id_ritase FROM active_ritase)
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
		FROM active_ritase r
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN seller_day sd ON sd.tanggal = r.tanggal
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
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
// Dioptimasi dengan Window Function LEAD() untuk eliminasi 3x self-join yang lambat.
func (r *Repository) GetAnalyticsDrivers(ctx context.Context, from, to string) ([]DriverPerf, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH active_ritase AS (
			SELECT id_ritase, id_driver, id_drop_point, tanggal, status, total_awb
			FROM ritase
			WHERE tanggal BETWEEN $1 AND $2
			  AND status IN ('selesai', 'berjalan')
		),
		stepped_events AS (
			SELECT id_ritase, status, created_at,
			       LEAD(status) OVER (PARTITION BY id_ritase ORDER BY created_at) AS next_status,
			       LEAD(created_at) OVER (PARTITION BY id_ritase ORDER BY created_at) AS next_time
			FROM ritase_event
			WHERE id_ritase IN (SELECT id_ritase FROM active_ritase)
		),
		durasi_per_ritase AS (
			SELECT id_ritase,
			       avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Tiba' AND next_status = 'Sedang Menuju') AS loading_dur,
			       avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Sedang Menuju' AND next_status = 'Tiba') AS jalan_dur,
			       avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Bongkar Muat Barang' AND next_status = 'Selesai') AS unloading_dur
			FROM stepped_events
			GROUP BY id_ritase
		),
		muatan AS (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.status = 'Bongkar Muat Barang'
			  AND ev.id_ritase IN (SELECT id_ritase FROM active_ritase)
			GROUP BY ev.id_ritase
		)
		SELECT d.id_driver, d.nama_driver,
		       count(DISTINCT r.id_ritase),
		       count(DISTINCT r.id_ritase) FILTER (WHERE LOWER(r.status) IN ('selesai','completed','done')),
		       COALESCE(sum(r.total_awb),0),
		       COALESCE(sum(m.koli),0), COALESCE(sum(m.hv),0), COALESCE(sum(m.ecer),0),
		       count(*) FILTER (WHERE `+arahSQL+` = 'outgoing'),
		       count(*) FILTER (WHERE `+arahSQL+` = 'incoming'),
		       avg(dur.loading_dur), avg(dur.jalan_dur), avg(dur.unloading_dur)
		FROM active_ritase r
		JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
		LEFT JOIN durasi_per_ritase dur ON dur.id_ritase = r.id_ritase
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
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
// Dioptimasi dengan Window Function LEAD() & scoped active_ritase.
func (r *Repository) GetAnalyticsSellers(ctx context.Context, from, to string) ([]SellerAnalytics, error) {
	from, to = defaultRange(from, to)

	rows, err := r.db.Query(ctx, `
		WITH active_ritase AS (
			SELECT id_ritase, tanggal, status, total_awb
			FROM ritase
			WHERE tanggal BETWEEN $1 AND $2
			  AND status IN ('selesai', 'berjalan')
		),
		stepped_events AS (
			SELECT id_ritase, status, created_at,
			       LEAD(status) OVER (PARTITION BY id_ritase ORDER BY created_at) AS next_status,
			       LEAD(created_at) OVER (PARTITION BY id_ritase ORDER BY created_at) AS next_time
			FROM ritase_event
			WHERE id_ritase IN (SELECT id_ritase FROM active_ritase)
		),
		loc_dur AS (
			SELECT id_ritase,
			       avg(EXTRACT(EPOCH FROM (next_time - created_at))) FILTER (WHERE status = 'Tiba' AND next_status = 'Sedang Menuju') AS dur
			FROM stepped_events
			GROUP BY id_ritase
		),
		muatan AS (
			SELECT ev.id_ritase,
			       sum(ev.jumlah_koli) AS koli,
			       sum(ev.jumlah_high_value) AS hv,
			       sum(ev.jumlah_ecer) AS ecer
			FROM ritase_event ev
			WHERE ev.status = 'Bongkar Muat Barang'
			  AND ev.id_ritase IN (SELECT id_ritase FROM active_ritase)
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
		JOIN active_ritase r ON r.id_ritase = rs.id_ritase
		LEFT JOIN loc_dur ld ON ld.id_ritase = r.id_ritase
		LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
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

// ResolveAlert menandai alert sebagai sudah ditangani oleh tower control.
func (r *Repository) ResolveAlert(ctx context.Context, idAlert int64, userID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE alert_anomali
		SET is_resolved = true, resolved_by = $2, resolved_at = now()
		WHERE id_alert = $1 AND is_resolved = false
	`, idAlert, userID)
	return err
}

// GetAlertsByRitase mengembalikan riwayat alert untuk satu ritase.
func (r *Repository) GetAlertsByRitase(ctx context.Context, idRitase int64) ([]AlertAnomali, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_alert, id_ritase, kategori, tingkat, pesan, deskripsi, rekomendasi,
		       latitude, longitude, nama_lokasi, durasi_detik, is_resolved, created_at
		FROM alert_anomali
		WHERE id_ritase = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, idRitase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []AlertAnomali
	for rows.Next() {
		var a AlertAnomali
		if err := rows.Scan(&a.ID, &a.IDRitase, &a.Kategori, &a.Tingkat, &a.Pesan, &a.Deskripsi, &a.Rekomendasi,
			&a.Latitude, &a.Longitude, &a.NamaLokasi, &a.DurasiDetik, &a.IsResolved, &a.Waktu); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

// CleanupOldAlerts menghapus alert yang sudah resolved dan usianya > 30 hari.
func (r *Repository) CleanupOldAlerts(ctx context.Context) (int64, error) {
	result, err := r.db.Exec(ctx, `
		DELETE FROM alert_anomali
		WHERE is_resolved = true
		  AND resolved_at < now() - interval '30 days'
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
