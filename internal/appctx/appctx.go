// Package appctx wires together config, the Drive client, Redis and the
// rate limiter exactly once per warm serverless instance. Each function in
// /api is compiled independently by Vercel, so each gets its own App —
// but within a single warm instance, Go/sync.Once ensures we don't rebuild
// http.Clients or re-parse env vars on every invocation.
package appctx

import (
	"sync"

	"movie/internal/config"
	"movie/internal/drive"
	"movie/internal/store"
)

type App struct {
	Config      config.Config
	Drive       *drive.Client
	Redis       *store.Redis
	RateLimiter *store.RateLimiter
}

var (
	once sync.Once
	app  *App
)

func Get() *App {
	once.Do(func() {
		cfg := config.Load()
		redis := store.NewRedis(cfg.RedisURL, cfg.RedisToken)
		app = &App{
			Config:      cfg,
			Drive:       drive.NewClient(cfg.DriveAPIKey, cfg.DriveMaxInFlight),
			Redis:       redis,
			RateLimiter: store.NewRateLimiter(redis, cfg.RateLimitPerWindow, cfg.RateLimitWindow),
		}
	})
	return app
}
