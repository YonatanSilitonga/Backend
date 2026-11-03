package mobile_api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/pkg/response"
)


// UploadManifest menangani upload foto bukti manifest dari driver (multipart form)
// POST /api/v1/driver/upload-manifest
func (h *APIHandler) UploadManifest(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "file foto wajib diunggah")
	}

	src, err := file.Open()
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membaca file foto")
	}
	defer src.Close()

	idRitase, _ := strconv.ParseInt(c.FormValue("id_ritase"), 10, 64)
	namaLokasi := c.FormValue("nama_lokasi")

	uploadDir := "./uploads/manifest"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal membuat direktori upload")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".webp"
	}
	fileName := fmt.Sprintf("manifest_r%d_%d%s", idRitase, time.Now().UnixNano()/1e6, ext)
	dstPath := filepath.Join(uploadDir, fileName)

	dst, err := os.Create(dstPath)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menyimpan file foto")
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return response.Error(c, http.StatusInternalServerError, "gagal menulis file foto")
	}

	photoURL := "/uploads/manifest/" + fileName

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	idStop, _ := strconv.ParseInt(c.FormValue("id_stop"), 10, 64)

	// 1. Update ritase_event
	if idRitase > 0 {
		if namaLokasi != "" {
			_, _ = h.DB.Exec(ctx, `
				UPDATE ritase_event
				SET foto_manifest_url = $1
				WHERE id_ritase = $2 AND input_by = 'driver' AND (nama_lokasi = $3 OR POSITION(LOWER($3) in LOWER(nama_lokasi)) > 0 OR POSITION(LOWER(nama_lokasi) in LOWER($3)) > 0)
			`, photoURL, idRitase, namaLokasi)
		} else {
			_, _ = h.DB.Exec(ctx, `
				UPDATE ritase_event
				SET foto_manifest_url = $1
				WHERE id_event = (
					SELECT id_event FROM ritase_event
					WHERE id_ritase = $2
					ORDER BY created_at DESC, id_event DESC
					LIMIT 1
				)
			`, photoURL, idRitase)
		}
	}

	// 2. Update ritase_stop
	if idStop > 0 {
		_, _ = h.DB.Exec(ctx, `UPDATE ritase_stop SET foto_manifest_url = $1 WHERE id_stop = $2`, photoURL, idStop)
	} else if idRitase > 0 && namaLokasi != "" {
		_, _ = h.DB.Exec(ctx, `
			UPDATE ritase_stop
			SET foto_manifest_url = $1
			WHERE id_stop = (
				SELECT rs.id_stop
				FROM ritase_stop rs
				LEFT JOIN seller s ON s.id_seller = rs.id_seller
				LEFT JOIN implant i ON i.id_implant = rs.id_implant
				LEFT JOIN drop_point dp ON dp.id_drop_point = rs.id_drop_point
				LEFT JOIN gudang g ON g.id_gudang = rs.id_gudang
				WHERE rs.id_ritase = $2
				  AND (
				    COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang) = $3
				    OR POSITION(LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, '')) in LOWER($3)) > 0
				    OR POSITION(LOWER($3) in LOWER(COALESCE(i.nama_implant, s.nama_seller, dp.nama_drop_point, g.nama_gudang, ''))) > 0
				  )
				LIMIT 1
			)
		`, photoURL, idRitase, namaLokasi)
	}

	return response.OK(c, map[string]interface{}{
		"message":   "Foto manifest berhasil diunggah",
		"photo_url": photoURL,
	})
}

