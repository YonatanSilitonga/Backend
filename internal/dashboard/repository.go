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
func (r *Repository) GetSummary(ctx context.Context) (*Summary, error) {
	s := &Summary{}
	today := time.Now().Format("2006-01-02")

	// hitung satu per satu biar query-nya jelas & aman
	counts := []struct {
		dst *int64
		sql string
	}{
		{&s.TotalKendaraan, "SELECT count(*) FROM kendaraan"},
		{&s.ArmadaAktif, "SELECT count(*) FROM kendaraan WHERE LOWER(status_kendaraan) IN ('aktif','berjalan','bertugas','tersedia')"},
		{&s.ArmadaSelesai, "SELECT count(*) FROM kendaraan WHERE LOWER(status_kendaraan) IN ('selesai','istirahat')"},
		{&s.ArmadaIdle, "SELECT count(*) FROM kendaraan WHERE LOWER(status_kendaraan) IN ('tersedia','idle','off')"},
		{&s.TotalDriver, "SELECT count(*) FROM driver"},
		{&s.DriverAktif, "SELECT count(*) FROM driver WHERE LOWER(status_driver) IN ('aktif','bertugas','on_duty')"},
		{&s.DriverLibur, "SELECT count(*) FROM driver WHERE LOWER(status_driver) IN ('libur','off','cuti')"},
		{&s.TotalRitase, "SELECT count(*) FROM ritase"},
		{&s.RitaseAktif, "SELECT count(*) FROM ritase WHERE LOWER(status) NOT IN ('selesai','completed','done','batal','cancelled')"},
		{&s.RitaseSelesai, "SELECT count(*) FROM ritase WHERE LOWER(status) IN ('selesai','completed','done')"},
		{&s.RitaseToday, "SELECT count(*) FROM ritase WHERE tanggal = $1"},
		{&s.TotalAWB, "SELECT COALESCE(sum(total_awb),0) FROM ritase"},
		{&s.TotalAWBToday, "SELECT COALESCE(sum(total_awb),0) FROM ritase WHERE tanggal = $1"},
		{&s.TotalKoli, "SELECT COALESCE(sum(total_koli),0) FROM ritase"},
		{&s.PaketTertinggal, "SELECT COALESCE(sum(paket_tertinggal),0) FROM ritase"},
		{&s.TotalSeller, "SELECT count(*) FROM seller"},
		{&s.SellerTerlayani, `SELECT count(DISTINCT rs.id_seller)
			FROM ritase_stop rs
			JOIN ritase r ON r.id_ritase = rs.id_ritase
			WHERE rs.jenis_stop = 'seller' AND LOWER(r.status) IN ('selesai','completed','done')`},
		{&s.TotalDropPoint, "SELECT count(*) FROM drop_point"},
		{&s.TotalKaryawan, "SELECT count(*) FROM karyawan"},
		{&s.TotalManpower, "SELECT COALESCE(sum(jumlah_manpower),0) FROM implant"},
		{&s.TotalAbsensi, "SELECT count(*) FROM absensi"},
		{&s.TotalImplant, "SELECT count(*) FROM implant"},
		{&s.TotalTracking, "SELECT count(*) FROM armada_tracking"},
	}

	for _, c := range counts {
		if c.sql == "SELECT count(*) FROM ritase WHERE tanggal = $1" ||
			c.sql == "SELECT COALESCE(sum(total_awb),0) FROM ritase WHERE tanggal = $1" {
			if err := r.db.QueryRow(ctx, c.sql, today).Scan(c.dst); err != nil {
				return nil, err
			}
			continue
		}
		if err := r.db.QueryRow(ctx, c.sql).Scan(c.dst); err != nil {
			return nil, err
		}
	}

	// driver terlambat: ritase berjalan yang umurnya > 6 jam sejak event pertama
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
