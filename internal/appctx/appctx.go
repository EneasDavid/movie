// Package appctx wires together config, the Drive client, Redis and the
// rate limiter exactly once per warm serverless instance. Vercel's Go
// Framework Preset runs cmd/server/main.go as one persistent process (not
// one function per file), but a cold start still means a fresh process —
// sync.Once just avoids redoing this setup on every request within the
// same warm instance.
package appctx

import (
	"log"
	"net/mail"
	"sync"
	"time"

	"movie/internal/config"
	"movie/internal/drive"
	"movie/internal/mailer"
	"movie/internal/store"
)

type App struct {
	Config      config.Config
	Drive       *drive.Client
	Redis       *store.Redis
	RateLimiter *store.RateLimiter
	// AuthRateLimiter is deliberately much stricter than RateLimiter —
	// login/signup are the one surface where a permissive limit turns
	// into a brute-force/credential-stuffing tool.
	AuthRateLimiter *store.RateLimiter
	Mailer          mailer.Mailer
}

var (
	once sync.Once
	app  *App
)

func Get() *App {
	once.Do(func() {
		cfg := config.Load()
		redis := store.NewRedis(cfg.RedisURL, cfg.RedisToken)

		var m mailer.Mailer = mailer.LogMailer{}
		if cfg.ResendAPIKey != "" {
			// A malformed EMAIL_FROM (e.g. missing "@" — happened in
			// production: "df-orfeu <naoresponder.ic.ufal.br>") makes
			// every single send fail with a 422 from Resend, which used
			// to only surface once someone tried to sign up or reset a
			// password, logged as a confusing per-request failure. Catch
			// it once, here, at boot, and fall back to just logging
			// emails instead of pretending delivery works.
			if _, err := mail.ParseAddress(cfg.EmailFrom); err != nil {
				log.Printf("appctx: EMAIL_FROM=%q is not a valid address (%v) — falling back to LogMailer, verification/reset emails will only be logged, not sent. Fix the EMAIL_FROM env var (format: \"Name <address@domain>\" or \"address@domain\", and the domain must be verified in Resend, or use the default onboarding@resend.dev for testing) to enable real delivery.", cfg.EmailFrom, err)
			} else {
				m = mailer.NewResendMailer(cfg.ResendAPIKey, cfg.EmailFrom)
			}
		}

		app = &App{
			Config:          cfg,
			Drive:           drive.NewClient(cfg.DriveAPIKey, cfg.DriveMaxInFlight),
			Redis:           redis,
			RateLimiter:     store.NewRateLimiter(redis, cfg.RateLimitPerWindow, cfg.RateLimitWindow),
			AuthRateLimiter: store.NewRateLimiter(redis, 8, 5*time.Minute),
			Mailer:          m,
		}
	})
	return app
}
