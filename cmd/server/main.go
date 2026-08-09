// Command server runs the exact same handlers Vercel deploys, behind a
// plain net/http server. It exists purely for local development and CI
// smoke tests without needing the Vercel CLI installed — it imports
// internal/handlers directly, so there is no separate implementation to
// keep in sync with production.
package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"movie/internal/appctx"
	"movie/internal/handlers"
	"movie/internal/middleware"
	"movie/internal/web"
)

func main() {
	a := appctx.Get()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", middleware.Guard(a.RateLimiter, []string{http.MethodGet}, handlers.Health))
	mux.HandleFunc("/api/catalog", middleware.Guard(a.RateLimiter, []string{http.MethodGet}, handlers.Catalog))
	mux.HandleFunc("/api/stream", middleware.Guard(a.RateLimiter, []string{http.MethodGet, http.MethodHead}, handlers.Stream))
	mux.HandleFunc("/api/thumbnail", middleware.Guard(a.RateLimiter, []string{http.MethodGet}, handlers.Thumbnail))

	// Static frontend: served from the embedded filesystem (see
	// internal/web), so behavior is identical regardless of the process's
	// working directory, locally or on Vercel.
	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		log.Fatalf("mount embedded static assets: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := ":" + envOr("PORT", "8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("netflix-player local dev server on http://localhost%s", addr)
	log.Printf("drive configured: %v | redis configured: %v", a.Config.DriveAPIKey != "" && a.Config.DriveFolderID != "", a.Redis.Enabled())
	log.Fatal(srv.ListenAndServe())
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
