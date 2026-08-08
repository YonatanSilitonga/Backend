package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository mengakses data agregat untuk dashboard.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
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

	// Armada online = ada posisi terbaru ≤ 5 menit (GPS fresh).
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM armada_tracking
		WHERE last_update > now() - interval '5 minutes'
	`).Scan(&s.ArmadaOnline); err != nil {
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
			Tingkat:  "warning",
			Kategori: "kendaraan_berhenti",
			Pesan:    fmt.Sprintf("Ritase %s berhenti lebih dari %s tanpa update", kode, dur.Round(time.Minute)),
			Waktu:    time.Now(),
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
			Tingkat:  "critical",
			Kategori: "perjalanan_terlalu_lama",
			Pesan:    fmt.Sprintf("Ritase %s perjalanan melebihi 8 jam", kode),
			Waktu:    time.Now(),
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
