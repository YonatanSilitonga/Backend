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

func (r *Repository) ListDriver(ctx context.Context, offlineMin int) ([]Driver, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT d.id_driver, d.nama_driver, d.no_hp, d.no_sim, d.jenis_sim, d.status_driver,
		       lat.plat_nomor, lat.id_kendaraan,
		       COALESCE((lat.last_update > now() - make_interval(mins => %d)), false) AS tracking_fresh
		FROM driver d
		LEFT JOIN LATERAL (
			SELECT k.plat_nomor, t.id_kendaraan, t.last_update
			FROM armada_tracking t
			JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
			WHERE t.id_driver = d.id_driver
			ORDER BY t.last_update DESC
			LIMIT 1
		) lat ON true
		ORDER BY d.id_driver
	`, offlineMin))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Driver
	for rows.Next() {
		var d Driver
		if err := rows.Scan(&d.ID, &d.NamaDriver, &d.NoHP, &d.NoSIM, &d.JenisSIM, &d.StatusDriver, &d.PlatNomor, &d.IDKendaraan, &d.TrackingFresh); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

/* ---------- Ritase ---------- */

const ritaseSelect = `
	SELECT r.id_ritase, r.kode_ritase, r.tanggal::text,
	       r.id_driver, COALESCE(d.nama_driver,''),
	       r.id_kendaraan, COALESCE(k.plat_nomor,''),
	       COALESCE((SELECT string_agg(DISTINCT s.nama_seller, ', ')
	                 FROM ritase_stop rs
	                 JOIN seller s ON s.id_seller = rs.id_seller
	                 WHERE rs.id_ritase = r.id_ritase AND rs.jenis_stop = 'seller'), '') AS nama_seller,
	       r.id_drop_point, COALESCE(dp.nama_drop_point,''),
	       r.ritase_ke, r.total_awb, r.total_koli,
	       r.paket_tertinggal, r.alasan_tertinggal,
	       r.jam_berangkat::text, r.jam_tiba::text,
	       r.jam_mulai::text, r.jam_selesai::text,
	       r.status, r.created_at
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
		&r.NamaSeller,
		&r.IDDropPoint, &r.NamaDropPoint,
		&r.RitaseKe, &r.TotalAWB, &r.TotalKoli,
		&r.PaketTertinggal, &r.AlasanTertinggal,
		&r.JamBerangkat, &r.JamTiba,
		&r.JamMulai, &r.JamSelesai,
		&r.Status, &r.CreatedAt)
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

// GetRitase mengambil satu ritase + timeline event + rute (stops).
func (r *Repository) GetRitase(ctx context.Context, id int64) (*RitaseDetail, error) {
	rit, err := scanRitase(r.db.QueryRow(ctx, ritaseSelect+" WHERE r.id_ritase = $1", id))
	if err != nil {
		return nil, err
	}

	events, err := r.ListEvents(ctx, id)
	if err != nil {
		return nil, err
	}

	stops, err := r.ListStops(ctx, id)
	if err != nil {
		return nil, err
	}

	return &RitaseDetail{Ritase: *rit, Events: events, Stops: stops}, nil
}

// ListStops mengambil urutan rute sebuah ritase.
func (r *Repository) ListStops(ctx context.Context, idRitase int64) ([]RitaseStop, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id_stop, s.id_ritase, s.urutan, s.jenis_stop,
		       s.id_gudang, COALESCE(g.nama_gudang,''), COALESCE(g.tipe,''),
		       s.id_seller, s.id_drop_point,
		       COALESCE(seller.nama_seller,''), COALESCE(dp.nama_drop_point,''),
		       s.keterangan,
		       COALESCE(g.latitude, seller.latitude, dp.latitude) AS latitude,
		       COALESCE(g.longitude, seller.longitude, dp.longitude) AS longitude
		FROM ritase_stop s
		LEFT JOIN gudang g ON g.id_gudang = s.id_gudang
		LEFT JOIN seller seller ON seller.id_seller = s.id_seller
		LEFT JOIN drop_point dp ON dp.id_drop_point = s.id_drop_point
		WHERE s.id_ritase = $1
		ORDER BY s.urutan
	`, idRitase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []RitaseStop
	for rows.Next() {
		var st RitaseStop
		var namaGudang, tipeGudang, namaSeller, namaDP string
		if err := rows.Scan(&st.IDStop, &st.IDRitase, &st.Urutan, &st.JenisStop,
			&st.IDGudang, &namaGudang, &tipeGudang,
			&st.IDSeller, &st.IDDropPoint, &namaSeller, &namaDP, &st.Keterangan,
			&st.Latitude, &st.Longitude); err != nil {
			return nil, err
		}
		if namaGudang != "" {
			st.NamaGudang = &namaGudang
		}
		if tipeGudang != "" {
			st.TipeGudang = &tipeGudang
		}
		if namaSeller != "" {
			st.NamaSeller = &namaSeller
		}
		if namaDP != "" {
			st.NamaDropPoint = &namaDP
		}
		items = append(items, st)
	}
	return items, rows.Err()
}

// CreateRitase membuat penugasan baru (RIT) + rute (stops).
func (r *Repository) CreateRitase(ctx context.Context, req CreateRitaseRequest) (*Ritase, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var newID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_drop_point, ritase_ke, total_awb, total_koli, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'direncanakan')
		RETURNING id_ritase
	`, req.KodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, req.IDDropPoint,
		req.RitaseKe, req.TotalAWB, req.TotalKoli).Scan(&newID)
	if err != nil {
		return nil, err
	}

	for _, s := range req.Stops {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ritase_stop (id_ritase, urutan, jenis_stop, id_gudang, id_seller, id_drop_point, keterangan)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, newID, s.Urutan, s.JenisStop, s.IDGudang, s.IDSeller, s.IDDropPoint, s.Keterangan); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
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
		INSERT INTO ritase_event (id_ritase, status, catatan, latitude, longitude, nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id_event, id_ritase, status, catatan, latitude, longitude, nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value, created_at
	`, idRitase, req.Status, req.Catatan, req.Latitude, req.Longitude, req.NamaLokasi, req.DurasiDetik, req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue).Scan(
		&ev.ID, &ev.IDRitase, &ev.Status, &ev.Catatan, &ev.Latitude, &ev.Longitude, &ev.NamaLokasi, &ev.DurasiDetik, &ev.JumlahKoli, &ev.JumlahEcer, &ev.JumlahHighValue, &ev.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, _ = tx.Exec(ctx, `
		UPDATE armada_tracking 
		SET jumlah_koli = COALESCE($1, jumlah_koli), 
		    jumlah_ecer = COALESCE($2, jumlah_ecer),
		    jumlah_high_value = COALESCE($3, jumlah_high_value),
		    status = $4,
		    nama_lokasi = $5
		WHERE id_ritase = $6`, 
		req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue, req.Status, req.NamaLokasi, idRitase)

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
		SELECT id_event, id_ritase, status, catatan, latitude, longitude, nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, created_at
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
		if err := rows.Scan(&e.ID, &e.IDRitase, &e.Status, &e.Catatan, &e.Latitude, &e.Longitude, &e.NamaLokasi, &e.DurasiDetik, &e.JumlahKoli, &e.JumlahEcer, &e.CreatedAt); err != nil {
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

// CreateTracking menyimpan posisi GPS terbaru (1 baris live per kendaraan).
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
// offlineMin = ambang (menit) tanpa GPS terbaru → offline.
// sessionHours = ambang (jam) sejak login tanpa aktivitas → session dianggap mati.
// sessionRequired = kalau true, LIVE butuh GPS fresh DAN session login aktif
// (offline = GPS basi ATAU gak login) — anti data ghost. Default false.
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
