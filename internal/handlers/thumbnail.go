package handlers

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"movie/internal/appctx"
	"movie/internal/drive"
	"movie/internal/httpx"
)

const maxThumbnailBytes = 5 << 20 // 5MB safety cap

// Thumbnail serves GET /api/thumbnail?id=<fileID>. Drive's thumbnailLink
// URLs are short-lived Google-hosted links, so we resolve+fetch once and
// cache the actual image bytes ourselves — this also means the browser
// never talks to googleusercontent.com directly, keeping every request on
// our own domain (simpler CSP, one cache/rate-limit surface).
//
// Method/rate-limit/security-header handling lives in the thin route
// wrapper (see cmd/server/main.go), not here.
func Thumbnail(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	fileID := r.URL.Query().Get("id")
	if !drive.IsValidFileID(fileID) {
		httpx.WriteJSONError(w, http.StatusBadRequest, "id inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if data, ok := a.Redis.GetThumbnail(ctx, fileID); ok {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=1800, immutable")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	srcURL, err := a.Drive.ThumbnailSourceURL(ctx, fileID)
	if err != nil {
		log.Printf("thumbnail: ThumbnailSourceURL(%s) failed: %v", fileID, err)
		httpx.WriteJSONError(w, http.StatusNotFound, "thumbnail não disponível")
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "erro interno")
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("thumbnail: fetch source failed for %s: err=%v status=%v", fileID, err, respStatus(resp))
		httpx.WriteJSONError(w, http.StatusBadGateway, "erro ao buscar thumbnail")
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxThumbnailBytes))
	if err != nil {
		httpx.WriteJSONError(w, http.StatusBadGateway, "erro ao ler thumbnail")
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	_ = a.Redis.SetThumbnail(ctx, fileID, data, a.Config.ThumbnailTTL)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=1800, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func respStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
