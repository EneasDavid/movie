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
	"path"
	"strings"
	"time"

	"movie/internal/appctx"
	"movie/internal/config"
	"movie/internal/handlers"
	"movie/internal/httpx"
	"movie/internal/middleware"
	"movie/internal/web"
)

const indexFile = "index.html"

func main() {
	config.LoadDotEnv(".env")
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
	mux.Handle("/", spaHandler(staticFS))

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

// spaHandler serves real files from fsys as-is (css/js/assets, each
// content-hashed by Vite so they're safe to cache forever), and falls
// back to index.html for everything else — that's what lets React
// Router's client-side routes (e.g. /watch) work on a hard refresh or
// direct link, since the server has no route matching "/watch" itself;
// only the React app does, once index.html has loaded and taken over.
//
// Deliberately NOT implemented as "rewrite path to /index.html and
// delegate to http.FileServer": that trips FileServer's built-in
// special case where any request ending in /index.html gets redirected
// to "/", which silently drops the query string (?id=...&title=...) the
// player page depends on. Serving the bytes directly avoids that.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	index, err := fs.ReadFile(fsys, indexFile)
	if err != nil {
		log.Fatalf("read embedded %s: %v (did you run the frontend build?)", indexFile, err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.SetSecurityHeaders(w)

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "" {
			cleanPath = indexFile
		}

		if f, err := fsys.Open(cleanPath); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}
