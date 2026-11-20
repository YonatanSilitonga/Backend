package dashboard

import (
	"context"
	"strings"
	"time"
	"unicode"

	"backend/internal/pkg/timeutil"
)

// DilayaniRit adalah rekap AWB per ritase untuk halaman implan/seller dilayani.
type DilayaniRit struct {
	RitaseKe    int   `json:"ritase_ke"`
	AWB         int64 `json:"awb"`
	TotalMuatan int64 `json:"total_muatan"`
}

// DilayaniItem adalah satu implant/seller yang dilayani pada periode.
type DilayaniItem struct {
	ID        int64  `json:"id"`
	Key       string `json:"key,omitempty"`
	Nama      string `json:"nama"`
	Kode      string `json:"kode"`
	Kota      string `json:"kota"`
	AWB       int64  `json:"awb"`
	Kunjungan int64  `json:"kunjungan"`
	Entries   int64  `json:"entries,omitempty"`
}

// DilayaniPage adalah payload halaman Implan/Seller Dilayani.
type DilayaniPage struct {
	Tanggal    string         `json:"tanggal"`
	LatestDate string         `json:"latest_date"`
	TotalAWB   int64          `json:"total_awb"`
	Rits       []DilayaniRit  `json:"rits"`
	Items      []DilayaniItem `json:"items"`
}

// DilayaniDetail adalah ringkasan satu implant/seller sampai tanggal terakhir.
type DilayaniDetail struct {
	ID              int64         `json:"id"`
	Key             string        `json:"key,omitempty"`
	Nama            string        `json:"nama"`
	Kode            string        `json:"kode"`
	Kota            string        `json:"kota"`
	Tanggal         string        `json:"tanggal"`
	TanggalTerakhir string        `json:"tanggal_terakhir"`
	HariTercatat    int64         `json:"hari_tercatat"`
	RataRataHarian  float64       `json:"rata_rata_harian"`
	TotalAWBTanggal int64         `json:"total_awb_tanggal"`
	Rits            []DilayaniRit `json:"rits"`
}

// DropperDilayaniRit adalah rekap AWB dan HV dropper per ritase.
type DropperDilayaniRit struct {
	RitaseKe int   `json:"ritase_ke"`
	AWB      int64 `json:"awb"`
	HV       int64 `json:"hv"`
	Entries  int64 `json:"entries"`
}

// DropperDilayaniItem adalah satu seller dropper pada tanggal laporan.
type DropperDilayaniItem struct {
	ID      int64  `json:"id"`
	Kode    string `json:"kode"`
	Nama    string `json:"nama"`
	AWB     int64  `json:"awb"`
	HV      int64  `json:"hv"`
	Entries int64  `json:"entries"`
}

// DropperDilayaniPage adalah payload laporan muatan dropper.
type DropperDilayaniPage struct {
	Tanggal      string                `json:"tanggal"`
	TotalAWB     int64                 `json:"total_awb"`
	TotalHV      int64                 `json:"total_hv"`
	TotalEntries int64                 `json:"total_entries"`
	Rits         []DropperDilayaniRit  `json:"rits"`
	Items        []DropperDilayaniItem `json:"items"`
}

type muatanRitRow struct {
	RitaseKe   int
	ImplanAWB  int64
	PickupAWB  int64
	DropperAWB int64
}

func (r muatanRitRow) total() int64 {
	return r.ImplanAWB + r.PickupAWB + r.DropperAWB
}

func todayWIB() string {
	return timeutil.NowWIB().Format("2006-01-02")
}

func normalizeDate(tanggal string) string {
	tanggal = strings.TrimSpace(tanggal)
	if tanggal == "" {
		return todayWIB()
	}
	if _, err := time.Parse("2006-01-02", tanggal); err != nil {
		return todayWIB()
	}
	return tanggal
}

func sellerKey(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}

func titleCase(raw string) string {
	raw = strings.TrimFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",.-_!#&", r)
	})
	if raw == "" {
		return raw
	}
	words := strings.Fields(strings.ToLower(raw))
	for i, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

var nonSellerTerms = []string{
	"gudang",
	"hub",
	"transit",
	"perjalanan",
	"menuju",
	"selesai",
	"standby",
	"istirahat",
	"sampai",
	"ke gudang",
	"di gudang",
	"ok",
	"yes",
	"oke",
	"test",
	"testing",
}

func normalizeSellerName(raw string) string {
	clean := strings.TrimFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",.-_!#&", r)
	})
	if clean == "" || len([]rune(clean)) < 3 {
		return ""
	}
	lower := strings.ToLower(clean)

	for _, term := range nonSellerTerms {
		if lower == term || strings.HasPrefix(lower, term+" ") || strings.HasSuffix(lower, " "+term) || strings.Contains(lower, "ke gudang") {
			return ""
		}
	}

	switch {
	case strings.Contains(lower, "pln"):
		return "PLN"
	case strings.Contains(lower, "dewa raya") || lower == "dwr":
		return "DEWA RAYA"
	case strings.Contains(lower, "msc") || strings.Contains(lower, "mcs"):
		return "MSC16"
	case lower == "uce" || strings.Contains(lower, "ucce") || strings.Contains(lower, "ucee"):
		return "UCE"
	case strings.Contains(lower, "kiss") || strings.Contains(lower, "kissa") || strings.Contains(lower, "kuasa studio"):
		return "KISSA STUDIO"
	case strings.Contains(lower, "solution") || strings.Contains(lower, "sulotion") || strings.Contains(lower, "gsol"):
		return "GSOLUTION"
	case strings.Contains(lower, "naimax") || lower == "nmx":
		return "NAIMAX"
	case strings.Contains(lower, "pc") && (strings.Contains(lower, "zen") || strings.Contains(lower, "zein")):
		return "PC & ZEN"
	case strings.Contains(lower, "monza"):
		return "Monza Mall"
	case strings.Contains(lower, "priskila"):
		return "PRISKILA"
	case strings.Contains(lower, "cutties"):
		return "CUTTIES"
	case strings.Contains(lower, "groglo"):
		return "GROGLO"
	case strings.Contains(lower, "plastic") || strings.Contains(lower, "plastik"):
		return "PLASTIC"
	case strings.Contains(lower, "svarga"):
		return "SVARGA"
	case strings.Contains(lower, "shenelin") || strings.Contains(lower, "senelin") || lower == "snln":
		return "SHENELIN"
	case strings.Contains(lower, "fiveplus") || strings.Contains(lower, "fvplus"):
		return "FIVEPLUS"
	case strings.Contains(lower, "thamrin"):
		return "MDP THAMRIN"
	case strings.Contains(lower, "cibodas"):
		return "MDP CIBODAS"
	case strings.Contains(lower, "pempers"):
		return "PEMPERS"
	case strings.Contains(lower, "melinda"):
		return "MELINDA"
	case strings.Contains(lower, "united"):
		return "UNITED CLEANING"
	case strings.Contains(lower, "modern park"):
		return "MODERN PARK"
	case strings.Contains(lower, "home living") || lower == "hmlv":
		return "HOME LIVING"
	}

	return titleCase(clean)
}

func pickupRitase(t time.Time) int {
	hour := t.In(timeutil.WIB()).Hour()
	if hour < 16 {
		return 1
	}
	if hour < 20 {
		return 2
	}
	return 3
}

func (r *Repository) getMuatanRits(ctx context.Context, tanggal string) ([]muatanRitRow, error) {
	rows, err := r.db.Query(ctx, `
		WITH implan AS (
			SELECT t.ritase_ke, COALESCE(SUM(t.jumlah_awb), 0) AS awb
			FROM input_kapten t
			WHERE (t.created_at + interval '7 hours')::date = $1::date
			  AND COALESCE(t.jenis_ritase, 'outgoing') = 'outgoing'
			  AND t.id_gudang IS NULL
			  AND (t.id_implant IS NOT NULL OR t.id_seller IS NOT NULL)
			  AND t.ritase_ke BETWEEN 1 AND 3
			GROUP BY t.ritase_ke
		),
		pickup AS (
			SELECT CASE
			           WHEN COALESCE(t.created_at, t.updated_at) IS NULL THEN 1
			           WHEN EXTRACT(HOUR FROM (COALESCE(t.created_at, t.updated_at) + interval '7 hours')) < 16 THEN 1
			           WHEN EXTRACT(HOUR FROM (COALESCE(t.created_at, t.updated_at) + interval '7 hours')) < 20 THEN 2
			           ELSE 3
			       END AS ritase_ke,
			       COALESCE(SUM(t.jumlah_barang), 0) AS awb
			FROM driver_pickup_log t
			WHERE t.tanggal = $1::date
			GROUP BY 1
		),
		dropper AS (
			SELECT t.ritase_ke, COALESCE(SUM(t.jumlah_awb), 0) AS awb
			FROM dropper_input t
			WHERE t.tanggal = $1::date
			  AND COALESCE(t.jenis_ritase, 'outgoing') = 'outgoing'
			  AND t.ritase_ke BETWEEN 1 AND 3
			GROUP BY t.ritase_ke
		)
		SELECT r.ritase_ke, COALESCE(i.awb, 0), COALESCE(p.awb, 0), COALESCE(d.awb, 0)
		FROM generate_series(1, 3) AS r(ritase_ke)
		LEFT JOIN implan i ON i.ritase_ke = r.ritase_ke
		LEFT JOIN pickup p ON p.ritase_ke = r.ritase_ke
		LEFT JOIN dropper d ON d.ritase_ke = r.ritase_ke
		ORDER BY r.ritase_ke
	`, tanggal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]muatanRitRow, 0, 3)
	for rows.Next() {
		var row muatanRitRow
		if err := rows.Scan(&row.RitaseKe, &row.ImplanAWB, &row.PickupAWB, &row.DropperAWB); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetImplanDilayani mengembalikan rekap implant yang dilayani pada satu tanggal.
func (r *Repository) GetImplanDilayani(ctx context.Context, tanggal string) (*DilayaniPage, error) {
	tanggal = normalizeDate(tanggal)

	muatan, err := r.getMuatanRits(ctx, tanggal)
	if err != nil {
		return nil, err
	}
	muatanByRit := make(map[int]muatanRitRow, len(muatan))
	for _, row := range muatan {
		muatanByRit[row.RitaseKe] = row
	}

	rows, err := r.db.Query(ctx, `
		SELECT i.id_implant,
		       COALESCE(i.kode_implant, ''),
		       COALESCE(i.nama_implant, ''),
		       COALESCE(i.kota, ''),
		       COALESCE(SUM(ik.jumlah_awb), 0) AS awb,
		       COUNT(*) AS entries
		FROM input_kapten ik
		JOIN implant i ON i.id_implant = ik.id_implant
		WHERE (ik.created_at + interval '7 hours')::date = $1::date
		  AND COALESCE(ik.jenis_ritase, 'outgoing') = 'outgoing'
		  AND ik.id_gudang IS NULL
		  AND ik.id_implant IS NOT NULL
		  AND ik.id_implant > 0
		  AND ik.ritase_ke BETWEEN 1 AND 3
		GROUP BY i.id_implant, i.kode_implant, i.nama_implant, i.kota
		ORDER BY awb DESC, i.nama_implant ASC
	`, tanggal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	page := &DilayaniPage{
		Tanggal: tanggal,
		Rits:    make([]DilayaniRit, 0, 3),
		Items:   make([]DilayaniItem, 0),
	}
	for rows.Next() {
		var item DilayaniItem
		if err := rows.Scan(&item.ID, &item.Kode, &item.Nama, &item.Kota, &item.AWB, &item.Entries); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, item)
		page.TotalAWB += item.AWB
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ritRows, err := r.db.Query(ctx, `
		SELECT ik.ritase_ke, COALESCE(SUM(ik.jumlah_awb), 0) AS awb
		FROM input_kapten ik
		WHERE (ik.created_at + interval '7 hours')::date = $1::date
		  AND COALESCE(ik.jenis_ritase, 'outgoing') = 'outgoing'
		  AND ik.id_gudang IS NULL
		  AND ik.id_implant IS NOT NULL
		  AND ik.id_implant > 0
		  AND ik.ritase_ke BETWEEN 1 AND 3
		GROUP BY ik.ritase_ke
	`, tanggal)
	if err != nil {
		return nil, err
	}
	defer ritRows.Close()

	implanByRit := make(map[int]int64, 3)
	for ritRows.Next() {
		var rit int
		var awb int64
		if err := ritRows.Scan(&rit, &awb); err != nil {
			return nil, err
		}
		implanByRit[rit] = awb
	}
	if err := ritRows.Err(); err != nil {
		return nil, err
	}
	for _, row := range muatan {
		page.Rits = append(page.Rits, DilayaniRit{
			RitaseKe:    row.RitaseKe,
			AWB:         implanByRit[row.RitaseKe],
			TotalMuatan: row.total(),
		})
	}

	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX((ik.created_at + interval '7 hours')::date)::text, '')
		FROM input_kapten ik
		WHERE COALESCE(ik.jenis_ritase, 'outgoing') = 'outgoing'
		  AND ik.id_gudang IS NULL
		  AND ik.id_implant IS NOT NULL
		  AND ik.id_implant > 0
		  AND ik.ritase_ke BETWEEN 1 AND 3
	`).Scan(&page.LatestDate); err != nil {
		return nil, err
	}

	return page, nil
}

// GetImplanDilayaniDetail mengembalikan ringkasan satu implant sampai tanggal terakhir.
func (r *Repository) GetImplanDilayaniDetail(ctx context.Context, id int64, tanggal string) (*DilayaniDetail, error) {
	tanggal = normalizeDate(tanggal)

	detail := &DilayaniDetail{ID: id, Tanggal: tanggal, Rits: make([]DilayaniRit, 0, 3)}
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(kode_implant, ''), COALESCE(nama_implant, ''), COALESCE(kota, '')
		FROM implant WHERE id_implant = $1
	`, id).Scan(&detail.Kode, &detail.Nama, &detail.Kota); err != nil {
		return nil, err
	}

	var totalHistory int64
	if err := r.db.QueryRow(ctx, `
		WITH daily AS (
			SELECT (ik.created_at + interval '7 hours')::date AS tgl,
			       COALESCE(SUM(ik.jumlah_awb), 0) AS awb
			FROM input_kapten ik
			WHERE ik.id_implant = $1
			  AND COALESCE(ik.jenis_ritase, 'outgoing') = 'outgoing'
			  AND ik.id_gudang IS NULL
			  AND ik.ritase_ke BETWEEN 1 AND 3
			GROUP BY 1
		)
		SELECT COALESCE(SUM(awb), 0), COUNT(*), COALESCE(MAX(tgl)::text, '')
		FROM daily
	`, id).Scan(&totalHistory, &detail.HariTercatat, &detail.TanggalTerakhir); err != nil {
		return nil, err
	}
	if detail.HariTercatat > 0 {
		detail.RataRataHarian = float64(totalHistory) / float64(detail.HariTercatat)
	}

	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(ik.jumlah_awb), 0)
		FROM input_kapten ik
		WHERE ik.id_implant = $1
		  AND (ik.created_at + interval '7 hours')::date = $2::date
		  AND COALESCE(ik.jenis_ritase, 'outgoing') = 'outgoing'
		  AND ik.id_gudang IS NULL
		  AND ik.ritase_ke BETWEEN 1 AND 3
	`, id, tanggal).Scan(&detail.TotalAWBTanggal); err != nil {
		return nil, err
	}

	muatan, err := r.getMuatanRits(ctx, tanggal)
	if err != nil {
		return nil, err
	}
	muatanByRit := make(map[int]muatanRitRow, len(muatan))
	for _, row := range muatan {
		muatanByRit[row.RitaseKe] = row
	}

	rows, err := r.db.Query(ctx, `
		SELECT ik.ritase_ke, COALESCE(SUM(ik.jumlah_awb), 0)
		FROM input_kapten ik
		WHERE ik.id_implant = $1
		  AND (ik.created_at + interval '7 hours')::date = $2::date
		  AND COALESCE(ik.jenis_ritase, 'outgoing') = 'outgoing'
		  AND ik.id_gudang IS NULL
		  AND ik.ritase_ke BETWEEN 1 AND 3
		GROUP BY ik.ritase_ke
	`, id, tanggal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	awbByRit := make(map[int]int64, 3)
	for rows.Next() {
		var rit int
		var awb int64
		if err := rows.Scan(&rit, &awb); err != nil {
			return nil, err
		}
		awbByRit[rit] = awb
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, row := range muatan {
		detail.Rits = append(detail.Rits, DilayaniRit{
			RitaseKe:    row.RitaseKe,
			AWB:         awbByRit[row.RitaseKe],
			TotalMuatan: row.total(),
		})
	}

	return detail, nil
}

// GetDropperDilayani mengembalikan rekap muatan dropper per seller dan per rit.
func (r *Repository) GetDropperDilayani(ctx context.Context, tanggal string) (*DropperDilayaniPage, error) {
	tanggal = normalizeDate(tanggal)

	page := &DropperDilayaniPage{
		Tanggal: tanggal,
		Rits:    make([]DropperDilayaniRit, 0, 3),
		Items:   make([]DropperDilayaniItem, 0),
	}

	rows, err := r.db.Query(ctx, `
		SELECT d.id_seller,
		       COALESCE(s.kode_seller, ''),
		       COALESCE(NULLIF(s.nama_seller, ''), MAX(d.nama_seller), ''),
		       COALESCE(SUM(d.jumlah_awb), 0) AS awb,
		       COALESCE(SUM(d.jumlah_awb_hv), 0) AS hv,
		       COUNT(*) AS entries
		FROM dropper_input d
		LEFT JOIN seller s ON s.id_seller = d.id_seller
		WHERE d.tanggal = $1::date
		  AND COALESCE(d.jenis_ritase, 'outgoing') = 'outgoing'
		  AND d.ritase_ke BETWEEN 1 AND 3
		GROUP BY d.id_seller, s.kode_seller, s.nama_seller
		ORDER BY awb DESC, entries DESC,
		         COALESCE(NULLIF(s.nama_seller, ''), MAX(d.nama_seller), '') ASC
	`, tanggal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item DropperDilayaniItem
		if err := rows.Scan(&item.ID, &item.Kode, &item.Nama, &item.AWB, &item.HV, &item.Entries); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, item)
		page.TotalAWB += item.AWB
		page.TotalHV += item.HV
		page.TotalEntries += item.Entries
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ritRows, err := r.db.Query(ctx, `
		WITH dropper AS (
			SELECT ritase_ke,
			       COALESCE(SUM(jumlah_awb), 0) AS awb,
			       COALESCE(SUM(jumlah_awb_hv), 0) AS hv,
			       COUNT(*) AS entries
			FROM dropper_input
			WHERE tanggal = $1::date
			  AND COALESCE(jenis_ritase, 'outgoing') = 'outgoing'
			  AND ritase_ke BETWEEN 1 AND 3
			GROUP BY ritase_ke
		)
		SELECT r.ritase_ke, COALESCE(d.awb, 0), COALESCE(d.hv, 0), COALESCE(d.entries, 0)
		FROM generate_series(1, 3) AS r(ritase_ke)
		LEFT JOIN dropper d ON d.ritase_ke = r.ritase_ke
		ORDER BY r.ritase_ke
	`, tanggal)
	if err != nil {
		return nil, err
	}
	defer ritRows.Close()

	for ritRows.Next() {
		var rit DropperDilayaniRit
		if err := ritRows.Scan(&rit.RitaseKe, &rit.AWB, &rit.HV, &rit.Entries); err != nil {
			return nil, err
		}
		page.Rits = append(page.Rits, rit)
	}
	if err := ritRows.Err(); err != nil {
		return nil, err
	}

	return page, nil
}

type pickupLogRow struct {
	IDLog      int64
	IDUser     int64
	NamaDriver string
	PlatNomor  string
	Tanggal    string
	AWB        int64
	Koli       int64
	Ecer       int64
	HighValue  int64
	Status     string
	AsalSeller string
	CreatedAt  time.Time
}

type pickupSession struct {
	SellerKey string
	RawSeller string
	DriverKey string
	Tanggal   string
	RitaseKe  int
	AWB       int64
	Koli      int64
	Ecer      int64
	HighValue int64
	Last      time.Time
}

type sellerMaster struct {
	ID   int64
	Kode string
	Nama string
	Kota string
}

func (r *Repository) loadSellerMaster(ctx context.Context) (map[string]sellerMaster, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id_seller, COALESCE(kode_seller, ''), COALESCE(nama_seller, ''), COALESCE(kota, '')
		FROM seller
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]sellerMaster)
	for rows.Next() {
		var item sellerMaster
		if err := rows.Scan(&item.ID, &item.Kode, &item.Nama, &item.Kota); err != nil {
			return nil, err
		}
		key := sellerKey(normalizeSellerName(item.Nama))
		if key != "" {
			out[key] = item
		}
	}
	return out, rows.Err()
}

func (r *Repository) loadPickupLogs(ctx context.Context, tanggal, from, to, sellerKeyFilter string) ([]pickupLogRow, error) {
	args := make([]any, 0, 3)
	where := make([]string, 0, 3)
	if tanggal != "" {
		args = append(args, tanggal)
		where = append(where, "t.tanggal = $1::date")
	} else {
		args = append(args, from, to)
		where = append(where, "t.tanggal BETWEEN $1::date AND $2::date")
	}
	if sellerKeyFilter != "" {
		args = append(args, sellerKeyFilter)
		where = append(where, "LOWER(TRIM(t.asal_seller)) LIKE '%' || $"+itoa(len(args))+" || '%'")
	}

	query := `
		SELECT COALESCE(t.id_log, 0),
		       COALESCE(t.id_user, 0),
		       COALESCE(t.nama_driver, ''),
		       COALESCE(t.plat_nomor, ''),
		       COALESCE(to_char(t.tanggal, 'YYYY-MM-DD'), ''),
		       COALESCE(t.jumlah_barang, 0),
		       COALESCE(t.koli, 0),
		       COALESCE(t.ecer, 0),
		       COALESCE(t.high_value, 0),
		       COALESCE(t.status, ''),
		       COALESCE(t.asal_seller, ''),
		       COALESCE(t.created_at, t.updated_at, NOW())
		FROM driver_pickup_log t
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY COALESCE(t.created_at, t.updated_at, NOW()) ASC, t.id_log ASC
	`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]pickupLogRow, 0)
	for rows.Next() {
		var row pickupLogRow
		if err := rows.Scan(
			&row.IDLog, &row.IDUser, &row.NamaDriver, &row.PlatNomor, &row.Tanggal,
			&row.AWB, &row.Koli, &row.Ecer, &row.HighValue, &row.Status,
			&row.AsalSeller, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func buildPickupSessions(logs []pickupLogRow) []pickupSession {
	loc := timeutil.WIB()
	sessions := make([]pickupSession, 0, len(logs))
	index := make(map[string]int)
	for _, log := range logs {
		created := log.CreatedAt.In(loc)
		tanggal := strings.TrimSpace(log.Tanggal)
		if tanggal == "" {
			tanggal = created.Format("2006-01-02")
		}
		driverKey := ""
		if log.IDUser > 0 {
			driverKey = "user:" + itoa(int(log.IDUser))
		} else {
			driverKey = sellerKey(log.NamaDriver)
		}
		if driverKey == "" {
			driverKey = "driver"
		}

		sellerName := normalizeSellerName(log.AsalSeller)
		if sellerName == "" {
			continue
		}
		key := sellerKey(sellerName)
		sessionKey := driverKey + "|" + key + "|" + tanggal
		if idx, ok := index[sessionKey]; ok && created.Sub(sessions[idx].Last) <= 30*time.Minute {
			session := &sessions[idx]
			session.Last = created
			session.RitaseKe = pickupRitase(created)
			if log.AWB > session.AWB {
				session.AWB = log.AWB
			}
			if log.Koli > session.Koli {
				session.Koli = log.Koli
			}
			if log.Ecer > session.Ecer {
				session.Ecer = log.Ecer
			}
			if log.HighValue > session.HighValue {
				session.HighValue = log.HighValue
			}
			continue
		}
		sessions = append(sessions, pickupSession{
			SellerKey: key,
			RawSeller: sellerName,
			DriverKey: driverKey,
			Tanggal:   tanggal,
			RitaseKe:  pickupRitase(created),
			AWB:       log.AWB,
			Koli:      log.Koli,
			Ecer:      log.Ecer,
			HighValue: log.HighValue,
			Last:      created,
		})
		index[sessionKey] = len(sessions) - 1
	}
	return sessions
}

func inRange(tanggal, from, to string) bool {
	return tanggal >= from && tanggal <= to
}

// GetSellerDilayani mengembalikan rekap seller yang dilayani (AWB pickup, dedup sesi 30 menit).
func (r *Repository) GetSellerDilayani(ctx context.Context, tanggal, from, to string) (*DilayaniPage, error) {
	page := &DilayaniPage{Rits: make([]DilayaniRit, 0, 3), Items: make([]DilayaniItem, 0)}

	singleDate := strings.TrimSpace(tanggal) != ""
	if singleDate {
		tanggal = normalizeDate(tanggal)
		page.Tanggal = tanggal
	} else {
		from, to = defaultRange(from, to)
		page.Tanggal = from + " s/d " + to
	}

	logs, err := r.loadPickupLogs(ctx, tanggal, from, to, "")
	if err != nil {
		return nil, err
	}
	master, err := r.loadSellerMaster(ctx)
	if err != nil {
		return nil, err
	}
	sessions := buildPickupSessions(logs)

	type agg struct {
		awb       int64
		kunjungan int64
		raw       string
		rits      map[int]int64
	}
	bySeller := make(map[string]*agg)
	for _, session := range sessions {
		if singleDate {
			if session.Tanggal != tanggal {
				continue
			}
		} else if !inRange(session.Tanggal, from, to) {
			continue
		}
		item, ok := bySeller[session.SellerKey]
		if !ok {
			item = &agg{raw: session.RawSeller, rits: make(map[int]int64, 3)}
			bySeller[session.SellerKey] = item
		}
		item.awb += session.AWB
		item.kunjungan++
		item.rits[session.RitaseKe] += session.AWB
	}

	for key, item := range bySeller {
		entry := DilayaniItem{AWB: item.awb, Kunjungan: item.kunjungan}
		if m, ok := master[key]; ok {
			entry.ID = m.ID
			entry.Kode = m.Kode
			entry.Nama = m.Nama
			entry.Kota = m.Kota
		} else {
			entry.Nama = item.raw
		}
		entry.Key = key
		page.Items = append(page.Items, entry)
		page.TotalAWB += entry.AWB
	}
	sortDilayaniItems(page.Items)

	for _, log := range logs {
		if log.Tanggal > page.LatestDate {
			page.LatestDate = log.Tanggal
		}
	}

	if singleDate {
		muatan, err := r.getMuatanRits(ctx, tanggal)
		if err != nil {
			return nil, err
		}
		for _, row := range muatan {
			awb := int64(0)
			for _, session := range sessions {
				if session.Tanggal == tanggal && session.RitaseKe == row.RitaseKe {
					awb += session.AWB
				}
			}
			page.Rits = append(page.Rits, DilayaniRit{
				RitaseKe:    row.RitaseKe,
				AWB:         awb,
				TotalMuatan: row.total(),
			})
		}
	}

	return page, nil
}

// GetSellerDilayaniDetail mengembalikan ringkasan satu seller sampai tanggal terakhir.
func (r *Repository) GetSellerDilayaniDetail(ctx context.Context, key, tanggal string) (*DilayaniDetail, error) {
	key = sellerKey(key)
	tanggal = normalizeDate(tanggal)

	detail := &DilayaniDetail{Key: key, Tanggal: tanggal, Rits: make([]DilayaniRit, 0, 3)}
	logs, err := r.loadPickupLogs(ctx, "", "2000-01-01", todayWIB(), "")
	if err != nil {
		return nil, err
	}
	sessions := buildPickupSessions(logs)

	daily := make(map[string]int64)
	var total int64
	var latest string
	var raw string
	for _, session := range sessions {
		if session.SellerKey != key {
			continue
		}
		if raw == "" {
			raw = session.RawSeller
		}
		daily[session.Tanggal] += session.AWB
		total += session.AWB
		if session.Tanggal > latest {
			latest = session.Tanggal
		}
	}

	detail.Nama = raw
	detail.TanggalTerakhir = latest
	detail.HariTercatat = int64(len(daily))
	if detail.HariTercatat > 0 {
		detail.RataRataHarian = float64(total) / float64(detail.HariTercatat)
	}
	detail.TotalAWBTanggal = daily[tanggal]

	if master, err := r.loadSellerMaster(ctx); err == nil {
		if m, ok := master[key]; ok {
			detail.ID = m.ID
			detail.Kode = m.Kode
			detail.Nama = m.Nama
			detail.Kota = m.Kota
		}
	}

	for _, session := range sessions {
		if session.SellerKey == key && session.Tanggal == tanggal {
			detail.Rits = append(detail.Rits, DilayaniRit{
				RitaseKe: session.RitaseKe,
				AWB:      session.AWB,
			})
		}
	}
	detail.Rits = mergeRits(detail.Rits)

	if muatan, err := r.getMuatanRits(ctx, tanggal); err == nil {
		muatanByRit := make(map[int]muatanRitRow, len(muatan))
		for _, row := range muatan {
			muatanByRit[row.RitaseKe] = row
		}
		for i := range detail.Rits {
			detail.Rits[i].TotalMuatan = muatanByRit[detail.Rits[i].RitaseKe].total()
		}
	}

	return detail, nil
}

func mergeRits(rows []DilayaniRit) []DilayaniRit {
	byRit := make(map[int]int64)
	for _, row := range rows {
		byRit[row.RitaseKe] += row.AWB
	}
	out := make([]DilayaniRit, 0, 3)
	for rit := 1; rit <= 3; rit++ {
		if awb, ok := byRit[rit]; ok {
			out = append(out, DilayaniRit{RitaseKe: rit, AWB: awb})
		}
	}
	return out
}

func sortDilayaniItems(items []DilayaniItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].AWB > items[j-1].AWB; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
