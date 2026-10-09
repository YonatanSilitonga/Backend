package armada

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		       kapasitas_koli, kapasitas_kg, status_kendaraan
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
			&k.KapasitasKoli, &k.KapasitasKg, &k.StatusKendaraan); err != nil {
			return nil, err
		}
		items = append(items, k)
	}
	return items, rows.Err()
}

/* ---------- Driver ---------- */

func (r *Repository) ListDriver(ctx context.Context) ([]Driver, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.id_driver, d.nama_driver, d.no_hp, d.no_sim, d.jenis_sim, d.status_driver,
		       k.plat_nomor, t.id_kendaraan,
		       CASE WHEN t.last_update IS NOT NULL AND t.last_update > now() - interval '5 minutes' THEN true ELSE false END AS tracking_fresh
		FROM driver d
		LEFT JOIN LATERAL (
			SELECT id_kendaraan, last_update
			FROM armada_tracking
			WHERE id_driver = d.id_driver
			ORDER BY last_update DESC
			LIMIT 1
		) t ON true
		LEFT JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
		ORDER BY d.id_driver
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Driver
	for rows.Next() {
		var d Driver
		if err := rows.Scan(&d.ID, &d.NamaDriver, &d.NoHP, &d.NoSIM, &d.JenisSIM, &d.StatusDriver,
			&d.PlatNomor, &d.IDKendaraan, &d.TrackingFresh); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

/* ---------- Ritase ---------- */

const ritaseSelect = `
	WITH muatan AS (
		SELECT id_ritase,
		       sum(jumlah_koli) AS koli,
		       sum(jumlah_high_value) AS hv,
		       sum(jumlah_ecer) AS ecer
		FROM ritase_event
		WHERE status IN ('Bongkar Muat Barang', 'Muat Barang')
		GROUP BY id_ritase
	)
	SELECT r.id_ritase, r.kode_ritase, COALESCE(r.tanggal::text, ''),
	       r.id_driver, COALESCE(d.nama_driver,''),
	       r.id_kendaraan, COALESCE(k.plat_nomor,''),
	       0 AS id_seller, '' AS nama_seller,
	       COALESCE(r.id_drop_point, 0), COALESCE(dp.nama_drop_point,''),
	       COALESCE(r.ritase_ke, 1), COALESCE(r.jenis_ritase, 'outgoing'),
	       COALESCE(r.total_awb, 0), COALESCE(m.koli, 0),
	       COALESCE(m.hv, 0), COALESCE(m.ecer, 0),
	       COALESCE(r.paket_tertinggal, 0), COALESCE(r.alasan_tertinggal, ''),
	       COALESCE(TO_CHAR((SELECT MIN(e.created_at) + interval '7 hours' FROM ritase_event e WHERE e.id_ritase = r.id_ritase), 'HH24:MI'), ''),
	       COALESCE(TO_CHAR((SELECT MAX(e.created_at) + interval '7 hours' FROM ritase_event e WHERE e.id_ritase = r.id_ritase AND e.status = 'Selesai'), 'HH24:MI'), ''),
	       TO_CHAR(r.jam_mulai, 'HH24:MI'), TO_CHAR(r.jam_selesai, 'HH24:MI'),
		COALESCE(r.status, 'direncanakan'),
		COALESCE(r.created_at::text, ''),
		COALESCE(r.updated_at::text, ''),
		COALESCE(u1.username, '') AS created_by_name,
		COALESCE(u2.username, '') AS updated_by_name
	FROM ritase r
	LEFT JOIN driver d ON d.id_driver = r.id_driver
	LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
	LEFT JOIN drop_point dp ON dp.id_drop_point = r.id_drop_point
	LEFT JOIN muatan m ON m.id_ritase = r.id_ritase
	LEFT JOIN users u1 ON u1.id_user = r.created_by
	LEFT JOIN users u2 ON u2.id_user = r.updated_by
`

func scanRitase(row pgx.Row) (*Ritase, error) {
	var r Ritase
	err := row.Scan(&r.ID, &r.KodeRitase, &r.Tanggal,
		&r.IDDriver, &r.NamaDriver,
		&r.IDKendaraan, &r.PlatNomor,
		&r.IDSeller, &r.NamaSeller,
		&r.IDDropPoint, &r.NamaDropPoint,
		&r.RitaseKe, &r.JenisRitase, &r.TotalAWB, &r.TotalKoli,
		&r.TotalHighValue, &r.TotalEceran,
		&r.PaketTertinggal, &r.AlasanTertinggal,
		&r.JamBerangkat, &r.JamTiba,
		&r.JamMulai, &r.JamSelesai,
		&r.Status, &r.CreatedAt, &r.UpdatedAt, &r.CreatedByName, &r.UpdatedByName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRitase mengambil semua penugasan, opsional filter driver/tanggal.
func (r *Repository) ListRitase(ctx context.Context, idDriver int64, startDate, endDate string) ([]Ritase, error) {
	query := ritaseSelect + `
		WHERE 1=1
	`
	var args []interface{}
	if idDriver > 0 {
		args = append(args, idDriver)
		query += " AND r.id_driver = $" + fmt.Sprint(len(args))
	}
	if startDate != "" && endDate != "" {
		args = append(args, startDate)
		query += " AND r.tanggal >= $" + fmt.Sprint(len(args))
		args = append(args, endDate)
		query += " AND r.tanggal <= $" + fmt.Sprint(len(args))
	} else if startDate != "" {
		args = append(args, startDate)
		query += " AND r.tanggal = $" + fmt.Sprint(len(args))
	} else {
		query += " AND r.tanggal >= current_date - interval '30 days'"
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
		SELECT rs.id_stop, rs.id_ritase, rs.urutan,
		       CASE WHEN rs.id_implant IS NOT NULL THEN 'implant' ELSE rs.jenis_stop END AS jenis_stop,
		       rs.id_gudang, g.nama_gudang, g.tipe,
		       rs.id_seller, s.nama_seller,
		       rs.id_implant, i.nama_implant,
		       rs.id_drop_point, dp.nama_drop_point,
		       rs.keterangan,
		COALESCE(g.latitude, s.latitude, i.latitude, dp.latitude) as latitude,
		       COALESCE(g.longitude, s.longitude, i.longitude, dp.longitude) as longitude,
		       re.jumlah_koli,
		       re.jumlah_ecer,
		       re.jumlah_high_value,
		       re.durasi_detik,
		       COALESCE(re.foto_manifest_url, rs.foto_manifest_url) AS foto_manifest_url
		FROM ritase_stop rs
		LEFT JOIN gudang g ON rs.id_gudang = g.id_gudang
		LEFT JOIN seller s ON rs.id_seller = s.id_seller
		LEFT JOIN implant i ON rs.id_implant = i.id_implant
		LEFT JOIN drop_point dp ON rs.id_drop_point = dp.id_drop_point
		LEFT JOIN LATERAL (
			SELECT 
				COALESCE(MAX(ev.jumlah_koli), 0) AS jumlah_koli,
				COALESCE(MAX(ev.jumlah_ecer), 0) AS jumlah_ecer,
				COALESCE(MAX(ev.jumlah_high_value), 0) AS jumlah_high_value,
				COALESCE((
					SELECT SUM(ev2.durasi_detik)
					FROM ritase_event ev2 
					WHERE ev2.id_ritase = rs.id_ritase 
					AND ev2.status IN ('Tiba', 'Bongkar Muat Barang', 'Muat Barang', 'Bongkar Barang')
					AND (
						ev2.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
						OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev2.nama_lokasi) IN LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
						OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) IN LOWER(ev2.nama_lokasi)) > 0)
					)
				), 0) AS durasi_detik,
				(SELECT ev2.foto_manifest_url FROM ritase_event ev2
				 WHERE ev2.id_ritase = rs.id_ritase
				   AND ev2.foto_manifest_url IS NOT NULL AND ev2.foto_manifest_url != ''
				   AND (
				     ev2.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
				     OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev2.nama_lokasi) in LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
				     OR (ev2.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER(ev2.nama_lokasi)) > 0)
				   )
				 LIMIT 1
				) AS foto_manifest_url
			FROM ritase_event ev
			WHERE ev.id_ritase = rs.id_ritase
			  AND (
			    ev.nama_lokasi = COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang)
			    OR (ev.nama_lokasi IS NOT NULL AND POSITION(LOWER(ev.nama_lokasi) in LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0)
			    OR (ev.nama_lokasi IS NOT NULL AND POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER(ev.nama_lokasi)) > 0)
			  )
		) re ON true
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
			&s.IDImplant, &s.NamaImplant,
			&s.IDDropPoint, &s.NamaDropPoint,
			&s.Keterangan, &s.Latitude, &s.Longitude,
			&s.JumlahKoli, &s.JumlahEcer, &s.JumlahHighValue,
			&s.DurasiDetik, &s.FotoManifestURL,
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
func (r *Repository) CreateRitase(ctx context.Context, req CreateRitaseRequest, createdBy int64) (*Ritase, error) {
	var newID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO ritase (kode_ritase, tanggal, id_driver, id_kendaraan, id_seller, id_drop_point, ritase_ke, total_awb, total_koli, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'direncanakan',$10)
		RETURNING id_ritase
	`, req.KodeRitase, req.Tanggal, req.IDDriver, req.IDKendaraan, req.IDSeller, req.IDDropPoint,
		req.RitaseKe, req.TotalAWB, req.TotalKoli, createdBy).Scan(&newID)
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
		INSERT INTO ritase_event (id_ritase, status, catatan, latitude, longitude, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id_event, id_ritase, status, catatan, latitude, longitude, durasi_detik, created_at
	`, idRitase, req.Status, req.Catatan, req.Latitude, req.Longitude, req.DurasiDetik,
		req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue).Scan(
		&ev.ID, &ev.IDRitase, &ev.Status, &ev.Catatan, &ev.Latitude, &ev.Longitude, &ev.DurasiDetik, &ev.CreatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, "UPDATE ritase SET status = $1, updated_at = NOW() WHERE id_ritase = $2", req.Status, idRitase); err != nil {
		return nil, err
	}

	// Isi jam_berangkat otomatis saat status pertama kali berubah dari direncanakan
	_, _ = tx.Exec(ctx, `
		UPDATE ritase SET jam_berangkat = NOW()
		WHERE id_ritase = $1 AND jam_berangkat IS NULL AND status != 'direncanakan'
	`, idRitase)

	// Isi jam_tiba otomatis saat driver tiba atau selesai
	statusLower := strings.ToLower(req.Status)
	if strings.Contains(statusLower, "tiba") || strings.Contains(statusLower, "selesai") || strings.Contains(statusLower, "sampai") {
		_, _ = tx.Exec(ctx, `
			UPDATE ritase SET jam_tiba = NOW()
			WHERE id_ritase = $1 AND jam_tiba IS NULL
		`, idRitase)
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
		    alasan_tertinggal = COALESCE($4, alasan_tertinggal),
		    updated_at = NOW()
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
		SELECT id_event, id_ritase, status, catatan, latitude, longitude,
		       nama_lokasi, durasi_detik, jumlah_koli, jumlah_ecer, jumlah_high_value, created_at
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
		if err := rows.Scan(
			&e.ID, &e.IDRitase, &e.Status, &e.Catatan, &e.Latitude, &e.Longitude,
			&e.NamaLokasi, &e.DurasiDetik, &e.JumlahKoli, &e.JumlahEcer, &e.JumlahHighValue, &e.CreatedAt,
		); err != nil {
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
		INSERT INTO armada_tracking (id_ritase, id_kendaraan, id_driver, latitude, longitude, kecepatan, arah, status, jumlah_koli, jumlah_ecer, jumlah_high_value, last_update)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
		ON CONFLICT (id_kendaraan)
		DO UPDATE SET id_driver   = EXCLUDED.id_driver,
		              latitude    = EXCLUDED.latitude,
		              longitude   = EXCLUDED.longitude,
		              kecepatan   = EXCLUDED.kecepatan,
		              arah        = EXCLUDED.arah,
		              status      = EXCLUDED.status,
		              jumlah_koli        = CASE WHEN EXCLUDED.jumlah_koli > 0 THEN EXCLUDED.jumlah_koli ELSE armada_tracking.jumlah_koli END,
		              jumlah_ecer        = CASE WHEN EXCLUDED.jumlah_ecer > 0 THEN EXCLUDED.jumlah_ecer ELSE armada_tracking.jumlah_ecer END,
		              jumlah_high_value  = CASE WHEN EXCLUDED.jumlah_high_value > 0 THEN EXCLUDED.jumlah_high_value ELSE armada_tracking.jumlah_high_value END,
		              last_update = now()
		RETURNING id_tracking, id_ritase, id_kendaraan, id_driver,
		          latitude, longitude, kecepatan, arah, status, last_update
	`, req.IDRitase, req.IDKendaraan, req.IDDriver, req.Latitude, req.Longitude,
		req.Kecepatan, req.Arah, req.Status,
		req.JumlahKoli, req.JumlahEcer, req.JumlahHighValue).Scan(
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
		SELECT t.id_tracking, t.id_ritase, r.status,
		       TO_CHAR(r.tanggal, 'YYYY-MM-DD'),
		       TO_CHAR(r.jam_mulai, 'HH24:MI:SS'), TO_CHAR(r.jam_selesai, 'HH24:MI:SS'),
		       t.id_kendaraan, COALESCE(k.plat_nomor,''),
		       t.id_driver, COALESCE(d.nama_driver,''),
		       t.latitude, t.longitude, t.kecepatan, t.arah, t.status,
		       COALESCE(NULLIF(t.nama_lokasi, ''), re.nama_lokasi, ''),
		       COALESCE(NULLIF(pl.pickup_koli, 0), t.jumlah_koli, re.jumlah_koli, 0),
		       COALESCE(NULLIF(pl.pickup_ecer, 0), t.jumlah_ecer, re.jumlah_ecer, 0),
		       COALESCE(NULLIF(pl.pickup_hv, 0), t.jumlah_high_value, re.jumlah_high_value, 0),
		       t.last_update,
		       %s AS offline,
		       (u.last_login IS NOT NULL AND u.last_login > now() - make_interval(hours => %d)) AS session_online,
		       u.last_login, u.last_open,
		       COALESCE(u.role, 'driver') AS role_driver,
		       COALESCE(NULLIF(pl.pickup_awb, 0), r.total_awb, t.jumlah_koli, re.jumlah_koli, 0) AS total_awb
		FROM armada_tracking t
		LEFT JOIN kendaraan k ON k.id_kendaraan = t.id_kendaraan
		LEFT JOIN ritase r ON r.id_ritase = t.id_ritase
		LEFT JOIN driver d ON d.id_driver = t.id_driver
		LEFT JOIN LATERAL (
			SELECT * FROM users WHERE id_driver = t.id_driver ORDER BY id_user DESC LIMIT 1
		) u ON true
		LEFT JOIN LATERAL (
			SELECT ev.nama_lokasi, ev.jumlah_koli, ev.jumlah_ecer, ev.jumlah_high_value
			FROM ritase_event ev
			WHERE ev.id_ritase = t.id_ritase AND (ev.jumlah_koli > 0 OR ev.jumlah_ecer > 0 OR ev.jumlah_high_value > 0)
			  AND (ev.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date = current_date
			ORDER BY ev.created_at DESC, ev.id_event DESC
			LIMIT 1
		) re ON true
		LEFT JOIN LATERAL (
			WITH last_selesai AS (
				SELECT COALESCE(MAX(id_log), 0) AS max_selesai_id
				FROM driver_pickup_log
				WHERE id_user = u.id_user 
				  AND status = 'selesai'
				  AND (tanggal = CURRENT_DATE OR tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
			),
			active_logs AS (
				SELECT dpl.*
				FROM driver_pickup_log dpl, last_selesai ls
				WHERE dpl.id_user = u.id_user
				  AND (dpl.tanggal = CURRENT_DATE OR dpl.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				  AND dpl.id_log > ls.max_selesai_id
			)
			SELECT 
				COALESCE(SUM(al.jumlah_barang), 0)::int AS pickup_awb,
				COALESCE(SUM(al.koli), 0)::int AS pickup_koli,
				COALESCE(SUM(al.ecer), 0)::int AS pickup_ecer,
				COALESCE(SUM(al.high_value), 0)::int AS pickup_hv
			FROM active_logs al
		) pl ON true
		ORDER BY t.last_update DESC
	`, offlineExpr, sessionHours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TrackingLive
	for rows.Next() {
		var t TrackingLive
		if err := rows.Scan(&t.ID, &t.IDRitase, &t.StatusRitase, &t.TanggalRitase, &t.JamMulai, &t.JamSelesai, &t.IDKendaraan, &t.PlatNomor,
			&t.IDDriver, &t.NamaDriver,
			&t.Latitude, &t.Longitude, &t.Kecepatan, &t.Arah, &t.Status, &t.NamaLokasi,
			&t.JumlahKoli, &t.JumlahEcer, &t.JumlahHighValue,
			&t.LastUpdate,
			&t.Offline, &t.SessionOnline, &t.LastLogin, &t.LastOpen,
			&t.RoleDriver, &t.TotalAWB); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// ListSellerLocations mengambil seller yang punya koordinat (untuk peta), termasuk status log barang hari ini (dari input kapten & implan log).
func (r *Repository) ListSellerLocations(ctx context.Context) ([]SellerLocation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id_seller, COALESCE(s.kode_seller,''), COALESCE(s.nama_seller,''), COALESCE(s.alamat,''),
		       COALESCE(s.kota,''), COALESCE(s.pic,''), COALESCE(s.no_hp,''),
		       s.latitude, s.longitude, s.jarak_tempuh_km, s.jarak_dc_km,
		       mu.total_koli, mu.total_ecer, mu.total_hv,
		       CASE 
		           WHEN kap.count_input > 0 AND (kap.kap_awb > 0 OR kap.kap_koli > 0 OR kap.kap_ecer > 0 OR kap.kap_hv > 0) THEN kap.kap_awb
		           ELSE COALESCE(ibl.jumlah_barang, 0)
		       END AS jumlah_barang,
		       CASE 
		           WHEN kap.count_input > 0 AND (kap.kap_awb > 0 OR kap.kap_koli > 0 OR kap.kap_ecer > 0 OR kap.kap_hv > 0) THEN kap.kap_koli
		           ELSE COALESCE(ibl.koli, 0)
		       END AS koli,
		       CASE 
		           WHEN kap.count_input > 0 AND (kap.kap_awb > 0 OR kap.kap_koli > 0 OR kap.kap_ecer > 0 OR kap.kap_hv > 0) THEN kap.kap_ecer
		           ELSE COALESCE(ibl.ecer, 0)
		       END AS ecer,
		       CASE 
		           WHEN kap.count_input > 0 AND (kap.kap_awb > 0 OR kap.kap_koli > 0 OR kap.kap_ecer > 0 OR kap.kap_hv > 0) THEN kap.kap_hv
		           ELSE COALESCE(ibl.high_value, 0)
		       END AS high_value,
		       CASE 
		           WHEN kap.count_input > 0 AND (kap.kap_awb > 0 OR kap.kap_koli > 0) THEN 
		               CASE WHEN kap.taken_awb >= kap.kap_awb AND kap.taken_koli >= kap.kap_koli AND kap.taken_awb > 0 THEN 'sudah_diambil' ELSE 'menunggu' END
		           WHEN ibl.status IS NOT NULL THEN ibl.status
		           ELSE 'menunggu'
		       END AS status_pickup,
		       CASE 
		           WHEN kap.count_input > 0 AND kap.kap_catatan != '' THEN kap.kap_catatan
		           ELSE COALESCE(ibl.catatan, '')
		       END AS catatan_pickup
		FROM seller s
		LEFT JOIN (
			SELECT rs.id_seller,
			       SUM(re.jumlah_koli) AS total_koli,
			       SUM(re.jumlah_ecer) AS total_ecer,
			       SUM(re.jumlah_high_value) AS total_hv
			FROM ritase_event re
			JOIN ritase r ON r.id_ritase = re.id_ritase
			JOIN ritase_stop rs ON rs.id_ritase = re.id_ritase AND rs.id_seller IS NOT NULL
			WHERE (r.tanggal = CURRENT_DATE OR r.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE) AND r.status != 'selesai'
			GROUP BY rs.id_seller
		) mu ON mu.id_seller = s.id_seller
		LEFT JOIN LATERAL (
			SELECT 
				COALESCE(SUM(ik.jumlah_awb), 0)::int AS kap_awb,
				COALESCE(SUM(ik.koli_jkt + ik.koli_seg + ik.koli_btn), 0)::int AS kap_koli,
				COALESCE(SUM(ik.ecer_jkt + ik.ecer_seg + ik.ecer_btn), 0)::int AS kap_ecer,
				COALESCE(SUM(ik.koli_hv_jkt + ik.koli_hv_seg + ik.koli_hv_btn + ik.ecer_hv_jkt + ik.ecer_hv_seg + ik.ecer_hv_btn), 0)::int AS kap_hv,
				COALESCE((
					SELECT ik2.catatan FROM input_kapten ik2 
					WHERE ik2.id_seller = s.id_seller 
					  AND (ik2.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
					  AND ik2.catatan IS NOT NULL AND ik2.catatan != ''
					ORDER BY ik2.id DESC LIMIT 1
				), '') AS kap_catatan,
				(COALESCE((
					SELECT SUM(kp.jumlah_awb) FROM konfirmasi_penjemputan kp
					WHERE kp.id_seller = s.id_seller
					  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
				), 0) +
				COALESCE((
					SELECT SUM(dpl.jumlah_barang) FROM driver_pickup_log dpl
					WHERE (LOWER(dpl.asal_seller) = LOWER(s.nama_seller) OR LOWER(dpl.asal_seller) LIKE '%' || LOWER(s.nama_seller) || '%')
					  AND (dpl.tanggal = CURRENT_DATE OR dpl.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				), 0))::int AS taken_awb,
				(COALESCE((
					SELECT SUM(kp.koli_jkt + kp.koli_seg + kp.koli_btn) FROM konfirmasi_penjemputan kp
					WHERE kp.id_seller = s.id_seller
					  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
				), 0) +
				COALESCE((
					SELECT SUM(dpl.koli) FROM driver_pickup_log dpl
					WHERE (LOWER(dpl.asal_seller) = LOWER(s.nama_seller) OR LOWER(dpl.asal_seller) LIKE '%' || LOWER(s.nama_seller) || '%')
					  AND (dpl.tanggal = CURRENT_DATE OR dpl.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				), 0))::int AS taken_koli,
				COUNT(ik.id) AS count_input
			FROM input_kapten ik
			WHERE ik.id_seller = s.id_seller
			  AND (ik.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		) kap ON TRUE
		LEFT JOIN LATERAL (
			SELECT log.jumlah_barang, log.koli, log.ecer, log.high_value, log.status, log.catatan
			FROM implan_barang_log log
			WHERE log.id_seller = s.id_seller
			  AND (log.tanggal = CURRENT_DATE OR log.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
			ORDER BY log.id_log DESC
			LIMIT 1
		) ibl ON TRUE
		WHERE s.latitude IS NOT NULL AND s.longitude IS NOT NULL AND s.status = 'aktif'
		ORDER BY s.id_seller ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SellerLocation
	for rows.Next() {
		var s SellerLocation
		if err := rows.Scan(&s.IDSeller, &s.KodeSeller, &s.NamaSeller, &s.Alamat,
			&s.Kota, &s.PIC, &s.NoHP, &s.Latitude, &s.Longitude,
			&s.JarakTempuhKm, &s.JarakDcKm,
			&s.TotalKoli, &s.TotalEcer, &s.TotalHighValue,
			&s.JumlahBarang, &s.Koli, &s.Ecer, &s.HighValue, &s.StatusPickup, &s.CatatanPickup); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

// ListImplantLocations mengambil implant aktif yang punya koordinat (untuk peta),
// termasuk status log barang hari ini. PIC = nama kapten (utama dulu).
func (r *Repository) ListImplantLocations(ctx context.Context) ([]ImplantLocation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id_implant, COALESCE(i.kode_implant,''), COALESCE(i.nama_implant,''), COALESCE(i.alamat,''),
		       COALESCE(i.kota,''), COALESCE(kapten_agg.kapten, ''), COALESCE(i.no_hp,''),
		       i.latitude, i.longitude, i.jarak_tempuh_km, i.jarak_dc_km,
		       mu.total_koli, mu.total_ecer, mu.total_hv,
		       ibl.jumlah_barang, ibl.koli, ibl.ecer, ibl.high_value, ibl.status, ibl.catatan
		FROM implant i
		LEFT JOIN (
			SELECT kim.id_implant, STRING_AGG(k.nama, ', ' ORDER BY CASE WHEN kim.peran = 'utama' THEN 0 ELSE 1 END, k.nama) AS kapten
			FROM kapten_implant_map kim
			JOIN kapten k ON k.id_user = kim.id_user
			GROUP BY kim.id_implant
		) kapten_agg ON kapten_agg.id_implant = i.id_implant
		LEFT JOIN (
			SELECT rs.id_implant,
			       SUM(re.jumlah_koli) AS total_koli,
			       SUM(re.jumlah_ecer) AS total_ecer,
			       SUM(re.jumlah_high_value) AS total_hv
			FROM ritase_event re
			JOIN ritase r ON r.id_ritase = re.id_ritase
			JOIN ritase_stop rs ON rs.id_ritase = re.id_ritase AND rs.id_implant IS NOT NULL
			WHERE (r.tanggal = CURRENT_DATE OR r.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE) AND r.status != 'selesai'
			GROUP BY rs.id_implant
		) mu ON mu.id_implant = i.id_implant
		LEFT JOIN LATERAL (
			SELECT log.jumlah_barang, log.koli, log.ecer, log.high_value, log.status, log.catatan
			FROM implan_barang_log log
			WHERE log.id_implant = i.id_implant
			  AND (log.tanggal = CURRENT_DATE OR log.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
			ORDER BY log.id_log DESC
			LIMIT 1
		) ibl ON TRUE
		WHERE i.latitude IS NOT NULL AND i.longitude IS NOT NULL AND i.status = 'aktif'
		ORDER BY i.id_implant ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ImplantLocation
	for rows.Next() {
		var im ImplantLocation
		if err := rows.Scan(&im.IDImplant, &im.KodeImplant, &im.NamaImplant, &im.Alamat,
			&im.Kota, &im.Kapten, &im.NoHP, &im.Latitude, &im.Longitude,
			&im.JarakTempuhKm, &im.JarakDcKm,
			&im.TotalKoli, &im.TotalEcer, &im.TotalHighValue,
			&im.JumlahBarang, &im.Koli, &im.Ecer, &im.HighValue, &im.StatusPickup, &im.CatatanPickup); err != nil {
			return nil, err
		}
		items = append(items, im)
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

func (r *Repository) ListTrackingHistory(ctx context.Context, idKendaraan, idDriver int64, tanggal string) ([]TrackingCheckpoint, error) {
	query := `
		SELECT e.id_event, e.id_ritase, COALESCE(r.kode_ritase,''),
		       e.status, e.catatan, e.latitude, e.longitude,
		       e.nama_lokasi, e.durasi_detik, e.jumlah_koli, e.jumlah_ecer, e.jumlah_high_value,
		       e.created_at
		FROM ritase_event e
		JOIN ritase r ON r.id_ritase = e.id_ritase
		WHERE 1=1
	`
	var args []interface{}
	if idDriver > 0 {
		args = append(args, idDriver)
		query += " AND r.id_driver = $" + fmt.Sprint(len(args))
	} else if idKendaraan > 0 {
		args = append(args, idKendaraan)
		query += " AND r.id_kendaraan = $" + fmt.Sprint(len(args))
	}
	if tanggal != "" {
		args = append(args, tanggal)
		// Cross-midnight: return ALL events from any ritase that has ≥1 event on selected date.
		// This ensures trips spanning midnight show complete logs.
		if idDriver > 0 {
			args = append(args, idDriver)
			query += " AND e.id_ritase IN (" +
				"SELECT e2.id_ritase FROM ritase_event e2 " +
				"JOIN ritase r2 ON r2.id_ritase = e2.id_ritase " +
				"WHERE (e2.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date = $" + fmt.Sprint(len(args)-1) +
				" AND r2.id_driver = $" + fmt.Sprint(len(args)) +
				")"
		} else if idKendaraan > 0 {
			args = append(args, idKendaraan)
			query += " AND e.id_ritase IN (" +
				"SELECT e2.id_ritase FROM ritase_event e2 " +
				"JOIN ritase r2 ON r2.id_ritase = e2.id_ritase " +
				"WHERE (e2.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date = $" + fmt.Sprint(len(args)-1) +
				" AND r2.id_kendaraan = $" + fmt.Sprint(len(args)) +
				")"
		} else {
			query += " AND e.id_ritase IN (" +
				"SELECT e2.id_ritase FROM ritase_event e2 " +
				"WHERE (e2.created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Jakarta')::date = $" + fmt.Sprint(len(args)) +
				")"
		}
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
		if err := rows.Scan(
			&c.IDEvent, &c.IDRitase, &c.KodeRitase,
			&c.Status, &c.Catatan, &c.Latitude, &c.Longitude,
			&c.NamaLokasi, &c.DurasiDetik, &c.JumlahKoli, &c.JumlahEcer, &c.JumlahHighValue,
			&c.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// ListGpsHistory mengembalikan seluruh titik GPS history untuk satu ritase.
func (r *Repository) ListGpsHistory(ctx context.Context, idRitase int64) ([]GpsPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT latitude, longitude, kecepatan, created_at
		FROM armada_gps_history
		WHERE id_ritase = $1
		ORDER BY created_at ASC
	`, idRitase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []GpsPoint
	for rows.Next() {
		var p GpsPoint
		if err := rows.Scan(&p.Latitude, &p.Longitude, &p.Kecepatan, &p.CreatedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// SaveImplanBarang mencatat atau memperbarui log barang di implan.
// Dual-write: id_implant sumber baru; id_seller lama diisi bila ada padanan
// (lookup via kode) agar pembaca lama tetap jalan. Keduanya boleh NULL bila
// tak ada padanan (implant baru tanpa riwayat seller).
func (r *Repository) SaveImplanBarang(ctx context.Context, req ImplanBarangInput, createdBy string) error {
	status := req.Status
	if status == "" {
		status = "menunggu"
	}
	var idSeller, idImplant *int64
	if req.IDSeller > 0 {
		idSeller = &req.IDSeller
	}
	if req.IDImplant > 0 {
		idImplant = &req.IDImplant
		if idSeller == nil {
			var sid int64
			err := r.db.QueryRow(ctx, `
				SELECT s.id_seller FROM seller s
				JOIN implant i ON i.kode_implant = s.kode_seller
				WHERE i.id_implant = $1`, req.IDImplant).Scan(&sid)
			if err == nil {
				idSeller = &sid
			}
		}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO implan_barang_log (id_seller, id_implant, tanggal, jumlah_barang, koli, ecer, high_value, status, catatan, created_by, updated_at)
		VALUES ($1, $2, (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP)
	`, idSeller, idImplant, req.JumlahBarang, req.Koli, req.Ecer, req.HighValue, status, req.Catatan, createdBy)
	return err
}

// GetImplanBarangHistory mengambil riwayat status barang untuk satu seller.
func (r *Repository) GetImplanBarangHistory(ctx context.Context, idSeller int64) ([]ImplanBarangLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_log, id_seller, id_implant, TO_CHAR(tanggal, 'YYYY-MM-DD'), jumlah_barang, COALESCE(koli, 0), COALESCE(ecer, 0), COALESCE(high_value, 0), status,
		       COALESCE(catatan, ''), COALESCE(created_by, ''), created_at, updated_at
		FROM implan_barang_log
		WHERE id_seller = $1
		ORDER BY id_log DESC
		LIMIT 50
	`, idSeller)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ImplanBarangLog
	for rows.Next() {
		var l ImplanBarangLog
		if err := rows.Scan(&l.ID, &l.IDSeller, &l.IDImplant, &l.Tanggal, &l.JumlahBarang, &l.Koli, &l.Ecer, &l.HighValue, &l.Status,
			&l.Catatan, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetImplantBarangHistory mengambil riwayat status barang untuk satu implant.
func (r *Repository) GetImplantBarangHistory(ctx context.Context, idImplant int64) ([]ImplanBarangLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_log, id_seller, id_implant, TO_CHAR(tanggal, 'YYYY-MM-DD'), jumlah_barang, COALESCE(koli, 0), COALESCE(ecer, 0), COALESCE(high_value, 0), status,
		       COALESCE(catatan, ''), COALESCE(created_by, ''), created_at, updated_at
		FROM implan_barang_log
		WHERE id_implant = $1
		ORDER BY id_log DESC
		LIMIT 50
	`, idImplant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ImplanBarangLog
	for rows.Next() {
		var l ImplanBarangLog
		if err := rows.Scan(&l.ID, &l.IDSeller, &l.IDImplant, &l.Tanggal, &l.JumlahBarang, &l.Koli, &l.Ecer, &l.HighValue, &l.Status,
			&l.Catatan, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ListDriverPickups mengembalikan seluruh driver pickup beserta status muatannya hari ini (diagregasikan per driver per hari).
func (r *Repository) ListDriverPickups(ctx context.Context) ([]DriverPickupItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT 
			u.id_user,
			u.username,
			COALESCE(d.nama_driver, u.username) AS nama_driver,
			COALESCE(d.no_hp, '') AS no_hp,
			COALESCE(l.jumlah_barang, 0) AS jumlah_barang,
			COALESCE(l.koli, 0) AS koli,
			COALESCE(l.ecer, 0) AS ecer,
			COALESCE(l.high_value, 0) AS high_value,
			COALESCE(l.status, 'standby') AS status,
			COALESCE(l.catatan, '') AS catatan,
			COALESCE(l.asal_seller, '') AS asal_seller,
			l.updated_at
		FROM users u
		LEFT JOIN driver d ON LOWER(d.nama_driver) = LOWER(u.username)
		LEFT JOIN LATERAL (
			WITH last_selesai AS (
				SELECT COALESCE(MAX(id_log), 0) AS max_selesai_id
				FROM driver_pickup_log
				WHERE id_user = u.id_user 
				  AND status = 'selesai'
				  AND (tanggal = CURRENT_DATE OR tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
			),
			active_logs AS (
				SELECT dpl.*
				FROM driver_pickup_log dpl, last_selesai ls
				WHERE dpl.id_user = u.id_user
				  AND (dpl.tanggal = CURRENT_DATE OR dpl.tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				  AND dpl.id_log > ls.max_selesai_id
			),
			latest_overall AS (
				SELECT status, catatan, updated_at, asal_seller
				FROM driver_pickup_log
				WHERE id_user = u.id_user
				  AND (tanggal = CURRENT_DATE OR tanggal = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE)
				ORDER BY id_log DESC
				LIMIT 1
			)
			SELECT 
				COALESCE(SUM(al.jumlah_barang), 0)::int AS jumlah_barang,
				COALESCE(SUM(al.koli), 0)::int AS koli,
				COALESCE(SUM(al.ecer), 0)::int AS ecer,
				COALESCE(SUM(al.high_value), 0)::int AS high_value,
				COALESCE(
					(ARRAY_AGG(al.status ORDER BY al.id_log DESC))[1],
					(SELECT lo.status FROM latest_overall lo),
					'standby'
				) AS status,
				COALESCE(
					NULLIF(
						STRING_AGG(DISTINCT NULLIF(TRIM(CASE WHEN al.asal_seller <> 'Gudang' THEN al.asal_seller END), ''), ', '),
						''
					),
					(SELECT lo.asal_seller FROM latest_overall lo),
					'Gudang'
				) AS asal_seller,
				COALESCE(
					(ARRAY_AGG(al.catatan ORDER BY al.id_log DESC))[1],
					(SELECT lo.catatan FROM latest_overall lo),
					''
				) AS catatan,
				COALESCE(MAX(al.updated_at), (SELECT lo.updated_at FROM latest_overall lo)) AS updated_at
			FROM active_logs al
		) l ON TRUE
		WHERE LOWER(TRIM(u.role)) = 'driver_pickup' AND (u.status = 'aktif' OR u.status IS NULL OR u.status = '')
		ORDER BY u.username ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DriverPickupItem, 0)
	for rows.Next() {
		var item DriverPickupItem
		if err := rows.Scan(
			&item.IDUser, &item.Username, &item.NamaDriver, &item.NoHP,
			&item.JumlahBarang, &item.Koli, &item.Ecer, &item.HighValue,
			&item.Status, &item.Catatan, &item.AsalSeller, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SaveDriverPickupBarang mencatat log muatan driver pickup (single).
func (r *Repository) SaveDriverPickupBarang(ctx context.Context, req DriverPickupInput, createdBy string) error {
	status := req.Status
	if status == "" {
		status = "menuju_gudang"
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO driver_pickup_log (id_user, nama_driver, tanggal, jumlah_barang, koli, ecer, high_value, status, catatan, asal_seller, created_by, updated_at)
		VALUES ($1, $2, (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_TIMESTAMP)
	`, req.IDUser, req.NamaDriver, req.JumlahBarang, req.Koli, req.Ecer, req.HighValue, status, req.Catatan, req.AsalSeller, createdBy)
	return err
}

// SaveDriverPickupBatch mencatat banyak seller sekaligus untuk satu driver pickup.
func (r *Repository) SaveDriverPickupBatch(ctx context.Context, req DriverPickupBatchInput, createdBy string) error {
	if len(req.Items) == 0 {
		return nil
	}
	status := req.Status
	if status == "" {
		status = "menuju_gudang"
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, item := range req.Items {
		seller := strings.TrimSpace(item.AsalSeller)
		if seller == "" && item.JumlahBarang <= 0 {
			continue
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO driver_pickup_log (id_user, nama_driver, tanggal, jumlah_barang, koli, ecer, high_value, status, catatan, asal_seller, created_by, updated_at)
			VALUES ($1, $2, (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::DATE, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_TIMESTAMP)
		`, req.IDUser, req.NamaDriver, item.JumlahBarang, item.Koli, item.Ecer, item.HighValue, status, req.Catatan, seller, createdBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// ListAllDriverPickupHistory mengambil seluruh riwayat muatan driver pickup dengan filter tanggal dan driver opsional.
func (r *Repository) ListAllDriverPickupHistory(ctx context.Context, idUser int64, startDate, endDate, status string, limit int) ([]DriverPickupLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	query := `
		SELECT id_log, id_user, nama_driver, TO_CHAR(tanggal, 'YYYY-MM-DD'), jumlah_barang, COALESCE(koli, 0), COALESCE(ecer, 0), COALESCE(high_value, 0), status,
		       COALESCE(catatan, ''), COALESCE(asal_seller, ''), COALESCE(created_by, ''), created_at, updated_at
		FROM driver_pickup_log
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if idUser > 0 {
		query += fmt.Sprintf(" AND id_user = $%d", argIdx)
		args = append(args, idUser)
		argIdx++
	}
	if startDate != "" {
		query += fmt.Sprintf(" AND tanggal >= $%d::DATE", argIdx)
		args = append(args, startDate)
		argIdx++
	}
	if endDate != "" {
		query += fmt.Sprintf(" AND tanggal <= $%d::DATE", argIdx)
		args = append(args, endDate)
		argIdx++
	}
	if status != "" && status != "all" {
		query += fmt.Sprintf(" AND LOWER(status) = LOWER($%d)", argIdx)
		args = append(args, status)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY id_log DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []DriverPickupLog
	for rows.Next() {
		var l DriverPickupLog
		if err := rows.Scan(&l.ID, &l.IDUser, &l.NamaDriver, &l.Tanggal, &l.JumlahBarang, &l.Koli, &l.Ecer, &l.HighValue, &l.Status,
			&l.Catatan, &l.AsalSeller, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetDriverPickupHistory mengambil riwayat muatan untuk satu driver pickup.
func (r *Repository) GetDriverPickupHistory(ctx context.Context, idUser int64) ([]DriverPickupLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_log, id_user, nama_driver, TO_CHAR(tanggal, 'YYYY-MM-DD'), jumlah_barang, COALESCE(koli, 0), COALESCE(ecer, 0), COALESCE(high_value, 0), status,
		       COALESCE(catatan, ''), COALESCE(asal_seller, ''), COALESCE(created_by, ''), created_at, updated_at
		FROM driver_pickup_log
		WHERE id_user = $1
		ORDER BY id_log DESC
		LIMIT 50
	`, idUser)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []DriverPickupLog
	for rows.Next() {
		var l DriverPickupLog
		if err := rows.Scan(&l.ID, &l.IDUser, &l.NamaDriver, &l.Tanggal, &l.JumlahBarang, &l.Koli, &l.Ecer, &l.HighValue, &l.Status,
			&l.Catatan, &l.AsalSeller, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
