// Package config centralizes all environment-driven settings for the server.
// Keeping every tunable in one place makes it obvious what can be adjusted
// without touching code (cache sizes, rate limits, TTLs, etc).
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string

	// Google Drive
	DriveAPIKey   string
	DriveFolderID string
	// Optional local media source for development/testing when Drive is
	// temporarily rate-limited. Never configured in production.
	LocalMediaFile string

	// Upstash Redis (REST API — works over plain HTTPS, no persistent TCP
	// connection needed, which is what makes it viable from stateless
	// Vercel serverless functions). Falls back to KV_REST_API_* names,
	// which is what Vercel's older "KV" integration sets.
	RedisURL   string
	RedisToken string

	// Catalog cache: how long the folder listing is kept before we
	// re-query Drive. Keeps the home page snappy and avoids hammering the
	// Drive API quota on every visit.
	CatalogTTL time.Duration

	// Thumbnail cache TTL (bytes stored in Redis, base64-encoded).
	ThumbnailTTL time.Duration

	// Video chunk cache: Redis-backed, used by the streaming proxy so
	// repeated plays/seeks don't re-fetch the same bytes from Drive.
	// Chunks are intentionally small (see ChunkSize) to stay well within
	// Upstash's per-value size limits and keep REST round-trips fast.
	ChunkSize int64 // size of each cached chunk, bytes
	ChunkTTL  time.Duration

	// Rate limiting (protects both our server and the Drive API quota).
	// Implemented as a Redis fixed-window counter so it's consistent
	// across serverless invocations/instances.
	RateLimitPerWindow int           // max requests per window, per client IP
	RateLimitWindow    time.Duration // window size

	DriveMaxInFlight int // max concurrent upstream requests to Drive per warm instance

	// Transactional email (signup verification link). Falls back to
	// logging the email instead of sending it when RESEND_API_KEY is
	// unset — see internal/mailer.
	ResendAPIKey  string
	EmailFrom     string
	PublicBaseURL string // absolute origin used to build links in emails, e.g. https://movie.vercel.app
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		DriveAPIKey:    getEnv("GOOGLE_DRIVE_API_KEY", ""),
		DriveFolderID:  getEnv("GOOGLE_DRIVE_FOLDER_ID", ""),
		LocalMediaFile: getEnv("LOCAL_MEDIA_FILE", ""),

		RedisURL:   firstNonEmpty(getEnv("UPSTASH_REDIS_REST_URL", ""), getEnv("KV_REST_API_URL", "")),
		RedisToken: firstNonEmpty(getEnv("UPSTASH_REDIS_REST_TOKEN", ""), getEnv("KV_REST_API_TOKEN", "")),

		CatalogTTL: getDuration("CATALOG_TTL", 5*time.Minute),

		ThumbnailTTL: getDuration("THUMBNAIL_TTL", 30*time.Minute),

		ChunkSize: getInt64("CHUNK_SIZE_BYTES", 1*1024*1024), // 1MB: fast over REST, few round-trips
		ChunkTTL:  getDuration("CHUNK_TTL", 20*time.Minute),

		RateLimitPerWindow: getInt("RATE_LIMIT_PER_WINDOW", 60),
		RateLimitWindow:    getDuration("RATE_LIMIT_WINDOW", 10*time.Second),

		DriveMaxInFlight: getInt("DRIVE_MAX_INFLIGHT", 8),

		ResendAPIKey:  getEnv("RESEND_API_KEY", ""),
		EmailFrom:     getEnv("EMAIL_FROM", "df-orfeu <df-orfeu@naoresponder.ic.ufal.br>"),
		PublicBaseURL: getEnv("PUBLIC_BASE_URL", ""),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
