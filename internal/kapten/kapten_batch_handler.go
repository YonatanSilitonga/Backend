package kapten

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

func parseTimeMinutes(s string) int {
	hour, min := 0, 0
	fmt.Sscanf(s, "%d:%d", &hour, &min)
	return hour*60 + min
}

type ritWindow struct {
	startMin int
	endMin   int
}

var defaultRitWindows = map[string]ritWindow{
	"outgoing|1": {startMin: 0, endMin: 19*60 + 59},
	"outgoing|2": {startMin: 16 * 60, endMin: 23*60 + 59},
	"outgoing|3": {startMin: 20 * 60, endMin: 3 * 60},
	"incoming|1": {startMin: 1 * 60, endMin: 4*60 + 30},
	"incoming|2": {startMin: 7 * 60, endMin: 10*60 + 30},
	"incoming|3": {startMin: 13 * 60, endMin: 16*60 + 30},
	"incoming|4": {startMin: 19 * 60, endMin: 22*60 + 30},
}

func ritWindowKey(jenis string, ritKe int) string {
	return fmt.Sprintf("%s|%d", jenis, ritKe)
}

func (h *Handler) loadRitWindows(ctx context.Context) map[string]ritWindow {
	out := make(map[string]ritWindow, len(defaultRitWindows))
	for k, v := range defaultRitWindows {
		out[k] = v
	}
	rows, err := h.DB.Query(ctx,
		`SELECT jenis, ritase_ke, jam_mulai::text, jam_selesai::text
		 FROM jadwal_ritase_config`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var jenis, mulai, selesai string
		var ritKe int
		if err := rows.Scan(&jenis, &ritKe, &mulai, &selesai); err != nil {
			continue
		}
		out[ritWindowKey(jenis, ritKe)] = ritWindow{
			startMin: parseTimeMinutes(mulai),
			endMin:   parseTimeMinutes(selesai),
		}
	}
	return out
}

func ritDeadline(createdAt time.Time, w ritWindow, loc *time.Location) time.Time {
	createdAt = createdAt.In(loc)
	year, month, day := createdAt.Date()
	deadline := time.Date(
		year, month, day,
		w.endMin/60, w.endMin%60, 59, int(999*time.Millisecond), loc,
	)
	if w.endMin <= w.startMin {
		createdMin := createdAt.Hour()*60 + createdAt.Minute()
		if createdMin >= w.startMin {
			deadline = deadline.AddDate(0, 0, 1)
		}
	}
	return deadline
}

func ritOverdue(createdAt time.Time, w ritWindow, now time.Time) bool {
	loc := now.Location()
	return now.After(ritDeadline(createdAt, w, loc))
}

func pendingOverdue(p map[string]interface{}, windows map[string]ritWindow, now time.Time) bool {
	raw := fmt.Sprint(p["created_at"])
	createdAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	var ritKe int
	fmt.Sscanf(fmt.Sprint(p["ritase_ke"]), "%d", &ritKe)
	jenis := fmt.Sprint(p["jenis_ritase"])
	w, ok := windows[ritWindowKey(jenis, ritKe)]
	if !ok {
		return false
	}
	return ritOverdue(createdAt, w, now)
}

func ritDateOverdue(tanggal, jenis string, ritKe int, windows map[string]ritWindow, now time.Time) bool {
	w, ok := windows[ritWindowKey(jenis, ritKe)]
	if !ok {
		return false
	}
	loc := now.Location()
	day, err := time.ParseInLocation("2006-01-02", tanggal, loc)
	if err != nil {
		return false
	}
	anchor := time.Date(
		day.Year(), day.Month(), day.Day(),
		w.startMin/60, w.startMin%60, 0, 0, loc,
	)
	return now.After(ritDeadline(anchor, w, loc))
}

// myImplantIDs mengembalikan id implant milik kapten yang login.
func (h *Handler) myImplantIDs(ctx context.Context, userID int64) ([]int64, map[int64]string, error) {
	rows, err := h.DB.Query(ctx, `
		SELECT i.id_implant, COALESCE(i.nama_implant, '')
		FROM implant i
		JOIN kapten_implant_map kim ON kim.id_implant = i.id_implant
		WHERE kim.id_user = $1 AND i.status = 'aktif'
		ORDER BY i.nama_implant
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []int64
	names := map[int64]string{}
	for rows.Next() {
		var id int64
		var nama string
		if err := rows.Scan(&id, &nama); err == nil {
			ids = append(ids, id)
			names[id] = nama
		}
	}
	return ids, names, rows.Err()
}

// implantPointsForUser memetakan implant sumber -> titik penjemputan.
func (h *Handler) implantPointsForUser(ctx context.Context, userID int64) (map[int64]int64, map[int64]string) {
	points := map[int64]int64{}
	names := map[int64]string{}
	rows, err := h.DB.Query(ctx, `
		SELECT kim.id_implant, COALESCE(kim.id_titik_implant, i.id_implant),
		       COALESCE(ht.nama_implant, i.nama_implant)
		FROM kapten_implant_map kim
		JOIN implant i ON i.id_implant = kim.id_implant
		LEFT JOIN implant ht ON ht.id_implant = COALESCE(kim.id_titik_implant, i.id_implant)
		WHERE kim.id_user = $1`, userID)
	if err != nil {
		return points, names
	}
	defer rows.Close()
	for rows.Next() {
		var source, point int64
		var name string
		if err := rows.Scan(&source, &point, &name); err == nil {
			points[source] = point
			names[point] = name
		}
	}
	return points, names
}

// implantPointForUser mengambil titik satu implant. Jika mapping tidak ada,
// implant dianggap titik sendiri (kompatibilitas data lama).
func (h *Handler) implantPointForUser(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, userID, implantID int64) (int64, string) {
	var point int64
	var name string
	err := q.QueryRow(ctx, `
		SELECT COALESCE(kim.id_titik_implant, i.id_implant), COALESCE(ht.nama_implant, i.nama_implant)
		FROM kapten_implant_map kim
		JOIN implant i ON i.id_implant = kim.id_implant
		LEFT JOIN implant ht ON ht.id_implant = COALESCE(kim.id_titik_implant, i.id_implant)
		WHERE kim.id_user = $1 AND kim.id_implant = $2`, userID, implantID).Scan(&point, &name)
	if err != nil {
		return implantID, ""
	}
	return point, name
}

// sellerForImplant mencari padanan seller legacy sebuah implant (0 bila tak ada).
func (h *Handler) sellerForImplant(ctx context.Context, implantID int64) int64 {
	var sellerID int64
	_ = h.DB.QueryRow(ctx, `
		SELECT s.id_seller FROM seller s
		JOIN implant i ON i.kode_implant = s.kode_seller
		WHERE i.id_implant = $1`, implantID).Scan(&sellerID)
	return sellerID
}

// batchPending mengambil grup input NULL satu implant dalam 7 hari terakhir.
// Dikelompokkan per tanggal input agar barang yang terlewat beberapa hari tetap
// bisa dikonfirmasi memakai Rit berikutnya.
func (h *Handler) batchPending(ctx context.Context, implantID, sellerID int64) []map[string]interface{} {
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	end := time.Now().In(wib).Format("2006-01-02")
	start := time.Now().In(wib).AddDate(0, 0, -6).Format("2006-01-02")
	return h.batchPendingBetween(ctx, implantID, sellerID, start, end)
}

// batchPendingBetween mengambil grup input NULL pada rentang tanggal WIB.
func (h *Handler) batchPendingBetween(ctx context.Context, implantID, sellerID int64, startDate, endDate string) []map[string]interface{} {
	list := []map[string]interface{}{}
	rows, err := h.DB.Query(ctx, `
		SELECT (created_at + interval '7 hours')::date AS tgl,
		       jenis_ritase, ritase_ke, COUNT(*),
		       COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt + koli_seg + koli_btn), 0),
		       COALESCE(SUM(ecer_jkt + ecer_seg + ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt + koli_hv_seg + koli_hv_btn + ecer_hv_jkt + ecer_hv_seg + ecer_hv_btn + awb_hv_jkt + awb_hv_seg + awb_hv_btn), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0),
		       MAX(created_at)
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date BETWEEN $3::date AND $4::date
		GROUP BY 1, 2, 3
		ORDER BY 1 DESC, 3 ASC
	`, implantID, sellerID, startDate, endDate)
	if err != nil {
		return list
	}
	defer rows.Close()
	for rows.Next() {
		var tgl, lastAt time.Time
		var jenis string
		var ritKe, count int
		var awb, koli, ecer, hv int
		var koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
		var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
		var awbHvJkt, awbHvSeg, awbHvBtn int
		if err := rows.Scan(&tgl, &jenis, &ritKe, &count, &awb, &koli, &ecer, &hv,
			&koliJkt, &koliSeg, &koliBtn, &ecerJkt, &ecerSeg, &ecerBtn,
			&koliHvJkt, &koliHvSeg, &koliHvBtn, &ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
			&awbHvJkt, &awbHvSeg, &awbHvBtn,
			&lastAt); err != nil {
			continue
		}
		list = append(list, map[string]interface{}{
			"jenis_ritase": jenis, "ritase_ke": ritKe, "jumlah_input": count,
			"tanggal_input": tgl.Format("2006-01-02"),
			"created_at":    lastAt.Format(time.RFC3339),
			"total_awb":     awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
			"koli_jkt": koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
			"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
			"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
			"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
			"awb_hv_jkt": awbHvJkt, "awb_hv_seg": awbHvSeg, "awb_hv_btn": awbHvBtn,
		})
	}
	return list
}

// batchTripForGroupDate mengambil trip terakhir sebuah grup pada rentang tanggal
// input. Trip aktif menang atas trip selesai agar grup tidak dianggap terlewat
// saat Rit baru untuk grup yang sama masih berjalan.
func (h *Handler) batchTripForGroupDate(
	ctx context.Context,
	implantID, pointID, sellerID int64,
	jenis string,
	ritKe int,
	tanggal string,
) map[string]interface{} {
	if pointID == 0 {
		pointID = implantID
	}
	if tanggal == "" {
		wib, _ := time.LoadLocation("Asia/Jakarta")
		if wib == nil {
			wib = time.FixedZone("WIB", 7*60*60)
		}
		tanggal = time.Now().In(wib).Format("2006-01-02")
	}
	var idRitase int64
	var kode, status, namaDriver, plat string
	var rKe int
	var rJenis *string
	var tgl time.Time
	var jamMulai, jamSelesai *string
	err := h.DB.QueryRow(ctx, `
		SELECT r.id_ritase, r.kode_ritase, r.status, r.ritase_ke, r.jenis_ritase,
		       r.tanggal, r.jam_mulai, r.jam_selesai,
		       COALESCE(d.nama_driver, ''), COALESCE(k.plat_nomor, '')
		FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
		                   AND (rs.id_implant = $1 OR rs.id_implant = $2 OR rs.id_seller = $3)
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		LEFT JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.jenis_ritase = $4 AND r.ritase_ke = $5
		  AND r.tanggal BETWEEN $6::date - 1 AND $6::date + 1
		ORDER BY (r.status <> 'selesai') DESC, r.tanggal DESC, r.id_ritase DESC
		LIMIT 1
	`, implantID, pointID, sellerID, jenis, ritKe, tanggal).Scan(
		&idRitase, &kode, &status, &rKe, &rJenis, &tgl, &jamMulai, &jamSelesai,
		&namaDriver, &plat,
	)
	if err != nil {
		return nil
	}
	statusGrup := "aktif"
	if status == "selesai" {
		statusGrup = "selesai"
	}
	jamMulaiStr, jamSelesaiStr := "", ""
	if jamMulai != nil {
		jamMulaiStr = *jamMulai
	}
	if jamSelesai != nil {
		jamSelesaiStr = *jamSelesai
	}
	jenisStr := jenis
	if rJenis != nil && *rJenis != "" {
		jenisStr = *rJenis
	}
	return map[string]interface{}{
		"status_grup":    statusGrup,
		"status_ritase":  status,
		"id_ritase":      idRitase,
		"kode_ritase":    kode,
		"jenis_ritase":   jenisStr,
		"ritase_ke":      rKe,
		"tanggal_ritase": tgl.Format("2006-01-02"),
		"jam_mulai":      jamMulaiStr,
		"jam_selesai":    jamSelesaiStr,
		"nama_driver":    namaDriver,
		"plat_nomor":     plat,
	}
}

// batchSisa mengambil sisa per grup satu implant (bentuk sama dengan sisa_grup).
func (h *Handler) batchSisa(ctx context.Context, implantID, sellerID int64) []map[string]interface{} {
	sisaGrup := []map[string]interface{}{}
	sRows, err := h.DB.Query(ctx, `
		WITH inp AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb,
			       COALESCE(SUM(t.awb_hv_jkt), 0) AS w_ahj, COALESCE(SUM(t.awb_hv_seg), 0) AS w_ahs, COALESCE(SUM(t.awb_hv_btn), 0) AS w_ahb
			FROM input_kapten t
			WHERE (t.id_implant = $1 OR t.id_seller = $2)
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			GROUP BY t.jenis_ritase, t.ritase_ke
		),
		amb AS (
			SELECT t.jenis_ritase, t.ritase_ke,
			       COALESCE(SUM(t.jumlah_awb), 0) AS awb,
			       COALESCE(SUM(t.koli_jkt + t.koli_seg + t.koli_btn), 0) AS koli,
			       COALESCE(SUM(t.ecer_jkt + t.ecer_seg + t.ecer_btn), 0) AS ecer,
			       COALESCE(SUM(t.koli_hv_jkt + t.koli_hv_seg + t.koli_hv_btn + t.ecer_hv_jkt + t.ecer_hv_seg + t.ecer_hv_btn + t.awb_hv_jkt + t.awb_hv_seg + t.awb_hv_btn), 0) AS hv,
			       COALESCE(SUM(t.jumlah_awb), 0) AS w_awb,
			       COALESCE(SUM(t.koli_jkt), 0) AS w_kj, COALESCE(SUM(t.koli_seg), 0) AS w_ks, COALESCE(SUM(t.koli_btn), 0) AS w_kb,
			       COALESCE(SUM(t.ecer_jkt), 0) AS w_ej, COALESCE(SUM(t.ecer_seg), 0) AS w_es, COALESCE(SUM(t.ecer_btn), 0) AS w_eb,
			       COALESCE(SUM(t.koli_hv_jkt), 0) AS w_khj, COALESCE(SUM(t.koli_hv_seg), 0) AS w_khs, COALESCE(SUM(t.koli_hv_btn), 0) AS w_khb,
			       COALESCE(SUM(t.ecer_hv_jkt), 0) AS w_ehj, COALESCE(SUM(t.ecer_hv_seg), 0) AS w_ehs, COALESCE(SUM(t.ecer_hv_btn), 0) AS w_ehb,
			       COALESCE(SUM(t.awb_hv_jkt), 0) AS w_ahj, COALESCE(SUM(t.awb_hv_seg), 0) AS w_ahs, COALESCE(SUM(t.awb_hv_btn), 0) AS w_ahb
			FROM konfirmasi_penjemputan t
			WHERE (t.id_implant = $1 OR t.id_seller = $2)
			  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			  AND EXISTS (
				SELECT 1 FROM input_kapten ik
				WHERE ik.id_ritase = t.id_ritase
				  AND ik.jenis_ritase = t.jenis_ritase
				  AND ik.ritase_ke = t.ritase_ke
				  AND (ik.id_implant = t.id_implant OR ik.id_seller = t.id_seller)
				  AND (ik.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			  )
			GROUP BY t.jenis_ritase, t.ritase_ke
		)
		SELECT COALESCE(i.jenis_ritase, a.jenis_ritase, ''),
		       COALESCE(i.ritase_ke, a.ritase_ke, 0),
		       COALESCE(i.awb, 0), COALESCE(i.koli, 0), COALESCE(i.ecer, 0), COALESCE(i.hv, 0),
		       COALESCE(a.awb, 0), COALESCE(a.koli, 0), COALESCE(a.ecer, 0), COALESCE(a.hv, 0),
		       COALESCE(i.w_awb, 0) - COALESCE(a.w_awb, 0),
		       COALESCE(i.w_kj, 0) - COALESCE(a.w_kj, 0),
		       COALESCE(i.w_ks, 0) - COALESCE(a.w_ks, 0),
		       COALESCE(i.w_kb, 0) - COALESCE(a.w_kb, 0),
		       COALESCE(i.w_ej, 0) - COALESCE(a.w_ej, 0),
		       COALESCE(i.w_es, 0) - COALESCE(a.w_es, 0),
		       COALESCE(i.w_eb, 0) - COALESCE(a.w_eb, 0),
		       COALESCE(i.w_khj, 0) - COALESCE(a.w_khj, 0),
		       COALESCE(i.w_khs, 0) - COALESCE(a.w_khs, 0),
		       COALESCE(i.w_khb, 0) - COALESCE(a.w_khb, 0),
		       COALESCE(i.w_ehj, 0) - COALESCE(a.w_ehj, 0),
		       COALESCE(i.w_ehs, 0) - COALESCE(a.w_ehs, 0),
		       COALESCE(i.w_ehb, 0) - COALESCE(a.w_ehb, 0),
		       COALESCE(i.w_ahj, 0) - COALESCE(a.w_ahj, 0),
		       COALESCE(i.w_ahs, 0) - COALESCE(a.w_ahs, 0),
		       COALESCE(i.w_ahb, 0) - COALESCE(a.w_ahb, 0)
		FROM inp i
		FULL OUTER JOIN amb a USING (jenis_ritase, ritase_ke)
		ORDER BY 2
	`, implantID, sellerID)
	if err != nil {
		return sisaGrup
	}
	defer sRows.Close()
	for sRows.Next() {
		var jenis string
		var ritKe int
		var inAwb, inKoli, inEcer, inHV, amAwb, amKoli, amEcer, amHV int
		var sAwb, sKj, sKs, sKb, sEj, sEs, sEb int
		var sKhj, sKhs, sKhb, sEhj, sEhs, sEhb int
		var sAhj, sAhs, sAhb int
		if err := sRows.Scan(&jenis, &ritKe,
			&inAwb, &inKoli, &inEcer, &inHV, &amAwb, &amKoli, &amEcer, &amHV,
			&sAwb, &sKj, &sKs, &sKb, &sEj, &sEs, &sEb,
			&sKhj, &sKhs, &sKhb, &sEhj, &sEhs, &sEhb, &sAhj, &sAhs, &sAhb); err != nil {
			continue
		}
		sisaGrup = append(sisaGrup, map[string]interface{}{
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"input_awb": inAwb, "input_koli": inKoli, "input_ecer": inEcer, "input_hv": inHV,
			"diambil_awb": amAwb, "diambil_koli": amKoli, "diambil_ecer": amEcer, "diambil_hv": amHV,
			"sisa_awb": inAwb - amAwb, "sisa_koli": inKoli - amKoli,
			"sisa_ecer": inEcer - amEcer, "sisa_hv": inHV - amHV,
			"sisa_jumlah_awb": sAwb,
			"sisa_koli_jkt":   sKj, "sisa_koli_seg": sKs, "sisa_koli_btn": sKb,
			"sisa_ecer_jkt": sEj, "sisa_ecer_seg": sEs, "sisa_ecer_btn": sEb,
			"sisa_koli_hv_jkt": sKhj, "sisa_koli_hv_seg": sKhs, "sisa_koli_hv_btn": sKhb,
			"sisa_ecer_hv_jkt": sEhj, "sisa_ecer_hv_seg": sEhs, "sisa_ecer_hv_btn": sEhb,
			"sisa_awb_hv_jkt": sAhj, "sisa_awb_hv_seg": sAhs, "sisa_awb_hv_btn": sAhb,
			"sisa_koli_hv": sKhj + sKhs + sKhb,
			"sisa_hv_awb":  sEhj + sEhs + sEhb,
			"sisa_ecer_hv": sEhj + sEhs + sEhb,
			"sisa_awb_hv":  sAhj + sAhs + sAhb,
		})
	}
	return sisaGrup
}

// GetBatchOverview mengembalikan pending + sisa + riwayat SEMUA implant
// kapten sekaligus â€” bahan layar konfirmasi gabungan satu tap.
// GET /api/v1/kapten/batch-overview
func (h *Handler) GetBatchOverview(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	ids, names, err := h.myImplantIDs(ctx, userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil daftar implant")
	}
	points, pointNames := h.implantPointsForUser(ctx, userID)
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(wib)
	todayStr := now.Format("2006-01-02")
	windows := h.loadRitWindows(ctx)
	intVal := func(v interface{}) int {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			var out int
			fmt.Sscanf(fmt.Sprint(v), "%d", &out)
			return out
		}
	}
	out := []map[string]interface{}{}
	for _, id := range ids {
		sellerID := h.sellerForImplant(ctx, id)
		pointID := points[id]
		if pointID == 0 {
			pointID = id
		}
		pointName := pointNames[pointID]
		if pointName == "" {
			pointName = names[pointID]
		}
		pendingAll := h.batchPending(ctx, id, sellerID)
		sisa := h.batchSisa(ctx, id, sellerID)
		for _, p := range pendingAll {
			p["id_titik_implant"] = pointID
			p["nama_titik_implant"] = pointName
		}
		for _, s := range sisa {
			s["id_titik_implant"] = pointID
			s["nama_titik_implant"] = pointName
		}
		// Riwayat hari ini (ringkas: id, ritase, jumlah, waktu, driver).
		riwayat := []map[string]interface{}{}
		rRows, err := h.DB.Query(ctx, `
			SELECT kp.id, kp.id_ritase, kp.jenis_ritase, kp.ritase_ke,
			       COALESCE(kp.jumlah_awb, 0),
			       COALESCE(kp.koli_jkt, 0) + COALESCE(kp.koli_seg, 0) + COALESCE(kp.koli_btn, 0),
			       COALESCE(kp.ecer_jkt, 0) + COALESCE(kp.ecer_seg, 0) + COALESCE(kp.ecer_btn, 0),
			       COALESCE(kp.koli_hv_jkt, 0) + COALESCE(kp.koli_hv_seg, 0) + COALESCE(kp.koli_hv_btn, 0) +
			           COALESCE(kp.ecer_hv_jkt, 0) + COALESCE(kp.ecer_hv_seg, 0) + COALESCE(kp.ecer_hv_btn, 0) +
			           COALESCE(kp.awb_hv_jkt, 0) + COALESCE(kp.awb_hv_seg, 0) + COALESCE(kp.awb_hv_btn, 0),
			       kp.created_at,
			       COALESCE(r.kode_ritase, ''), COALESCE(d.nama_driver, ''),
			       COALESCE(kp.foto_penjemputan_url, ''), COALESCE(kp.catatan, '')
			FROM konfirmasi_penjemputan kp
			LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
			LEFT JOIN driver d ON d.id_driver = r.id_driver
			WHERE (kp.id_implant = $1 OR kp.id_seller = $2)
			  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			ORDER BY kp.created_at DESC
		`, id, sellerID)
		if err == nil {
			for rRows.Next() {
				var rid, rrit int64
				var rke, awb, koli, ecer, hv int
				var jenis, kode, driver, foto, catatan string
				var created time.Time
				if err := rRows.Scan(&rid, &rrit, &jenis, &rke, &awb, &koli, &ecer, &hv,
					&created, &kode, &driver, &foto, &catatan); err == nil {
					riwayat = append(riwayat, map[string]interface{}{
						"id": rid, "id_ritase": rrit, "jenis_ritase": jenis, "ritase_ke": rke,
						"jumlah_awb": awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
						"created_at":  created.Format(time.RFC3339),
						"kode_ritase": kode, "nama_driver": driver,
						"foto_penjemputan_url": foto, "catatan": catatan,
						"id_implant": id, "nama_implant": names[id],
						"id_titik_implant": pointID, "nama_titik_implant": pointName,
					})
				}
			}
			rRows.Close()
		}

		pendingOut := []map[string]interface{}{}
		normalSeen := map[string]bool{}
		overdueTodayKeys := map[string]bool{}
		ritGroupMap := map[string]map[string]interface{}{}
		addGroupInfo := func(key, jenis string, ritKe int, status string, trip map[string]interface{}) {
			info := map[string]interface{}{
				"jenis_ritase": jenis, "ritase_ke": ritKe, "status_grup": status,
			}
			if trip != nil {
				for k, v := range trip {
					info[k] = v
				}
			}
			if old, ok := ritGroupMap[key]; ok && old["status_grup"] == "aktif" && status != "aktif" {
				return
			}
			ritGroupMap[key] = info
		}
		for _, p := range pendingAll {
			jenis := fmt.Sprint(p["jenis_ritase"])
			ritKe := intVal(p["ritase_ke"])
			tgl := fmt.Sprint(p["tanggal_input"])
			key := fmt.Sprintf("%s|%d", jenis, ritKe)
			if pendingOverdue(p, windows, now) {
				if tgl == todayStr {
					overdueTodayKeys[key] = true
				}
				continue
			}
			trip := h.batchTripForGroupDate(ctx, id, pointID, sellerID, jenis, ritKe, tgl)
			status := "belum_ada_trip"
			if trip != nil {
				status = fmt.Sprint(trip["status_grup"])
			}
			entry := make(map[string]interface{}, len(p)+12)
			for k, v := range p {
				entry[k] = v
			}
			entry["status_grup"] = status
			if trip != nil {
				for k, v := range trip {
					entry[k] = v
				}
			}
			addGroupInfo(key, jenis, ritKe, status, trip)
			// Pending lama tanpa trip tidak ditampilkan sebagai form biasa.
			if status == "belum_ada_trip" && tgl != todayStr {
				continue
			}
			if normalSeen[key] {
				continue
			}
			normalSeen[key] = true
			entry["status"] = "aktif"
			entry["terlambat"] = false
			pendingOut = append(pendingOut, entry)
		}
		// Anotasi sisa grup dengan status trip agar UI tidak menampilkan form
		// untuk grup yang trip asalnya sudah selesai.
		for _, s := range sisa {
			jenis := fmt.Sprint(s["jenis_ritase"])
			ritKe := intVal(s["ritase_ke"])
			key := fmt.Sprintf("%s|%d", jenis, ritKe)
			if overdueTodayKeys[key] {
				continue
			}
			trip := h.batchTripForGroupDate(ctx, id, pointID, sellerID, jenis, ritKe, todayStr)
			status := "belum_ada_trip"
			if trip != nil {
				status = fmt.Sprint(trip["status_grup"])
			}
			s["status_grup"] = status
			addGroupInfo(key, jenis, ritKe, status, trip)
		}
		ritGroups := make([]map[string]interface{}, 0, len(ritGroupMap))
		for _, g := range ritGroupMap {
			ritGroups = append(ritGroups, g)
		}
		sort.Slice(ritGroups, func(i, j int) bool {
			ri, rj := intVal(ritGroups[i]["ritase_ke"]), intVal(ritGroups[j]["ritase_ke"])
			if ri != rj {
				return ri < rj
			}
			return fmt.Sprint(ritGroups[i]["jenis_ritase"]) < fmt.Sprint(ritGroups[j]["jenis_ritase"])
		})
		out = append(out, map[string]interface{}{
			"id_implant": id, "nama_implant": names[id],
			"id_titik_implant": pointID, "nama_titik_implant": pointName,
			"pending": pendingOut, "late_pending": []map[string]interface{}{},
			"sisa_grup": sisa, "riwayat": riwayat, "rit_groups": ritGroups,
		})
	}
	return response.OK(c, map[string]interface{}{"implants": out})
}

// GetBatchRitase mengembalikan ritase hari ini (WIB) yang menyinggahi SEMUA
// implant yang diminta + sudah ada driver & kendaraan â€” kandidat satu driver.
// GET /api/v1/kapten/batch-ritase?implant_ids=1,2,3
func (h *Handler) GetBatchRitase(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	var ids []int64
	pointMode := false
	pointID := int64(0)
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	myIDs, _, _ := h.myImplantIDs(ctx0, userID)
	points, _ := h.implantPointsForUser(ctx0, userID)
	cancel0()
	allowed := map[int64]bool{}
	for _, id := range myIDs {
		allowed[id] = true
	}
	if raw := strings.TrimSpace(c.QueryParam("titik_id")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &pointID); err != nil || pointID <= 0 {
			return response.Error(c, http.StatusBadRequest, "titik_id tidak valid")
		}
		pointMode = true
		if !allowed[pointID] {
			return response.Error(c, http.StatusBadRequest, "titik tidak valid / di luar akses")
		}
		ids = append(ids, pointID)
		for source, host := range points {
			if host == pointID && source != pointID {
				ids = append(ids, source)
			}
		}
	} else {
		for _, part := range strings.Split(c.QueryParam("implant_ids"), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			var id int64
			if _, err := fmt.Sscanf(part, "%d", &id); err != nil || id <= 0 || !allowed[id] {
				return response.Error(c, http.StatusBadRequest, "daftar implant tidak valid / di luar akses")
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return response.Error(c, http.StatusBadRequest, "titik_id atau implant_ids wajib diisi")
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	condition := `(SELECT COUNT(DISTINCT rs.id_implant) FROM ritase_stop rs
		       WHERE rs.id_ritase = r.id_ritase AND rs.id_implant = ANY($1)
		      ) = array_length($1, 1)`
	if pointMode {
		condition = `EXISTS (SELECT 1 FROM ritase_stop rs
		       WHERE rs.id_ritase = r.id_ritase AND rs.id_implant = ANY($1))`
	}
	query := `
		SELECT r.id_ritase, r.kode_ritase, r.ritase_ke, r.status,
		       r.jam_mulai, r.jam_selesai, r.jenis_ritase, r.tanggal,
		       COALESCE(d.nama_driver, 'Driver'), COALESCE(k.plat_nomor, '-')
		FROM ritase r
		JOIN driver d ON d.id_driver = r.id_driver
		JOIN kendaraan k ON k.id_kendaraan = r.id_kendaraan
		WHERE r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
		                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
		  AND r.status != 'selesai'
		  AND ` + condition + `
		ORDER BY r.ritase_ke ASC, r.id_ritase ASC
	`
	rows, err := h.DB.Query(ctx, query, ids)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data ritase")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	seen := map[int64]bool{}
	for rows.Next() {
		var idRitase int64
		var kode, status, namaDriver, plat string
		var ritKe int
		var jamMulai, jamSelesai, jenis *string
		var tanggal time.Time
		if err := rows.Scan(&idRitase, &kode, &ritKe, &status,
			&jamMulai, &jamSelesai, &jenis, &tanggal,
			&namaDriver, &plat); err != nil {
			continue
		}
		if seen[idRitase] {
			continue
		}
		seen[idRitase] = true
		list = append(list, map[string]interface{}{
			"id_ritase": idRitase, "kode_ritase": kode,
			"ritase_ke": ritKe, "status": status,
			"jam_mulai": jamMulai, "jam_selesai": jamSelesai,
			"jenis_ritase": jenis,
			"tanggal":      tanggal.Format("2006-01-02"),
			"nama_driver":  namaDriver, "plat_nomor": plat,
		})
	}
	return response.OK(c, list)
}

// BatchConfirmItem adalah satu titik dalam konfirmasi gabungan.
type BatchConfirmItem struct {
	IDImplant        int64 `json:"id_implant"`
	DiambilAWB       int   `json:"diambil_awb"`
	DiambilKoliJKT   int   `json:"diambil_koli_jkt"`
	DiambilKoliSEG   int   `json:"diambil_koli_seg"`
	DiambilKoliBTN   int   `json:"diambil_koli_btn"`
	DiambilEcerJKT   int   `json:"diambil_ecer_jkt"`
	DiambilEcerSEG   int   `json:"diambil_ecer_seg"`
	DiambilEcerBTN   int   `json:"diambil_ecer_btn"`
	DiambilKoliHVJKT int   `json:"diambil_koli_hv_jkt"`
	DiambilKoliHVSEG int   `json:"diambil_koli_hv_seg"`
	DiambilKoliHVBTN int   `json:"diambil_koli_hv_btn"`
	DiambilEcerHVJKT int   `json:"diambil_ecer_hv_jkt"`
	DiambilEcerHVSEG int   `json:"diambil_ecer_hv_seg"`
	DiambilEcerHVBTN int   `json:"diambil_ecer_hv_btn"`
	DiambilAwbHVJKT  int   `json:"diambil_awb_hv_jkt"`
	DiambilAwbHVSEG  int   `json:"diambil_awb_hv_seg"`
	DiambilAwbHVBTN  int   `json:"diambil_awb_hv_btn"`
	DiambilKoliHV    int   `json:"diambil_koli_hv"`
	DiambilHvAwb     int   `json:"diambil_hv_awb"`
	DiambilEcerHV    int   `json:"diambil_ecer_hv"`
	DiambilAwbHv     int   `json:"diambil_awb_hv"`
}

// BatchConfirmRequest adalah body konfirmasi gabungan: SATU ritase/driver
// untuk BANYAK implant sekaligus (satu tap, satu transaksi atomik).
type BatchConfirmRequest struct {
	JenisRitase        string             `json:"jenis_ritase"`
	RitaseKe           int                `json:"ritase_ke"`
	RitaseKeAsal       int                `json:"ritase_ke_asal"`
	TanggalAsal        string             `json:"tanggal_asal"`
	IDRitase           int64              `json:"id_ritase"`
	FotoPenjemputanURL string             `json:"foto_penjemputan_url"`
	Catatan            string             `json:"catatan"`
	Items              []BatchConfirmItem `json:"items"`
}

// PostBatchConfirmPickup menautkan input NULL semua implant terpilih ke satu
// ritase + mencatat realisasi per implant â€” atomik (semua/sama-sekali).
// POST /api/v1/kapten/confirm-pickup-batch
func (h *Handler) PostBatchConfirmPickup(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	var req BatchConfirmRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if req.RitaseKe < 1 || req.RitaseKe > 4 {
		return response.Error(c, http.StatusBadRequest, "ritase_ke tidak valid")
	}
	if req.RitaseKeAsal == 0 {
		req.RitaseKeAsal = req.RitaseKe
	}
	if req.RitaseKeAsal < 1 || req.RitaseKeAsal != req.RitaseKe {
		return response.Error(c, http.StatusBadRequest, "konfirmasi hanya untuk Rit asal; barang terlambat masuk Riwayat")
	}
	if req.IDRitase <= 0 || len(req.Items) == 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase dan items wajib diisi")
	}
	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(wib)
	if req.TanggalAsal == "" {
		req.TanggalAsal = now.Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", req.TanggalAsal); err != nil {
		return response.Error(c, http.StatusBadRequest, "tanggal_asal tidak valid")
	}
	hariIni := now.Format("2006-01-02")
	kemarin := now.AddDate(0, 0, -1).Format("2006-01-02")
	if req.TanggalAsal != hariIni && req.TanggalAsal != kemarin {
		return response.Error(c, http.StatusBadRequest, "tanggal_asal di luar rentang")
	}

	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	myIDs, _, _ := h.myImplantIDs(ctx0, userID)
	cancel0()
	allowed := map[int64]bool{}
	for _, id := range myIDs {
		allowed[id] = true
	}
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	seenImplant := map[int64]bool{}
	for i := range req.Items {
		it := &req.Items[i]
		if !allowed[it.IDImplant] {
			return response.Error(c, http.StatusBadRequest, "implant di luar akses")
		}
		if seenImplant[it.IDImplant] {
			return response.Error(c, http.StatusBadRequest, "implant duplikat dalam satu konfirmasi")
		}
		seenImplant[it.IDImplant] = true
		it.DiambilAWB = clamp(it.DiambilAWB)
		it.DiambilKoliJKT = clamp(it.DiambilKoliJKT)
		it.DiambilKoliSEG = clamp(it.DiambilKoliSEG)
		it.DiambilKoliBTN = clamp(it.DiambilKoliBTN)
		it.DiambilEcerJKT = clamp(it.DiambilEcerJKT)
		it.DiambilEcerSEG = clamp(it.DiambilEcerSEG)
		it.DiambilEcerBTN = clamp(it.DiambilEcerBTN)
		it.DiambilKoliHVJKT, it.DiambilKoliHVSEG, it.DiambilKoliHVBTN =
			normalisasiHV(it.DiambilKoliHV, 0, it.DiambilKoliHVJKT, it.DiambilKoliHVSEG, it.DiambilKoliHVBTN)
		it.DiambilEcerHVJKT, it.DiambilEcerHVSEG, it.DiambilEcerHVBTN =
			normalisasiHV(it.DiambilHvAwb, it.DiambilEcerHV, it.DiambilEcerHVJKT, it.DiambilEcerHVSEG, it.DiambilEcerHVBTN)
		it.DiambilAwbHVJKT, it.DiambilAwbHVSEG, it.DiambilAwbHVBTN =
			normalisasiHV(it.DiambilAwbHv, 0, it.DiambilAwbHVJKT, it.DiambilAwbHVSEG, it.DiambilAwbHVBTN)
	}

	var fotoURL interface{}
	if req.FotoPenjemputanURL != "" {
		fotoURL = req.FotoPenjemputanURL
	}
	var kaptenUID interface{}
	if userID > 0 {
		kaptenUID = userID
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()
	windows := h.loadRitWindows(ctx)
	if ritDateOverdue(req.TanggalAsal, req.JenisRitase, req.RitaseKe, windows, now) {
		return response.Error(c, http.StatusConflict, "waktu Rit sudah habis; barang terlambat masuk Riwayat")
	}
	if w, ok := windows[ritWindowKey(req.JenisRitase, req.RitaseKe)]; ok && w.endMin <= w.startMin {
		nowMin := now.Hour()*60 + now.Minute()
		if nowMin >= w.endMin && nowMin < w.startMin {
			return response.Error(c, http.StatusConflict, "di luar jam Rit; barang terlambat masuk Riwayat")
		}
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	results := []map[string]interface{}{}
	sellerIDs := map[int64]int64{}
	for _, it := range req.Items {
		// 1. Validasi: ritase ada, belum selesai, dan singgah di titik implant
		//    (fallback: implant sumber untuk rute lama). Satu ritase boleh
		//    menyinggahi beberapa titik dalam satu konfirmasi.
		pointID, _ := h.implantPointForUser(ctx, tx, userID, it.IDImplant)
		if pointID == 0 {
			pointID = it.IDImplant
		}
		var foundID int64
		err = tx.QueryRow(ctx, `
			SELECT r.id_ritase FROM ritase r
			JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
			                   AND (rs.id_implant = $1 OR rs.id_implant = $2)
			WHERE r.id_ritase = $3
			  AND r.status != 'selesai'
			  AND r.jenis_ritase = $4 AND r.ritase_ke = $5
			  AND r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
			                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
		`, it.IDImplant, pointID, req.IDRitase, req.JenisRitase, req.RitaseKe).Scan(&foundID)
		if err != nil {
			return response.Error(c, http.StatusBadRequest, "ritase tidak valid untuk salah satu implant")
		}

		var sellerID int64
		_ = tx.QueryRow(ctx, `
			SELECT s.id_seller FROM seller s
			JOIN implant i ON i.kode_implant = s.kode_seller
			WHERE i.id_implant = $1`, it.IDImplant).Scan(&sellerID)
		sellerIDs[it.IDImplant] = sellerID
		var colSeller interface{}
		if sellerID > 0 {
			colSeller = sellerID
		}

		// 2. Catat serah terima titik ini.
		var idKonfirmasi int64
		err = tx.QueryRow(ctx, `
			INSERT INTO konfirmasi_penjemputan (
				id_ritase, id_seller, id_implant, id_user, jenis_ritase, ritase_ke,
				jumlah_awb,
				koli_jkt, koli_seg, koli_btn,
				ecer_jkt, ecer_seg, ecer_btn,
				koli_hv_jkt, koli_hv_seg, koli_hv_btn,
				ecer_hv_jkt, ecer_hv_seg, ecer_hv_btn,
				awb_hv_jkt, awb_hv_seg, awb_hv_btn,
				foto_penjemputan_url, catatan, id_titik_implant
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, $12, $13, $14, $15,
				$16, $17, $18, $19, $20, $21, $22, $23, $24, $25
			) RETURNING id
		`, req.IDRitase, colSeller, it.IDImplant, kaptenUID, req.JenisRitase, req.RitaseKeAsal,
			it.DiambilAWB,
			it.DiambilKoliJKT, it.DiambilKoliSEG, it.DiambilKoliBTN,
			it.DiambilEcerJKT, it.DiambilEcerSEG, it.DiambilEcerBTN,
			it.DiambilKoliHVJKT, it.DiambilKoliHVSEG, it.DiambilKoliHVBTN,
			it.DiambilEcerHVJKT, it.DiambilEcerHVSEG, it.DiambilEcerHVBTN,
			it.DiambilAwbHVJKT, it.DiambilAwbHVSEG, it.DiambilAwbHVBTN,
			fotoURL, req.Catatan, pointID,
		).Scan(&idKonfirmasi)
		if err != nil {
			log.Printf("[Kapten] gagal insert batch konfirmasi implant %d: %v", it.IDImplant, err)
			return response.Error(c, http.StatusInternalServerError, "gagal menyimpan serah terima")
		}

		// 3. Tautkan input NULL grup asal.
		tag, err := tx.Exec(ctx, `
			UPDATE input_kapten
			SET id_ritase = $1, updated_at = NOW()
			WHERE (id_implant = $2 OR id_seller = $3) AND jenis_ritase = $4 AND ritase_ke = $5
			  AND id_ritase IS NULL
			  AND (created_at + interval '7 hours')::date = $6::date
		`, req.IDRitase, it.IDImplant, sellerID, req.JenisRitase, req.RitaseKeAsal, req.TanggalAsal)
		if err != nil {
			return response.Error(c, http.StatusInternalServerError, "gagal menautkan input")
		}

		results = append(results, map[string]interface{}{
			"id_implant":       it.IDImplant,
			"id_titik_implant": pointID,
			"id_konfirmasi":    idKonfirmasi,
			"total_linked":     tag.RowsAffected(),
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan konfirmasi")
	}

	ctxSisa, cancelSisa := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancelSisa()
	for i := range req.Items {
		id := req.Items[i].IDImplant
		var sisaGroup map[string]interface{}
		for _, s := range h.batchSisa(ctxSisa, id, sellerIDs[id]) {
			if s["jenis_ritase"] == req.JenisRitase && s["ritase_ke"] == req.RitaseKeAsal {
				sisaGroup = s
				break
			}
		}
		if sisaGroup != nil {
			results[i]["sisa"] = sisaGroup
			results[i]["sisa_awb"] = sisaGroup["sisa_awb"]
		}
	}

	h.bus.Publish("force_refresh", "kapten_batch_confirm_pickup")

	return response.OK(c, map[string]interface{}{
		"id_ritase":      req.IDRitase,
		"ritase_ke":      req.RitaseKe,
		"ritase_ke_asal": req.RitaseKeAsal,
		"tanggal_asal":   req.TanggalAsal,
		"items":          results,
	})
}
