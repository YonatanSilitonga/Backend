package kapten

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	appMiddleware "backend/internal/pkg/middleware"
	"backend/internal/pkg/response"
)

type ConfirmPickupRequest struct {
	JenisRitase string `json:"jenis_ritase"` // 'outgoing' (default) atau 'incoming'
	RitaseKe    int    `json:"ritase_ke"`    // 1, 2, atau 3
	IDRitase    int64  `json:"id_ritase"`    // trip yang dipilih kapten

	DiambilAWB       int `json:"diambil_awb"`
	DiambilKoliJKT   int `json:"diambil_koli_jkt"`
	DiambilKoliSEG   int `json:"diambil_koli_seg"`
	DiambilKoliBTN   int `json:"diambil_koli_btn"`
	DiambilEcerJKT   int `json:"diambil_ecer_jkt"`
	DiambilEcerSEG   int `json:"diambil_ecer_seg"`
	DiambilEcerBTN   int `json:"diambil_ecer_btn"`
	DiambilKoliHVJKT int `json:"diambil_koli_hv_jkt"`
	DiambilKoliHVSEG int `json:"diambil_koli_hv_seg"`
	DiambilKoliHVBTN int `json:"diambil_koli_hv_btn"`
	DiambilEcerHVJKT int `json:"diambil_ecer_hv_jkt"`
	DiambilEcerHVSEG int `json:"diambil_ecer_hv_seg"`
	DiambilEcerHVBTN int `json:"diambil_ecer_hv_btn"`
	DiambilAwbHVJKT  int `json:"diambil_awb_hv_jkt"`
	DiambilAwbHVSEG  int `json:"diambil_awb_hv_seg"`
	DiambilAwbHVBTN  int `json:"diambil_awb_hv_btn"`
	// Alias lama (Opsi A, gabungan): menang hanya bila ketiga region masih 0.
	DiambilKoliHV int `json:"diambil_koli_hv"`
	DiambilHvAwb  int `json:"diambil_hv_awb"`
	DiambilEcerHV int `json:"diambil_ecer_hv"`
	DiambilAwbHv  int `json:"diambil_awb_hv"`

	FotoPenjemputanURL string `json:"foto_penjemputan_url"` // opsional
	Catatan            string `json:"catatan"`              // opsional (mis. barang sisa)
}

// ConfirmPickup menautkan input kapten yang belum ter-link ke ritase pilihan kapten
// dan mencatat realisasi pengambilan (jumlah + foto + catatan).
// POST /api/v1/kapten/confirm-pickup
func (h *Handler) ConfirmPickup(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	var req ConfirmPickupRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "format request tidak valid")
	}
	if req.JenisRitase == "" {
		req.JenisRitase = "outgoing"
	}
	if req.RitaseKe < 1 || req.RitaseKe > 4 {
		return response.Error(c, http.StatusBadRequest, "ritase_ke tidak valid")
	}
	if req.IDRitase <= 0 {
		return response.Error(c, http.StatusBadRequest, "id_ritase wajib dipilih")
	}
	// Pertahanan: angka negatif dijepit ke 0.
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	req.DiambilAWB = clamp(req.DiambilAWB)
	req.DiambilKoliJKT = clamp(req.DiambilKoliJKT)
	req.DiambilKoliSEG = clamp(req.DiambilKoliSEG)
	req.DiambilKoliBTN = clamp(req.DiambilKoliBTN)
	req.DiambilEcerJKT = clamp(req.DiambilEcerJKT)
	req.DiambilEcerSEG = clamp(req.DiambilEcerSEG)
	req.DiambilEcerBTN = clamp(req.DiambilEcerBTN)
	req.DiambilKoliHVJKT = clamp(req.DiambilKoliHVJKT)
	req.DiambilKoliHVSEG = clamp(req.DiambilKoliHVSEG)
	req.DiambilKoliHVBTN = clamp(req.DiambilKoliHVBTN)
	req.DiambilEcerHVJKT = clamp(req.DiambilEcerHVJKT)
	req.DiambilEcerHVSEG = clamp(req.DiambilEcerHVSEG)
	req.DiambilEcerHVBTN = clamp(req.DiambilEcerHVBTN)
	req.DiambilAwbHVJKT = clamp(req.DiambilAwbHVJKT)
	req.DiambilAwbHVSEG = clamp(req.DiambilAwbHVSEG)
	req.DiambilAwbHVBTN = clamp(req.DiambilAwbHVBTN)
	// Alias lama menang hanya bila ketiga region masih 0 (kompatibilitas app lama).
	req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN =
		normalisasiHV(req.DiambilKoliHV, 0, req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN)
	req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN =
		normalisasiHV(req.DiambilHvAwb, req.DiambilEcerHV, req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN)
	req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN =
		normalisasiHV(req.DiambilAwbHv, 0, req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal memulai transaksi: "+err.Error())
	}
	defer tx.Rollback(ctx)

	uid, _ := c.Get(appMiddleware.CtxUserID).(int64)
	pointID := implantID
	if uid > 0 && implantID > 0 {
		pointID, _ = h.implantPointForUser(ctx, tx, uid, implantID)
	}

	// 1. Validasi: ritase harus ada, belum selesai, singgah di lokasi kapten,
	//    dan jenis + ritase_ke-nya sama dengan grup yang dikonfirmasi.
	var foundID int64
	err = tx.QueryRow(ctx, `
		SELECT r.id_ritase FROM ritase r
		JOIN ritase_stop rs ON rs.id_ritase = r.id_ritase
		                   AND (rs.id_implant = $1 OR rs.id_implant = $2 OR rs.id_seller = $3)
		WHERE r.id_ritase = $4
		  AND r.status != 'selesai'
		  AND r.jenis_ritase = $5 AND r.ritase_ke = $6
		  AND r.tanggal BETWEEN ((now() AT TIME ZONE 'Asia/Jakarta')::date - 1)
		                    AND ((now() AT TIME ZONE 'Asia/Jakarta')::date + 1)
	`, implantID, pointID, sellerID, req.IDRitase, req.JenisRitase, req.RitaseKe).Scan(&foundID)
	if err != nil {
		log.Printf("[Kapten] konfirmasi ditolak implant %d seller %d -> ritase %d: %v", implantID, sellerID, req.IDRitase, err)
		return response.Error(c, http.StatusBadRequest, "ritase tidak valid untuk lokasi ini (bukan jadwal yang singgah ke sini)")
	}

	// 2. Catat kejadian serah terima (realisasi pengambilan).
	var userID interface{}
	if uid > 0 {
		userID = uid
	}
	var colTitik interface{}
	if pointID > 0 {
		colTitik = pointID
	}
	var fotoURL interface{}
	if req.FotoPenjemputanURL != "" {
		fotoURL = req.FotoPenjemputanURL
	}
	var catatan interface{}
	if req.Catatan != "" {
		catatan = req.Catatan
	}
	var idKonfirmasi int64
	var colImplant, colSeller interface{}
	if implantID > 0 {
		colImplant = implantID
	}
	if sellerID > 0 {
		colSeller = sellerID
	}
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
			$7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22,
			$23, $24, $25
		) RETURNING id
	`, req.IDRitase, colSeller, colImplant, userID, req.JenisRitase, req.RitaseKe,
		req.DiambilAWB,
		req.DiambilKoliJKT, req.DiambilKoliSEG, req.DiambilKoliBTN,
		req.DiambilEcerJKT, req.DiambilEcerSEG, req.DiambilEcerBTN,
		req.DiambilKoliHVJKT, req.DiambilKoliHVSEG, req.DiambilKoliHVBTN,
		req.DiambilEcerHVJKT, req.DiambilEcerHVSEG, req.DiambilEcerHVBTN,
		req.DiambilAwbHVJKT, req.DiambilAwbHVSEG, req.DiambilAwbHVBTN,
		fotoURL, catatan, colTitik,
	).Scan(&idKonfirmasi)
	if err != nil {
		log.Printf("[Kapten] gagal insert konfirmasi_penjemputan: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan serah terima")
	}

	// 3. Tautkan semua input hari ini (WIB) yang masih NULL dalam grup ini.
	//    Boleh 0 baris (pengambilan susulan) â€” bukan error.
	tag, err := tx.Exec(ctx, `
		UPDATE input_kapten
		SET id_ritase = $1, updated_at = NOW()
		WHERE (id_implant = $2 OR id_seller = $3) AND jenis_ritase = $4 AND ritase_ke = $5
		  AND id_ritase IS NULL
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, req.IDRitase, implantID, sellerID, req.JenisRitase, req.RitaseKe)
	if err != nil {
		log.Printf("[Kapten] gagal link input saat confirm-pickup: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menautkan input")
	}
	linked := tag.RowsAffected()

	// 4. Hitung sisa grup hari ini (WIB): Î£ input âˆ’ Î£ diambil (akumulasi semua
	//    kejadian hari ini dalam grup, boleh negatif = selisih lebih ambil).
	var inAWB, inKJ, inKS, inKB, inEJ, inES, inEB int
	var inKHJ, inKHS, inKHB, inEHJ, inEHS, inEHB int
	var inAHJ, inAHS, inAHB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0)
		FROM input_kapten
		WHERE (id_implant = $1 OR id_seller = $2) AND jenis_ritase = $3 AND ritase_ke = $4
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
	`, implantID, sellerID, req.JenisRitase, req.RitaseKe).Scan(
		&inAWB, &inKJ, &inKS, &inKB, &inEJ, &inES, &inEB,
		&inKHJ, &inKHS, &inKHB, &inEHJ, &inEHS, &inEHB, &inAHJ, &inAHS, &inAHB)

	var tkAWB, tkKJ, tkKS, tkKB, tkEJ, tkES, tkEB int
	var tkKHJ, tkKHS, tkKHB, tkEHJ, tkEHS, tkEHB int
	var tkAHJ, tkAHS, tkAHB int
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(jumlah_awb), 0),
		       COALESCE(SUM(koli_jkt), 0), COALESCE(SUM(koli_seg), 0), COALESCE(SUM(koli_btn), 0),
		       COALESCE(SUM(ecer_jkt), 0), COALESCE(SUM(ecer_seg), 0), COALESCE(SUM(ecer_btn), 0),
		       COALESCE(SUM(koli_hv_jkt), 0), COALESCE(SUM(koli_hv_seg), 0), COALESCE(SUM(koli_hv_btn), 0),
		       COALESCE(SUM(ecer_hv_jkt), 0), COALESCE(SUM(ecer_hv_seg), 0), COALESCE(SUM(ecer_hv_btn), 0),
		       COALESCE(SUM(awb_hv_jkt), 0), COALESCE(SUM(awb_hv_seg), 0), COALESCE(SUM(awb_hv_btn), 0)
		FROM konfirmasi_penjemputan t
		WHERE (t.id_implant = $1 OR t.id_seller = $2) AND t.jenis_ritase = $3 AND t.ritase_ke = $4
		  AND (t.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		  AND EXISTS (
			SELECT 1 FROM input_kapten ik
			WHERE ik.id_ritase = t.id_ritase
			  AND ik.jenis_ritase = t.jenis_ritase
			  AND ik.ritase_ke = t.ritase_ke
			  AND (ik.id_implant = t.id_implant OR ik.id_seller = t.id_seller)
			  AND (ik.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		  )
	`, implantID, sellerID, req.JenisRitase, req.RitaseKe).Scan(
		&tkAWB, &tkKJ, &tkKS, &tkKB, &tkEJ, &tkES, &tkEB,
		&tkKHJ, &tkKHS, &tkKHB, &tkEHJ, &tkEHS, &tkEHB, &tkAHJ, &tkAHS, &tkAHB)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("[Kapten] gagal commit confirm-pickup: %v", err)
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan konfirmasi")
	}

	sisaAWB := inAWB - tkAWB
	sisaKJ, sisaKS, sisaKB := inKJ-tkKJ, inKS-tkKS, inKB-tkKB
	sisaEJ, sisaES, sisaEB := inEJ-tkEJ, inES-tkES, inEB-tkEB
	sisaKHJ, sisaKHS, sisaKHB := inKHJ-tkKHJ, inKHS-tkKHS, inKHB-tkKHB
	sisaEHJ, sisaEHS, sisaEHB := inEHJ-tkEHJ, inEHS-tkEHS, inEHB-tkEHB
	sisaAHJ, sisaAHS, sisaAHB := inAHJ-tkAHJ, inAHS-tkAHS, inAHB-tkAHB
	log.Printf("[Kapten] seller %d konfirmasi ritase %d (%s rit %d): %d baris ter-link, sisa koli=%d",
		sellerID, req.IDRitase, req.JenisRitase, req.RitaseKe, linked,
		sisaKJ+sisaKS+sisaKB)

	h.bus.Publish("force_refresh", "kapten_confirm_pickup")

	return response.OK(c, map[string]interface{}{
		"id_ritase":     req.IDRitase,
		"total_linked":  linked,
		"id_konfirmasi": idKonfirmasi,
		"sisa": map[string]interface{}{
			"jumlah_awb": sisaAWB,
			"koli_jkt":   sisaKJ, "koli_seg": sisaKS, "koli_btn": sisaKB,
			"ecer_jkt": sisaEJ, "ecer_seg": sisaES, "ecer_btn": sisaEB,
			"koli_hv_jkt": sisaKHJ, "koli_hv_seg": sisaKHS, "koli_hv_btn": sisaKHB,
			"ecer_hv_jkt": sisaEHJ, "ecer_hv_seg": sisaEHS, "ecer_hv_btn": sisaEHB,
			"awb_hv_jkt": sisaAHJ, "awb_hv_seg": sisaAHS, "awb_hv_btn": sisaAHB,
			"koli_hv":    sisaKHJ + sisaKHS + sisaKHB,
			"hv_awb":     sisaEHJ + sisaEHS + sisaEHB,
			"ecer_hv":    sisaEHJ + sisaEHS + sisaEHB,
			"awb_hv":     sisaAHJ + sisaAHS + sisaAHB,
			"total_koli": sisaKJ + sisaKS + sisaKB,
			"total_ecer": sisaEJ + sisaES + sisaEB,
			"total_hv":   sisaKHJ + sisaKHS + sisaKHB + sisaEHJ + sisaEHS + sisaEHB + sisaAHJ + sisaAHS + sisaAHB,
		},
	})
}

// GetPendingConfirmations mengambil grup input hari ini (WIB) milik lokasi kapten
// yang belum ter-link ke ritase â€” bahan layar "Konfirmasi Pengambilan".
// GET /api/v1/kapten/pending-confirmations
func (h *Handler) GetPendingConfirmations(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT jenis_ritase, ritase_ke, COUNT(*),
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
		  AND (created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		GROUP BY jenis_ritase, ritase_ke
		ORDER BY ritase_ke ASC
	`, implantID, sellerID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil pending implant %d seller %d: %v", implantID, sellerID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil data pending")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var jenis string
		var ritKe, count int
		var awb, koli, ecer, hv int
		var koliJkt, koliSeg, koliBtn, ecerJkt, ecerSeg, ecerBtn int
		var koliHvJkt, koliHvSeg, koliHvBtn, ecerHvJkt, ecerHvSeg, ecerHvBtn int
		var awbHvJkt, awbHvSeg, awbHvBtn int
		var lastAt time.Time
		if err := rows.Scan(&jenis, &ritKe, &count, &awb, &koli, &ecer, &hv,
			&koliJkt, &koliSeg, &koliBtn, &ecerJkt, &ecerSeg, &ecerBtn,
			&koliHvJkt, &koliHvSeg, &koliHvBtn, &ecerHvJkt, &ecerHvSeg, &ecerHvBtn,
			&awbHvJkt, &awbHvSeg, &awbHvBtn,
			&lastAt); err != nil {
			log.Printf("[Kapten] scan pending error (seller %d): %v", sellerID, err)
			continue
		}
		list = append(list, map[string]interface{}{
			"jenis_ritase": jenis,
			"ritase_ke":    ritKe,
			"jumlah_input": count,
			"total_awb":    awb,
			"total_koli":   koli,
			"total_ecer":   ecer,
			"total_hv":     hv,
			"koli_jkt":     koliJkt, "koli_seg": koliSeg, "koli_btn": koliBtn,
			"ecer_jkt": ecerJkt, "ecer_seg": ecerSeg, "ecer_btn": ecerBtn,
			"koli_hv_jkt": koliHvJkt, "koli_hv_seg": koliHvSeg, "koli_hv_btn": koliHvBtn,
			"ecer_hv_jkt": ecerHvJkt, "ecer_hv_seg": ecerHvSeg, "ecer_hv_btn": ecerHvBtn,
			"awb_hv_jkt": awbHvJkt, "awb_hv_seg": awbHvSeg, "awb_hv_btn": awbHvBtn,
		})
	}

	return response.OK(c, list)
}

// GetKonfirmasiPenjemputan mengambil riwayat serah terima hari ini (WIB) milik
// lokasi kapten â€” lengkap dengan trip, driver, jumlah diambil, foto, catatan.
// GET /api/v1/kapten/konfirmasi-penjemputan
func (h *Handler) GetKonfirmasiPenjemputan(c echo.Context) error {
	ctx0, cancel0 := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel0()
	implantID, sellerID := h.lokasiKapten(ctx0, c)
	if implantID == 0 && sellerID == 0 {
		return response.Error(c, http.StatusBadRequest, "akun kapten tidak terkait dengan implant/seller")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT kp.id, kp.id_ritase, kp.jenis_ritase, kp.ritase_ke,
		       COALESCE(kp.jumlah_awb, 0),
		       COALESCE(kp.koli_jkt, 0), COALESCE(kp.koli_seg, 0), COALESCE(kp.koli_btn, 0),
		       COALESCE(kp.ecer_jkt, 0), COALESCE(kp.ecer_seg, 0), COALESCE(kp.ecer_btn, 0),
		       COALESCE(kp.koli_hv_jkt, 0), COALESCE(kp.koli_hv_seg, 0), COALESCE(kp.koli_hv_btn, 0),
		       COALESCE(kp.ecer_hv_jkt, 0), COALESCE(kp.ecer_hv_seg, 0), COALESCE(kp.ecer_hv_btn, 0),
		       COALESCE(kp.awb_hv_jkt, 0), COALESCE(kp.awb_hv_seg, 0), COALESCE(kp.awb_hv_btn, 0),
		       COALESCE(kp.foto_penjemputan_url, ''),
		       COALESCE(kp.catatan, ''),
		       kp.created_at,
		       COALESCE(r.kode_ritase, ''),
		       COALESCE(d.nama_driver, '')
		FROM konfirmasi_penjemputan kp
		LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
		LEFT JOIN driver d ON d.id_driver = r.id_driver
		WHERE (kp.id_implant = $1 OR kp.id_seller = $2)
		  AND (kp.created_at + interval '7 hours')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
		ORDER BY kp.created_at DESC
	`, implantID, sellerID)
	if err != nil {
		log.Printf("[Kapten] gagal ambil riwayat penjemputan implant %d seller %d: %v", implantID, sellerID, err)
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil riwayat")
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id, idRitase int64
		var jenis string
		var ritKe int
		var awb, kj, ks, kb, ej, es, eb, khj, khs, khb, ehj, ehs, ehb, ahj, ahs, ahb int
		var foto, catatan, kode, driver string
		var createdAt time.Time
		if err := rows.Scan(&id, &idRitase, &jenis, &ritKe,
			&awb, &kj, &ks, &kb, &ej, &es, &eb, &khj, &khs, &khb, &ehj, &ehs, &ehb, &ahj, &ahs, &ahb,
			&foto, &catatan, &createdAt, &kode, &driver); err != nil {
			log.Printf("[Kapten] scan riwayat penjemputan error (seller %d): %v", sellerID, err)
			continue
		}
		tKoli := kj + ks + kb
		tEcer := ej + es + eb
		tHV := khj + khs + khb + ehj + ehs + ehb + ahj + ahs + ahb
		list = append(list, map[string]interface{}{
			"id": id, "id_ritase": idRitase,
			"jenis_ritase": jenis, "ritase_ke": ritKe,
			"jumlah_awb": awb,
			"koli_jkt":   kj, "koli_seg": ks, "koli_btn": kb,
			"ecer_jkt": ej, "ecer_seg": es, "ecer_btn": eb,
			"koli_hv_jkt": khj, "koli_hv_seg": khs, "koli_hv_btn": khb,
			"ecer_hv_jkt": ehj, "ecer_hv_seg": ehs, "ecer_hv_btn": ehb,
			"awb_hv_jkt": ahj, "awb_hv_seg": ahs, "awb_hv_btn": ahb,
			"awb_hv":     ahj + ahs + ahb,
			"total_koli": tKoli, "total_ecer": tEcer, "total_hv": tHV,
			"foto_penjemputan_url": foto,
			"catatan":              catatan,
			"created_at":           createdAt.Format(time.RFC3339),
			"kode_ritase":          kode,
			"nama_driver":          driver,
		})
	}

	// Sisa per grup hari ini (WIB): Î£ input âˆ’ Î£ diambil (ringkas + per wilayah).
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
	if err == nil {
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
				log.Printf("[Kapten] scan sisa grup error (seller %d): %v", sellerID, err)
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
	} else {
		log.Printf("[Kapten] gagal ambil sisa grup seller %d: %v", sellerID, err)
	}

	return response.OK(c, map[string]interface{}{
		"riwayat":   list,
		"sisa_grup": sisaGrup,
	})
}

// GetKonfirmasiHistory mengambil riwayat konfirmasi seluruh implant kapten:
// konfirmasi tersimpan (diambil/terlambat) + barang terlewat yang dihitung
// dari input pending dan trip asal yang sudah selesai.
// GET /api/v1/kapten/konfirmasi-history
func (h *Handler) GetKonfirmasiHistory(c echo.Context) error {
	userID, _ := c.Get(appMiddleware.CtxUserID).(int64)
	if userID == 0 {
		return response.Error(c, http.StatusUnauthorized, "tidak terautentikasi")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()

	ids, names, err := h.myImplantIDs(ctx, userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal mengambil daftar implant")
	}
	if len(ids) == 0 {
		return response.OK(c, map[string]interface{}{"riwayat": []map[string]interface{}{}})
	}
	points, pointNames := h.implantPointsForUser(ctx, userID)
	allowed := map[int64]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	filterImplant := int64(0)
	if raw := strings.TrimSpace(c.QueryParam("id_implant")); raw != "" {
		fmt.Sscanf(raw, "%d", &filterImplant)
		if filterImplant > 0 && !allowed[filterImplant] {
			return response.Error(c, http.StatusBadRequest, "implant di luar akses")
		}
	}

	wib, _ := time.LoadLocation("Asia/Jakarta")
	if wib == nil {
		wib = time.FixedZone("WIB", 7*60*60)
	}
	today := time.Now().In(wib)
	todayStr := today.Format("2006-01-02")
	windows := h.loadRitWindows(ctx)
	startDate, endDate := "", ""
	filter := strings.ToLower(strings.TrimSpace(c.QueryParam("filter")))
	switch filter {
	case "today":
		startDate, endDate = todayStr, todayStr
	case "week", "7d":
		startDate, endDate = today.AddDate(0, 0, -6).Format("2006-01-02"), todayStr
	case "month":
		startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, wib).Format("2006-01-02")
		endDate = todayStr
	case "all":
		startDate, endDate = "2000-01-01", todayStr
	default:
		startDate = strings.TrimSpace(c.QueryParam("start_date"))
		endDate = strings.TrimSpace(c.QueryParam("end_date"))
		if startDate == "" && endDate == "" {
			startDate, endDate = todayStr, todayStr
		} else if startDate == "" {
			startDate = endDate
		} else if endDate == "" {
			endDate = todayStr
		}
	}
	if startDate != "" {
		if _, err := time.Parse("2006-01-02", startDate); err != nil {
			return response.Error(c, http.StatusBadRequest, "start_date tidak valid")
		}
	}
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err != nil {
			return response.Error(c, http.StatusBadRequest, "end_date tidak valid")
		}
	}

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
	result := []map[string]interface{}{}
	for _, id := range ids {
		if filterImplant > 0 && id != filterImplant {
			continue
		}
		sellerID := h.sellerForImplant(ctx, id)
		pointID := points[id]
		if pointID == 0 {
			pointID = id
		}
		pointName := pointNames[pointID]
		args := []interface{}{id, sellerID}
		where := `(kp.id_implant = $1 OR kp.id_seller = $2)`
		if startDate != "" {
			args = append(args, startDate)
			where += fmt.Sprintf(` AND (kp.created_at + interval '7 hours')::date >= $%d::date`, len(args))
		}
		if endDate != "" {
			args = append(args, endDate)
			where += fmt.Sprintf(` AND (kp.created_at + interval '7 hours')::date <= $%d::date`, len(args))
		}
		rows, qerr := h.DB.Query(ctx, `
			SELECT kp.id, kp.id_ritase, COALESCE(kp.jenis_ritase, ''), kp.ritase_ke,
			       COALESCE(r.ritase_ke, kp.ritase_ke),
			       COALESCE(r.tanggal, (kp.created_at + interval '7 hours')::date),
			       COALESCE(kp.jumlah_awb, 0),
			       COALESCE(kp.koli_jkt, 0) + COALESCE(kp.koli_seg, 0) + COALESCE(kp.koli_btn, 0),
			       COALESCE(kp.ecer_jkt, 0) + COALESCE(kp.ecer_seg, 0) + COALESCE(kp.ecer_btn, 0),
			       COALESCE(kp.koli_hv_jkt, 0) + COALESCE(kp.koli_hv_seg, 0) + COALESCE(kp.koli_hv_btn, 0) +
			           COALESCE(kp.ecer_hv_jkt, 0) + COALESCE(kp.ecer_hv_seg, 0) + COALESCE(kp.ecer_hv_btn, 0) +
			           COALESCE(kp.awb_hv_jkt, 0) + COALESCE(kp.awb_hv_seg, 0) + COALESCE(kp.awb_hv_btn, 0),
			       COALESCE(kp.foto_penjemputan_url, ''), COALESCE(kp.catatan, ''),
			       kp.created_at, COALESCE(r.kode_ritase, ''), COALESCE(d.nama_driver, '')
			FROM konfirmasi_penjemputan kp
			LEFT JOIN ritase r ON r.id_ritase = kp.id_ritase
			LEFT JOIN driver d ON d.id_driver = r.id_driver
			WHERE `+where+`
			ORDER BY kp.created_at DESC
			LIMIT 500
		`, args...)
		if qerr == nil {
			for rows.Next() {
				var kid, kRitase int64
				var kAsal, kAktual, awb, koli, ecer, hv int
				var jenis, foto, catatan, kode, driver string
				var tgl, created time.Time
				if err := rows.Scan(&kid, &kRitase, &jenis, &kAsal, &kAktual, &tgl,
					&awb, &koli, &ecer, &hv, &foto, &catatan, &created, &kode, &driver); err != nil {
					continue
				}
				status := "diambil"
				if kAktual > kAsal {
					status = "terlambat"
				}
				result = append(result, map[string]interface{}{
					"id": kid, "id_ritase": kRitase,
					"jenis_ritase": jenis, "ritase_ke": kAsal, "ritase_ke_aktual": kAktual,
					"status":        status,
					"tanggal_input": tgl.Format("2006-01-02"), "tanggal_aktual": tgl.Format("2006-01-02"),
					"jumlah_awb": awb, "total_koli": koli, "total_ecer": ecer, "total_hv": hv,
					"foto_penjemputan_url": foto, "catatan": catatan,
					"created_at":  created.Format(time.RFC3339),
					"kode_ritase": kode, "nama_driver": driver,
					"id_implant": id, "nama_implant": names[id],
					"id_titik_implant": pointID, "nama_titik_implant": pointName,
				})
			}
			rows.Close()
		}

		pending := h.batchPendingBetween(ctx, id, sellerID, startDate, endDate)
		for _, p := range pending {
			jenis := fmt.Sprint(p["jenis_ritase"])
			ritKe := intVal(p["ritase_ke"])
			tgl := fmt.Sprint(p["tanggal_input"])
			trip := h.batchTripForGroupDate(ctx, id, pointID, sellerID, jenis, ritKe, tgl)
			if !pendingOverdue(p, windows, today) {
				continue
			}
			entry := make(map[string]interface{}, len(p)+16)
			for k, v := range p {
				entry[k] = v
			}
			entry["id"] = 0
			entry["status"] = "terlewat"
			entry["terlambat"] = true
			entry["ritase_ke_aktual"] = 0
			entry["tanggal_aktual"] = ""
			entry["id_ritase"] = int64(0)
			entry["kode_ritase"] = ""
			entry["nama_driver"] = ""
			if trip != nil {
				entry["id_ritase"] = trip["id_ritase"]
				entry["kode_ritase"] = trip["kode_ritase"]
				entry["nama_driver"] = trip["nama_driver"]
			}
			entry["foto_penjemputan_url"] = ""
			entry["catatan"] = ""
			entry["id_implant"] = id
			entry["nama_implant"] = names[id]
			entry["id_titik_implant"] = pointID
			entry["nama_titik_implant"] = pointName
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return fmt.Sprint(result[i]["created_at"]) > fmt.Sprint(result[j]["created_at"])
	})
	if len(result) > 200 {
		result = result[:200]
	}
	return response.OK(c, map[string]interface{}{"riwayat": result})
}

// parseTimeMinutes mengkonversi string jam "HH:MM:SS" atau "HH:MM" ke menit dari midnight.