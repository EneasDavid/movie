package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"movie/internal/appctx"
	"movie/internal/drive"
	"movie/internal/httpx"
	"movie/internal/middleware"
)

type progressBody struct {
	Time     float64 `json:"time"`
	Duration float64 `json:"duration"`
	Title    string  `json:"title"`
}

// ListProgress serves GET /api/progress — the signed-in user's
// "continuar assistindo" entries, newest first. Requires RequireAuth.
func ListProgress(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, _ := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	entries, err := a.Redis.ListProgress(ctx, userID)
	if err != nil {
		log.Printf("progress: ListProgress(%s) failed: %v", userID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "não foi possível carregar o progresso")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries})
}

// UpsertProgress serves PUT /api/progress?id=<fileId>. Idempotent: called
// repeatedly with the current playback position from a throttled
// timeupdate handler, each call just overwrites the same record.
func UpsertProgress(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, _ := middleware.UserIDFromContext(r.Context())

	fileID := r.URL.Query().Get("id")
	if !drive.IsValidFileID(fileID) {
		httpx.WriteJSONError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var body progressBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.Time < 0 || body.Duration < 0 {
		httpx.WriteJSONError(w, http.StatusBadRequest, "valores de tempo inválidos")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := a.Redis.SetProgress(ctx, userID, fileID, body.Time, body.Duration, body.Title); err != nil {
		log.Printf("progress: SetProgress(%s,%s) failed: %v", userID, fileID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "não foi possível salvar o progresso")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteProgressEntry serves DELETE /api/progress?id=<fileId> — "remover
// do continuar assistindo".
func DeleteProgressEntry(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, _ := middleware.UserIDFromContext(r.Context())

	fileID := r.URL.Query().Get("id")
	if !drive.IsValidFileID(fileID) {
		httpx.WriteJSONError(w, http.StatusBadRequest, "id inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := a.Redis.DeleteProgress(ctx, userID, fileID); err != nil {
		log.Printf("progress: DeleteProgress(%s,%s) failed: %v", userID, fileID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "não foi possível remover")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
