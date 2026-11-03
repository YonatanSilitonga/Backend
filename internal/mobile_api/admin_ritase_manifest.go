package mobile_api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

// AdminGetManifestPhotos mengambil seluruh daftar foto bukti manifest yang diunggah driver
// GET /api/v1/admin/manifest-photos?tanggal=YYYY-MM-DD&driver_id=&search=&jenis_ritase=&ritase_ke=&input_by=
func (h *APIHandler) AdminGetManifestPhotos(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tanggalParam := c.QueryParam("tanggal")
	driverIDParam, _ := strconv.ParseInt(c.QueryParam("driver_id"), 10, 64)
	searchParam := strings.TrimSpace(c.QueryParam("search"))
	jenisRitaseParam := strings.TrimSpace(c.QueryParam("jenis_ritase"))
	ritaseKeParam, _ := strconv.Atoi(c.QueryParam("ritase_ke"))
	inputByParam := strings.TrimSpace(c.QueryParam("input_by"))

	// Subquery 1: data dari ritase_event (driver upload foto)
	// Subquery 2: data dari input_kapten (kapten input muatan)
	// Filter diterapkan di outer query supaya konsisten untuk kedua sumber.
	query := `
		SELECT * FROM (
			(
				SELECT
					ev.id_event,
					0 AS id_seller,
					0 AS id_implant,
					COALESCE(ev.id_ritase, 0) AS id_ritase,
					COALESCE(r.kode_ritase, 'R-' || COALESCE(ev.id_ritase, 0)) AS kode_ritase,
					COALESCE(r.tanggal, (ev.created_at + interval '7 hours')::date) AS tanggal,
					COALESCE(r.ritase_ke, 1) AS ritase_ke,
					COALESCE(r.jenis_ritase, '') AS jenis_ritase,
					COALESCE(r.id_driver, 0) AS id_driver,
					COALESCE(d.nama_driver, 'Driver') AS nama_driver,
					COALESCE(d.jabatan, 'TRANSPORTER') AS jabatan_driver,
					COALESCE(r.id_kendaraan, 0) AS id_kendaraan,
					COALESCE(k.plat_nomor, '-') AS nopol,
					COALESCE(k.jenis_kendaraan, 'Blindvan') AS jenis_kendaraan,
					COALESCE(ev.nama_lokasi, 'Lokasi') AS nama_lokasi,
					COALESCE(ev.status, 'Muat Barang') AS status,
					COALESCE(ev.jumlah_awb, 0) AS jumlah_awb,
					COALESCE(ev.koli_jkt, 0) AS koli_jkt,
					COALESCE(ev.koli_seg, 0) AS koli_seg,
					COALESCE(ev.koli_btn, 0) AS koli_btn,
					COALESCE(ev.ecer_jkt, 0) AS ecer_jkt,
					COALESCE(ev.ecer_seg, 0) AS ecer_seg,
					COALESCE(ev.ecer_btn, 0) AS ecer_btn,
					COALESCE(ev.koli_hv_jkt, 0) AS koli_hv_jkt,
					COALESCE(ev.koli_hv_seg, 0) AS koli_hv_seg,
					COALESCE(ev.koli_hv_btn, 0) AS koli_hv_btn,
					COALESCE(ev.ecer_hv_jkt, 0) AS ecer_hv_jkt,
					COALESCE(ev.ecer_hv_seg, 0) AS ecer_hv_seg,
					COALESCE(ev.ecer_hv_btn, 0) AS ecer_hv_btn,
					0 AS awb_hv_jkt,
					0 AS awb_hv_seg,
					0 AS awb_hv_btn,
					COALESCE(ev.jumlah_koli, 0) AS jumlah_koli,
					COALESCE(ev.jumlah_ecer, 0) AS jumlah_ecer,
					COALESCE(ev.jumlah_high_value, 0) AS jumlah_high_value,
					COALESCE(ev.durasi_detik, 0) AS durasi_detik,
					COALESCE(ev.foto_manifest_url, '') AS foto_manifest_url,
					ev.created_at,
					COALESCE(ev.input_by, 'driver') AS input_by,
					ev.input_by_id,
					CASE WHEN ev.input_by = 'kapten' THEN COALESCE(u.username, 'Kapten')
					     ELSE COALESCE(d.nama_driver, 'Driver')
					END AS input_by_name,
					CASE WHEN ev.updated_at IS NOT NULL THEN true ELSE false END AS is_updated,
					ev.updated_at
				FROM ritase_event ev
				LEFT JOIN ritase r ON r.id_ritase = ev.id_ritase
				LEFT JOIN driver d ON d.id_driver = r.id_driver
				LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
				LEFT JOIN users u ON ev.input_by = 'kapten' AND u.id_user = ev.input_by_id
				WHERE ev.foto_manifest_url IS NOT NULL AND ev.foto_manifest_url != ''
			)

			UNION ALL

			(
				SELECT
					ik.id AS id_event,
					COALESCE(ik.id_seller, 0) AS id_seller,
					COALESCE(ik.id_implant, 0) AS id_implant,
					COALESCE(ik.id_ritase, 0) AS id_ritase,
					COALESCE(r.kode_ritase, 'IK-' || ik.id) AS kode_ritase,
					COALESCE(r.tanggal, (ik.created_at + interval '7 hours')::date) AS tanggal,
					ik.ritase_ke,
					COALESCE(ik.jenis_ritase, 'outgoing') AS jenis_ritase,
					COALESCE(r.id_driver, 0) AS id_driver,
					COALESCE(d.nama_driver, u.username, 'Kapten') AS nama_driver,
					'KAPTEN' AS jabatan_driver,
					COALESCE(r.id_kendaraan, 0) AS id_kendaraan,
					COALESCE(k.plat_nomor, '-') AS nopol,
					'-' AS jenis_kendaraan,
					COALESCE(ik.nama_lokasi, 'Lokasi') AS nama_lokasi,
					'Muat Barang' AS status,
					COALESCE(ik.jumlah_awb, 0) AS jumlah_awb,
					COALESCE(ik.koli_jkt, 0) AS koli_jkt,
					COALESCE(ik.koli_seg, 0) AS koli_seg,
					COALESCE(ik.koli_btn, 0) AS koli_btn,
					COALESCE(ik.ecer_jkt, 0) AS ecer_jkt,
					COALESCE(ik.ecer_seg, 0) AS ecer_seg,
					COALESCE(ik.ecer_btn, 0) AS ecer_btn,
					COALESCE(ik.koli_hv_jkt, 0) AS koli_hv_jkt,
					COALESCE(ik.koli_hv_seg, 0) AS koli_hv_seg,
					COALESCE(ik.koli_hv_btn, 0) AS koli_hv_btn,
					COALESCE(ik.ecer_hv_jkt, 0) AS ecer_hv_jkt,
					COALESCE(ik.ecer_hv_seg, 0) AS ecer_hv_seg,
					COALESCE(ik.ecer_hv_btn, 0) AS ecer_hv_btn,
					COALESCE(ik.awb_hv_jkt, 0) AS awb_hv_jkt,
					COALESCE(ik.awb_hv_seg, 0) AS awb_hv_seg,
					COALESCE(ik.awb_hv_btn, 0) AS awb_hv_btn,
					0 AS jumlah_koli, 0 AS jumlah_ecer, 0 AS jumlah_high_value,
					0 AS durasi_detik,
					COALESCE(ik.foto_manifest_url, '') AS foto_manifest_url,
					ik.created_at,
					'kapten' AS input_by,
					ik.id_user AS input_by_id,
					COALESCE(u.username, 'Kapten') AS input_by_name,
					CASE WHEN ik.updated_at IS NOT NULL THEN true ELSE false END AS is_updated,
					ik.updated_at
				FROM input_kapten ik
				LEFT JOIN ritase r ON r.id_ritase = ik.id_ritase
				LEFT JOIN driver d ON d.id_driver = r.id_driver
				LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
				LEFT JOIN users u ON u.id_user = ik.id_user
			)
		) sub
		WHERE 1=1
	`

	args := make([]interface{}, 0)
	argIdx := 1

	if tanggalParam != "" {
		query += fmt.Sprintf(" AND sub.tanggal = $%d::date", argIdx)
		args = append(args, tanggalParam)
		argIdx++
	}

	if driverIDParam > 0 {
		query += fmt.Sprintf(" AND sub.id_driver = $%d", argIdx)
		args = append(args, driverIDParam)
		argIdx++
	}

	if searchParam != "" {
		likePattern := "%" + strings.ToLower(searchParam) + "%"
		query += fmt.Sprintf(" AND (LOWER(sub.nama_driver) LIKE $%d OR LOWER(sub.nopol) LIKE $%d OR LOWER(sub.nama_lokasi) LIKE $%d OR LOWER(sub.kode_ritase) LIKE $%d)", argIdx, argIdx, argIdx, argIdx)
		args = append(args, likePattern)
		argIdx++
	}

	if jenisRitaseParam != "" {
		query += fmt.Sprintf(" AND sub.jenis_ritase = $%d", argIdx)
		args = append(args, jenisRitaseParam)
		argIdx++
	}

	if inputByParam != "" {
		query += fmt.Sprintf(" AND sub.input_by = $%d", argIdx)
		args = append(args, inputByParam)
		argIdx++
	}

	if ritaseKeParam > 0 {
		query += fmt.Sprintf(" AND sub.ritase_ke = $%d", argIdx)
		args = append(args, ritaseKeParam)
		argIdx++
	}

	query += " ORDER BY sub.created_at DESC LIMIT 200"

	rows, err := h.DB.Query(ctx, query, args...)
	if err != nil {
		log.Printf("[ManifestPhotos] query error: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil foto manifest: "+err.Error())
	}
	defer rows.Close()

	photos := make([]map[string]interface{}, 0)
	for rows.Next() {
		var idEvent, idSeller, idImplant, idRitase, idDriver, idKendaraan int64
		var kodeRitase, tanggal, namaDriver, jabatanDriver, nopol, jenisKendaraan, namaLokasi, status, fotoURL string
		var ritaseKe int
		var jenisRitase string
		var awb, durasiDetik int
		var koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
		var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
		var awbHvJkt, awbHvSeg, awbHvBtn int
		var jumlahKoli, jumlahEcer, jumlahHighValue int
		var createdAt time.Time
		var inputBy, inputByName string
		var inputByID *int64
		var isUpdated bool
		var updatedAt *time.Time

		if err := rows.Scan(
			&idEvent, &idSeller, &idImplant, &idRitase, &kodeRitase, &tanggal, &ritaseKe, &jenisRitase,
			&idDriver, &namaDriver, &jabatanDriver,
			&idKendaraan, &nopol, &jenisKendaraan,
			&namaLokasi, &status,
			&awb, &koliJkt, &koliSeg, &koliBtn,
			&ecerJkt, &ecerSeg, &ecerBtn,
			&koliHvJkt, &koliHvSeg, &koliHvBtn,
			&ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
			&awbHvJkt, &awbHvSeg, &awbHvBtn,
			&jumlahKoli, &jumlahEcer, &jumlahHighValue,
			&durasiDetik,
			&fotoURL, &createdAt,
			&inputBy, &inputByID, &inputByName,
			&isUpdated, &updatedAt,
		); err != nil {
			// Jangan telan error diam-diam — baris gagal scan harus terlihat di log.
			log.Printf("[ManifestPhotos] scan error (baris %d): %v", idEvent, err)
			continue
		}
		totalKoli := koliJkt + koliSeg + koliBtn
		if totalKoli == 0 {
			totalKoli = jumlahKoli
		}
		totalEcer := ecerJkt + ecerSeg + ecerBtn
		if totalEcer == 0 {
			totalEcer = jumlahEcer
		}
		totalHV := koliHvJkt + koliHvSeg + koliHvBtn + ecerHvJkt + ecerHvSeg + ecerHvBtn + awbHvJkt + awbHvSeg + awbHvBtn
		if totalHV == 0 {
			totalHV = jumlahHighValue
		}
		photos = append(photos, map[string]interface{}{
			"id_event":        idEvent,
			"id_seller":       idSeller,
			"id_implant":      idImplant,
			"id_ritase":       idRitase,
			"kode_ritase":     kodeRitase,
			"tanggal":         tanggal,
			"ritase_ke":       ritaseKe,
			"jenis_ritase":    jenisRitase,
			"id_driver":       idDriver,
			"nama_driver":     namaDriver,
			"jabatan_driver":  jabatanDriver,
			"id_kendaraan":    idKendaraan,
			"nopol":           nopol,
			"jenis_kendaraan": jenisKendaraan,
			"nama_lokasi":     namaLokasi,
			"status":          status,
			"jumlah_awb":      awb,
			"koli_jkt":        koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
			"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
			"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
			"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
			"awb_hv_jkt": awbHvJkt, "awb_hv_seg": awbHvSeg, "awb_hv_btn": awbHvBtn,
			"awb_hv":            awbHvJkt + awbHvSeg + awbHvBtn,
			"total_koli":        totalKoli,
			"total_ecer":        totalEcer,
			"total_hv":          totalHV,
			"durasi_detik":      durasiDetik,
			"foto_manifest_url": fotoURL,
			"created_at":        createdAt.Format(time.RFC3339),
			"input_by":          inputBy,
			"input_by_id":       inputByID,
			"input_by_name":     inputByName,
			"is_updated":        isUpdated,
			"updated_at":        updatedAt,
		})
	}

	return response.OK(c, photos)
}

// AdminGetKonfirmasiPenjemputan mengambil ringkasan + riwayat serah terima
// kapten (diambil & sisa) untuk tanggal tertentu.
// GET /api/v1/manifest-konfirmasi-penjemputan?tanggal=YYYY-MM-DD&ritase_ke=&jenis_ritase=&search=
//
// ringkasan: per (seller, jenis, rit) → input vs diambil vs sisa (tanggal WIB).
// riwayat: tiap kejadian penjemputan (foto, catatan, driver, waktu).
func (h *APIHandler) AdminGetKonfirmasiPenjemputan(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	tanggalParam := c.QueryParam("tanggal")
	ritaseKeParam, _ := strconv.Atoi(c.QueryParam("ritase_ke"))
	jenisRitaseParam := strings.TrimSpace(c.QueryParam("jenis_ritase"))
	searchParam := strings.TrimSpace(c.QueryParam("search"))

	// ── RINGKASAN per (seller, jenis, rit) ──
	ringkasanQuery := `
		WITH inp AS (
			SELECT t.id_implant, t.id_seller, t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv
			FROM input_kapten t
			WHERE 1=1
	`
	ringkasanArgs := []interface{}{}
	argIdx := 1
	if tanggalParam != "" {
		ringkasanQuery += fmt.Sprintf(" AND (t.created_at + interval '7 hours')::date = $%d::date", argIdx)
		ringkasanArgs = append(ringkasanArgs, tanggalParam)
		argIdx++
	}
	ringkasanQuery += ` GROUP BY t.id_implant, t.id_seller, t.jenis_ritase, t.ritase_ke
		),
		amb AS (
			SELECT t.id_implant, t.id_seller, t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv
			FROM konfirmasi_penjemputan t
			WHERE 1=1
	`
	if tanggalParam != "" {
		ringkasanQuery += fmt.Sprintf(" AND (t.created_at + interval '7 hours')::date = $%d::date", argIdx)
		ringkasanArgs = append(ringkasanArgs, tanggalParam)
		argIdx++
	}
	ringkasanQuery += ` GROUP BY t.id_implant, t.id_seller, t.jenis_ritase, t.ritase_ke
		)
		SELECT COALESCE(i.id_implant, a.id_implant, 0) AS id_implant,
		       COALESCE(i.id_seller, a.id_seller, 0) AS id_seller,
		       COALESCE(s.nama_seller, ik.nama_lokasi, 'Seller') AS nama_lokasi,
		       COALESCE(i.jenis_ritase, a.jenis_ritase, '') AS jenis_ritase,
		       COALESCE(i.ritase_ke, a.ritase_ke, 0) AS ritase_ke,
		       COALESCE(i.awb, 0), COALESCE(i.koli, 0), COALESCE(i.ecer, 0), COALESCE(i.hv, 0),
		       COALESCE(a.awb, 0), COALESCE(a.koli, 0), COALESCE(a.ecer, 0), COALESCE(a.hv, 0)
		FROM inp i
		FULL OUTER JOIN amb a
		  ON COALESCE(i.id_implant, 0) = COALESCE(a.id_implant, 0)
		 AND COALESCE(i.id_seller, 0) = COALESCE(a.id_seller, 0)
		 AND COALESCE(i.jenis_ritase, '') = COALESCE(a.jenis_ritase, '')
		 AND COALESCE(i.ritase_ke, 0) = COALESCE(a.ritase_ke, 0)
		LEFT JOIN seller s ON s.id_seller = COALESCE(i.id_seller, a.id_seller)
		LEFT JOIN LATERAL (
			SELECT NULLIF(TRIM(nama_lokasi), '') AS nama_lokasi
			FROM input_kapten
			WHERE id_implant = COALESCE(i.id_implant, a.id_implant)
			ORDER BY created_at DESC
			LIMIT 1
		) ik ON true
		WHERE 1=1
	`
	if ritaseKeParam > 0 {
		ringkasanQuery += fmt.Sprintf(" AND COALESCE(i.ritase_ke, a.ritase_ke, 0) = $%d", argIdx)
		ringkasanArgs = append(ringkasanArgs, ritaseKeParam)
		argIdx++
	}
	if jenisRitaseParam != "" {
		ringkasanQuery += fmt.Sprintf(" AND COALESCE(i.jenis_ritase, a.jenis_ritase, '') = $%d", argIdx)
		ringkasanArgs = append(ringkasanArgs, jenisRitaseParam)
		argIdx++
	}
	if searchParam != "" {
		ringkasanQuery += fmt.Sprintf(" AND LOWER(COALESCE(s.nama_seller, ik.nama_lokasi, '')) LIKE $%d", argIdx)
		ringkasanArgs = append(ringkasanArgs, "%"+strings.ToLower(searchParam)+"%")
		argIdx++
	}
	ringkasanQuery += ` ORDER BY nama_lokasi, ritase_ke`

	ringkasan := []map[string]interface{}{}
	rRows, err := h.DB.Query(ctx, ringkasanQuery, ringkasanArgs...)
	if err != nil {
		log.Printf("[ManifestPenjemputan] ringkasan error: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil ringkasan: "+err.Error())
	}
	for rRows.Next() {
		var idImplant, idSeller int64
		var namaLokasi, jenis string
		var ritKe int
		var inAwb, inKoli, inEcer, inHV, amAwb, amKoli, amEcer, amHV int
		if err := rRows.Scan(&idImplant, &idSeller, &namaLokasi, &jenis, &ritKe,
			&inAwb, &inKoli, &inEcer, &inHV, &amAwb, &amKoli, &amEcer, &amHV); err != nil {
			log.Printf("[ManifestPenjemputan] scan ringkasan error: %v", err)
			continue
		}
		ringkasan = append(ringkasan, map[string]interface{}{
			"id_implant": idImplant, "id_seller": idSeller, "nama_lokasi": namaLokasi,
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"input_awb": inAwb, "input_koli": inKoli, "input_ecer": inEcer, "input_hv": inHV,
			"diambil_awb": amAwb, "diambil_koli": amKoli, "diambil_ecer": amEcer, "diambil_hv": amHV,
			"sisa_awb": inAwb - amAwb, "sisa_koli": inKoli - amKoli,
			"sisa_ecer": inEcer - amEcer, "sisa_hv": inHV - amHV,
		})
	}
	rRows.Close()

	// ── RIWAYAT kejadian penjemputan ──
	riwayatQuery := `
		SELECT kp.id, COALESCE(kp.id_ritase, 0),
		       COALESCE(r.kode_ritase, ''),
		       (kp.created_at + interval '7 hours')::date AS tanggal,
		       kp.ritase_ke, COALESCE(kp.jenis_ritase, ''),
		       COALESCE(kp.id_implant, 0),
		       COALESCE(kp.id_seller, 0), COALESCE(s.nama_seller, ik.nama_lokasi, 'Seller'),
		       COALESCE(d.nama_driver, ''),
		       COALESCE(kp.jumlah_awb, 0),
		       COALESCE(kp.koli_jkt + kp.koli_seg + kp.koli_btn, 0),
		       COALESCE(kp.ecer_jkt + kp.ecer_seg + kp.ecer_btn, 0),
		       COALESCE(kp.koli_hv_jkt + kp.koli_hv_seg + kp.koli_hv_btn + kp.ecer_hv_jkt + kp.ecer_hv_seg + kp.ecer_hv_btn + kp.awb_hv_jkt + kp.awb_hv_seg + kp.awb_hv_btn, 0),
		       COALESCE(kp.koli_jkt, 0), COALESCE(kp.koli_seg, 0), COALESCE(kp.koli_btn, 0),
		       COALESCE(kp.ecer_jkt, 0), COALESCE(kp.ecer_seg, 0), COALESCE(kp.ecer_btn, 0),
		       COALESCE(kp.koli_hv_jkt, 0), COALESCE(kp.koli_hv_seg, 0), COALESCE(kp.koli_hv_btn, 0),
		       COALESCE(kp.ecer_hv_jkt, 0), COALESCE(kp.ecer_hv_seg, 0), COALESCE(kp.ecer_hv_btn, 0),
		       COALESCE(kp.awb_hv_jkt, 0), COALESCE(kp.awb_hv_seg, 0), COALESCE(kp.awb_hv_btn, 0),
		       COALESCE(kp.foto_penjemputan_url, ''),
		       COALESCE(kp.catatan, ''),
		       kp.created_at
		FROM konfirmasi_penjemputan kp
		LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN seller s ON s.id_seller = kp.id_seller
		LEFT JOIN LATERAL (
			SELECT NULLIF(TRIM(nama_lokasi), '') AS nama_lokasi
			FROM input_kapten
			WHERE id_implant = kp.id_implant
			ORDER BY created_at DESC
			LIMIT 1
		) ik ON true
		WHERE 1=1
	`
	riwayatArgs := []interface{}{}
	argIdx = 1
	if tanggalParam != "" {
		riwayatQuery += fmt.Sprintf(" AND (kp.created_at + interval '7 hours')::date = $%d::date", argIdx)
		riwayatArgs = append(riwayatArgs, tanggalParam)
		argIdx++
	}
	if ritaseKeParam > 0 {
		riwayatQuery += fmt.Sprintf(" AND kp.ritase_ke = $%d", argIdx)
		riwayatArgs = append(riwayatArgs, ritaseKeParam)
		argIdx++
	}
	if jenisRitaseParam != "" {
		riwayatQuery += fmt.Sprintf(" AND kp.jenis_ritase = $%d", argIdx)
		riwayatArgs = append(riwayatArgs, jenisRitaseParam)
		argIdx++
	}
	if searchParam != "" {
		like := "%" + strings.ToLower(searchParam) + "%"
		riwayatQuery += fmt.Sprintf(" AND (LOWER(COALESCE(s.nama_seller, ik.nama_lokasi, '')) LIKE $%d OR LOWER(COALESCE(d.nama_driver, '')) LIKE $%d OR LOWER(COALESCE(r.kode_ritase, '')) LIKE $%d)", argIdx, argIdx, argIdx)
		riwayatArgs = append(riwayatArgs, like)
		argIdx++
	}
	riwayatQuery += ` ORDER BY kp.created_at DESC LIMIT 200`

	riwayat := []map[string]interface{}{}
	wRows, err := h.DB.Query(ctx, riwayatQuery, riwayatArgs...)
	if err != nil {
		log.Printf("[ManifestPenjemputan] riwayat error: %v", err)
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil riwayat: "+err.Error())
	}
	for wRows.Next() {
		var id, idRitase, idImplant, idSeller int64
		var kode, tanggal, jenis, namaLokasi, namaDriver, foto, catatan string
		var ritKe int
		var awb, koli, ecer, hv int
		var kj, ks, kb, ej, es, eb, khj, khs, khb, ehj, ehs, ehb, ahj, ahs, ahb int
		var createdAt time.Time
		if err := wRows.Scan(&id, &idRitase, &kode, &tanggal, &ritKe, &jenis,
			&idImplant, &idSeller, &namaLokasi, &namaDriver, &awb, &koli, &ecer, &hv,
			&kj, &ks, &kb, &ej, &es, &eb, &khj, &khs, &khb, &ehj, &ehs, &ehb, &ahj, &ahs, &ahb,
			&foto, &catatan, &createdAt); err != nil {
			log.Printf("[ManifestPenjemputan] scan riwayat error: %v", err)
			continue
		}
		riwayat = append(riwayat, map[string]interface{}{
			"id": id, "id_ritase": idRitase, "kode_ritase": kode,
			"tanggal": tanggal, "ritase_ke": ritKe, "jenis_ritase": jenis,
			"id_implant": idImplant,
			"id_seller":  idSeller, "nama_lokasi": namaLokasi, "nama_driver": namaDriver,
			"jumlah_awb": awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
			"koli_jkt": kj, "koli_seg": ks, "koli_btn": kb,
			"ecer_jkt": ej, "ecer_seg": es, "ecer_btn": eb,
			"koli_hv_jkt": khj, "koli_hv_seg": khs, "koli_hv_btn": khb,
			"ecer_hv_jkt": ehj, "ecer_hv_seg": ehs, "ecer_hv_btn": ehb,
			"awb_hv_jkt": ahj, "awb_hv_seg": ahs, "awb_hv_btn": ahb,
			"awb_hv":               ahj + ahs + ahb,
			"foto_penjemputan_url": foto, "catatan": catatan,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	wRows.Close()

	return response.OK(c, map[string]interface{}{
		"ringkasan": ringkasan,
		"riwayat":   riwayat,
	})
}
