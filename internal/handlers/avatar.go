package handlers

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"movie/internal/appctx"
	"movie/internal/httpx"
	"movie/internal/middleware"
)

const maxAvatarBytes = 3 << 20 // 3MB — a profile photo, not a video

var allowedAvatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// UploadAvatar serves POST /api/auth/avatar — replaces the caller's own
// profile photo. Raw image bytes in the body (no multipart form: the
// frontend sends the File object directly), Content-Type identifies the
// format. No default-icon fallback lives here — that's a frontend
// concern (an SVG shown whenever HasAvatar is false).
func UploadAvatar(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, _ := middleware.UserIDFromContext(r.Context())

	contentType := r.Header.Get("Content-Type")
	if !allowedAvatarTypes[contentType] {
		httpx.WriteJSONError(w, http.StatusBadRequest, "formato de imagem não suportado (use JPEG, PNG ou WebP)")
		return
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, maxAvatarBytes+1))
	if err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "erro ao ler a imagem")
		return
	}
	if len(data) > maxAvatarBytes {
		httpx.WriteJSONError(w, http.StatusRequestEntityTooLarge, "imagem muito grande (máximo 3MB)")
		return
	}
	if len(data) == 0 {
		httpx.WriteJSONError(w, http.StatusBadRequest, "imagem vazia")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := a.Redis.SetAvatar(ctx, userID, data, contentType); err != nil {
		log.Printf("avatar: upload failed user=%s: %v", userID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "não foi possível salvar a foto")
		return
	}
	log.Printf("avatar: upload ok user=%s bytes=%d type=%s", userID, len(data), contentType)
	w.WriteHeader(http.StatusNoContent)
}

// GetAvatar serves GET /api/auth/avatar — the caller's own photo. 404 if
// none was uploaded; the frontend falls back to the default icon in that
// case rather than treating it as an error.
func GetAvatar(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, _ := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	data, contentType, ok := a.Redis.GetAvatar(ctx, userID)
	if !ok {
		httpx.WriteJSONError(w, http.StatusNotFound, "sem foto de perfil")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
