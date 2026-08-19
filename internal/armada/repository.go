package armada

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("data tidak ditemukan")
)

// Repository mengakses tabel-tabel armada.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

/* ---------- Kendaraan ---------- */

func (r *Repository) ListKendaraan(ctx context.Context) ([]Kendaraan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_kendaraan, plat_nomor, jenis_kendaraan,
		       kapasitas_kg, status_kendaraan
		FROM kendaraan
		ORDER BY id_kendaraan
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Kendaraan
	for rows.Next() {
		var k Kendaraan
		if err := rows.Scan(&k.ID, &k.PlatNomor, &k.JenisKendaraan,
			&k.KapasitasKg, &k.StatusKendaraan); err != nil {
			return nil, err
		}
		items = append(items, k)
	}
	return items, rows.Err()
}

/* ---------- Driver ---------- */

func (r *Repository) ListDriver(ctx context.Context) ([]Driver, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_driver, nama_driver, no_hp, no_sim, jenis_sim, status_driver
		FROM driver
		ORDER BY id_driver
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Driver
	for rows.Next() {
		var d Driver
		if err := rows.Scan(&d.ID, &d.NamaDriver, &d.NoHP, &d.NoSIM, &d.JenisSIM, &d.StatusDriver); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

/* ---------- Ritase ---------- */

const ritaseSelect = `
	SELECT r.id_ritase, r.kode_ritase, COALESCE(r.tanggal::text, ''),
	       r.id_driver, COALESCE(d.nama_driver,''),
	       r.id_kendaraan, COALESCE(k.plat_nomor,''),
	       0 AS id_seller, '' AS nama_seller,
	       COALESCE(r.id_drop_point, 0), COALESCE(dp.nama_drop_point,''),
	       COALESCE(r.ritase_ke, 1), COALESCE(r.total_awb, 0), COALESCE(r.total_koli, 0),
	       COALESCE(r.paket_tertinggal, 0), COALESCE(r.alasan_tertinggal, ''),
	       COALESCE(r.jam_berangkat::text, ''), COALESCE(r.jam_tiba::text, ''), COALESCE(r.status, 'direncanakan'), COALESCE(r.created_at, now())
	FROM ritase r
	LEFT JOIN driver d ON d.id_driver = r.id_driver
	LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
	LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
`

func scanRitase(row pgx.Row) (*Ritase, error) {
	var r Ritase
	err := row.Scan(&r.ID, &r.KodeRitase, &r.Tanggal,
		&r.IDDriver, &r.NamaDriver,
		&r.IDKendaraan, &r.PlatNomor,
		&r.IDSeller, &r.NamaSeller,
		&r.IDDropPoint, &r.NamaDropPoint,
		&r.RitaseKe, &r.TotalAWB, &r.TotalKoli,
		&r.PaketTertinggal, &r.AlasanTertinggal,
		&r.JamBerangkat, &r.JamTiba, &r.Status, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRitase mengambil semua penugasan, opsional filter driver/tanggal.
func (r *Repository) ListRitase(ctx context.Context, idDriver int64, tanggal string) ([]Ritase, error) {
	query := ritaseSelect + `
		WHERE 1=1
	`
	var args []interface{}
	if idDriver > 0 {
		args = append(args, idDriver)
		query += " AND r.id_driver = $" + fmt.Sprint(len(args))
	}
	if tanggal != "" {
		args = append(args, tanggal)
		query += " AND r.tanggal = $" + fmt.Sprint(len(args))
	}
	query += " ORDER BY r.tanggal DESC, r.ritase_ke, r.id_ritase DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Ritase
	for rows.Next() {
		rit, err := scanRitase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *rit)
	}
	return items, rows.Err()
}

// ListStops mengambil daftar titik perhentian (stops) rute penugasan ritase.
func (r *Repository) ListStops(ctx context.Context, idRitase int64) ([]RitaseStop, error) {
	rows, err := r.db.Query(ctx, `
		SELECT rs.id_stop, rs.id_ritase, rs.urutan, rs.jenis_stop,
		       rs.id_gudang, g.nama_gudang, g.tipe,
		       rs.id_seller, s.nama_seller,
		       rs.id_drop_point, dp.nama_drop_point,
		       rs.keterangan,
		       COALESCE(g.latitude, s.latitude, dp.latitude) as latitude,
		       COALESCE(g.longitude, s.longitude, dp.longitude) as longitude
		FROM ritase_stop rs
		LEFT JOIN gudang g ON rs.id_gudang = g.id_gudang
		LEFT JOIN seller s ON rs.id_seller = s.id_seller
		LEFT JOIN drop_point dp ON rs.id_drop_point = dp.id_drop_point
		WHERE rs.id_ritase = $1
		ORDER BY rs.urutan ASC
	`, idRitase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RitaseStop
	for rows.Next() {
		var s RitaseStop
		if err := rows.Scan(
			&s.IDStop, &s.IDRitase, &s.Urutan, &s.JenisStop,
			&s.IDGudang, &s.NamaGudang, &s.TipeGudang,
			&s.IDSeller, &s.NamaSeller,
			&s.IDDropPoint, &s.NamaDropPoint,
			&s.Keterangan, &s.Latitude, &s.Longitude,
		); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

// GetRitase mengambil satu ritase + rute stops + event timeline-nya.
func (r *Repository) GetRitase(ctx context.Context, id int64) (*RitaseDetail, error) {
	rit, err := scanRitase(r.db.QueryRow(ctx, ritaseSelect+" WHERE r.id_ritase = $1", id))
	if err != nil {
		return nil, err
	}

	stops, err := r.ListStops(ctx, id)
	if err != nil {
		stops = []RitaseStop{}
	}

	events, err := r.ListEvents(ctx, id)
	if err != nil {
		events = []RitaseEvent{}
	}

	return &RitaseDetail{Ritase: *rit, Stops: stops, Events: events}, nil
}

// CreateRitase membuat penugasan baru.
func (r *Repository) CreateRitase(ctx context.Context, req CreateRitaseRequest) (*Ritase, error) {
	var newID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'direncanakan')
		RETURNING id_ritase
	`, req.KodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, req.IDSeller, req.IDDropPoint,
		req.RitaseKe, req.TotalAWB, req.TotalKoli).Scan(&newID)
	if err != nil {
		return nil, err
	}
	detail, err := r.GetRitase(ctx, newID)
	if err != nil {
		return nil, err
	}
	return &detail.Ritase, nil
}

// AddEvent mencatat status baru di timeline & update status ritase.
func (r *Repository) AddEvent(ctx context.Context, idRitase int64, req UpdateStatusRequest) (*RitaseEvent, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var ev RitaseEvent
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase_event (id_ritase, status, catatan, latitude, longitude, durasi_detik)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id_event, id_ritase, status, catatan, latitude, longitude, durasi_detik, created_at
	`, idRitase, req.Status, req.Catatan, req.Latitude, req.Longitude, req.DurasiDetik).Scan(
		&ev.ID, &ev.IDRitase, &ev.Status, &ev.Catatan, &ev.Latitude, &ev.Longitude, &ev.DurasiDetik, &ev.CreatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, "UPDATE ritase SET status = $1 WHERE id_ritase = $2", req.Status, idRitase); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ev, nil
}

// UpdateMuatan memperbarui data muatan ritase.
func (r *Repository) UpdateMuatan(ctx context.Context, idRitase int64, req UpdateMuatanRequest) (*Ritase, error) {
	if _, err := r.db.Exec(ctx, `
		UPDATE ritase
		SET total_awb = COALESCE($1, total_awb),
		    total_koli = COALESCE($2, total_koli),
		    paket_tertinggal = COALESCE($3, paket_tertinggal),
		    alasan_tertinggal = COALESCE($4, alasan_tertinggal)
		WHERE id_ritase = $5
	`, req.TotalAWB, req.TotalKoli, req.PaketTertinggal, req.AlasanTertinggal, idRitase); err != nil {
		return nil, err
	}
	detail, err := r.GetRitase(ctx, idRitase)
	if err != nil {
		return nil, err
	}
	return &detail.Ritase, nil
}

// ListEvents mengambil timeline status sebuah ritase.
func (r *Repository) ListEvents(ctx context.Context, idRitase int64) ([]RitaseEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_event, id_ritase, status, catatan, latitude, longitude, created_at
		FROM ritase_event
		WHERE id_ritase = $1
		ORDER BY created_at, id_event
	`, idRitase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RitaseEvent
	for rows.Next() {
		var e RitaseEvent
		if err := rows.Scan(&e.ID, &e.IDRitase, &e.Status, &e.Catatan, &e.Latitude, &e.Longitude, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

/* ---------- Tracking ---------- */

func (r *Repository) ListTracking(ctx context.Context, limit int) ([]Tracking, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id_tracking, id_ritase, id_kendaraan, id_driver,
		       latitude, longitude, kecepatan, arah, status, last_update
		FROM armada_tracking
		ORDER BY last_update DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Tracking
	for rows.Next() {
		var t Tracking
		if err := rows.Scan(&t.ID, &t.IDRitase, &t.IDKendaraan, &t.IDDriver,
			&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status, &t.LastUpdate); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// CreateTracking menyimpan posisi GPS terbaru.
func (r *Repository) CreateTracking(ctx context.Context, req CreateTrackingRequest) (*Tracking, error) {
	var t Tracking
	err := r.db.QueryRow(ctx, `
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, last_update)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
		ON CONFLICT (id_kendaraan)
		DO UPDATE SET id_driver   = EXCLUDED.id_driver,
		              latitude    = EXCLUDED.latitude,
		              longitude   = EXCLUDED.longitude,
		              kecepatan   = EXCLUDED.kecepatan,
		              arah        = EXCLUDED.arah,
		              status      = EXCLUDED.status,
		              last_update = now()
		RETURNING id_tracking, id_ritase, id_kendaraan, id_driver,
		          latitude, longitude, kecepatan, arah, status, last_update
	`, req.IDRitase, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude,
		req.Kecepatan, req.Arah, req.Status).Scan(
		&t.ID, &t.IDRitase, &t.IDKendaraan, &t.IDDriver,
		&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status, &t.LastUpdate)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListLatestTracking mengambil 1 posisi terbaru per kendaraan (data live untuk peta).
func (r *Repository) ListLatestTracking(ctx context.Context, offlineMin int, sessionHours int, sessionRequired bool) ([]TrackingLive, error) {
	offlineExpr := fmt.Sprintf("(t.last_update < now() - make_interval(mins => %d))", offlineMin)
	if sessionRequired {
		offlineExpr = fmt.Sprintf(
			"((t.last_update < now() - make_interval(mins => %d)) OR (u.last_login IS NULL OR u.last_login < now() - make_interval(hours => %d)))",
			offlineMin, sessionHours,
		)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT t.id_tracking, t.id_ritase, t.id_kendaraan, COALESCE(k.plat_nomor,''),
		       t.id_driver, COALESCE(d.nama_driver,''),
		       t.latitude, t.longitude, t.kecepatan, t.arah, t.status, t.nama_lokasi, t.last_update,
		       %s AS offline,
		       (u.last_login IS NOT NULL AND u.last_login > now() - make_interval(hours => %d)) AS session_online,
		       u.last_login, u.last_open
		FROM armada_tracking t
		LEFT JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
		LEFT JOIN driver d ON d.id_driver = t.id_driver
		LEFT JOIN users u ON u.id_driver = d.id_driver
		ORDER BY t.last_update DESC
	`, offlineExpr, sessionHours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TrackingLive
	for rows.Next() {
		var t TrackingLive
		if err := rows.Scan(&t.ID, &t.IDRitase, &t.IDKendaraan, &t.PlatNomor,
			&t.IDDriver, &t.NamaDriver,
			&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status, &t.NamaLokasi, &t.LastUpdate,
			&t.Offline, &t.SessionOnline, &t.LastLogin, &t.LastOpen); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// ListSellerLocations mengambil seller yang punya koordinat (untuk peta).
func (r *Repository) ListSellerLocations(ctx context.Context) ([]SellerLocation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_seller, COALESCE(kode_seller,''), COALESCE(nama_seller,''), COALESCE(alamat,''),
		       COALESCE(kota,''), COALESCE(pic,''), COALESCE(no_hp,''),
		       latitude, longitude, jarak_tempuh_km, jarak_dc_km
		FROM seller
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		ORDER BY id_seller ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SellerLocation
	for rows.Next() {
		var s SellerLocation
		if err := rows.Scan(&s.IDSeller, &s.KodeSeller, &s.NamaSeller, &s.Alamat, &s.Kota, &s.PIC, &s.NoHP, &s.Latitude, &s.Longitude, &s.JarakTempuhKm, &s.JarakDcKm); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

// ListGudangLocations mengambil posisi gudang yang punya koordinat (peta).
func (r *Repository) ListGudangLocations(ctx context.Context) ([]GudangPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_gudang, COALESCE(nama_gudang,''), COALESCE(tipe,''), latitude, longitude
		FROM gudang
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		ORDER BY id_gudang ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GudangPoint
	for rows.Next() {
		var g GudangPoint
		if err := rows.Scan(&g.IDGudang, &g.NamaGudang, &g.Tipe, &g.Latitude, &g.Longitude); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

// ListDropPoints mengambil posisi drop_point yang punya koordinat (peta).
func (r *Repository) ListDropPoints(ctx context.Context) ([]DropPointPoi, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_drop_point, COALESCE(kode_dp,''), COALESCE(nama_drop_point,''), latitude, longitude,
		       jarak_tempuh_km, jarak_dc_km
		FROM drop_point
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		ORDER BY id_drop_point ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DropPointPoi
	for rows.Next() {
		var p DropPointPoi
		if err := rows.Scan(&p.IDDropPoint, &p.KodeDP, &p.NamaDP, &p.Latitude, &p.Longitude, &p.JarakTempuhKm, &p.JarakDcKm); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *Repository) ListTrackingHistory(ctx context.Context, idKendaraan int64, tanggal string) ([]TrackingCheckpoint, error) {
	query := `
		SELECT e.id_event, e.id_ritase, COALESCE(r.kode_ritase,''),
		       e.status, e.catatan, e.latitude, e.longitude, e.durasi_detik, e.created_at
		FROM ritase_event e
		JOIN ritase r ON r.id_ritase = e.id_ritase
		WHERE r.id_kendaraan = $1
	`
	var args []interface{} = []interface{}{idKendaraan}
	if tanggal != "" {
		args = append(args, tanggal)
		query += " AND e.created_at::date = $" + fmt.Sprint(len(args))
	}
	query += " ORDER BY e.created_at DESC, e.id_event DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TrackingCheckpoint
	for rows.Next() {
		var c TrackingCheckpoint
		if err := rows.Scan(&c.IDEvent, &c.IDRitase, &c.KodeRitase,
			&c.Status, &c.Catatan, &c.Latitude, &c.Longitude, &c.DurasiDetik, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
