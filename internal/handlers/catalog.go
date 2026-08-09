package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"movie/internal/appctx"
	"movie/internal/httpx"
)

// Catalog serves GET /api/catalog: the full row/category listing built
// from the configured Google Drive folder. Cached as one JSON blob in
// Redis so repeat visits (and the many browser tabs a user might have
// open) don't each re-walk the whole Drive folder tree. Idempotent and
// side-effect free: any number of concurrent callers on a cold cache will
// each rebuild and overwrite the same cache key with equivalent data.
//
// Method/rate-limit/security-header handling lives in the thin route
// wrapper (see cmd/server/main.go), not here — this function is pure
// business logic.
func Catalog(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	if a.Config.DriveAPIKey == "" || a.Config.DriveFolderID == "" {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "servidor não configurado: defina GOOGLE_DRIVE_API_KEY e GOOGLE_DRIVE_FOLDER_ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	var catalog any
	var raw json.RawMessage
	if a.Redis.GetCatalog(ctx, &raw) {
		catalog = raw
	} else {
		built, err := a.Drive.BuildCatalog(ctx, a.Config.DriveFolderID)
		if err != nil {
			log.Printf("catalog: BuildCatalog failed: %v", err)
			httpx.WriteJSONError(w, http.StatusBadGateway,
				"não foi possível carregar o catálogo do Google Drive — verifique se a pasta está compartilhada como 'Qualquer pessoa com o link' e se a API key é válida")
			return
		}
		_ = a.Redis.SetCatalog(ctx, built, a.Config.CatalogTTL)
		catalog = built
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=30")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(catalog)
}
