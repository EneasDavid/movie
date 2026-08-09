package store

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"movie/internal/httpx"
)

// RateLimiter is a fixed-window counter per client IP, backed by Redis so
// the limit holds even though each request may land on a different
// serverless instance (a local in-memory bucket wouldn't survive that).
type RateLimiter struct {
	redis  *Redis
	limit  int
	window time.Duration
}

func NewRateLimiter(redis *Redis, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{redis: redis, limit: limit, window: window}
}

// Allow reports whether the request from remoteAddr may proceed. If Redis
// isn't configured (e.g. running locally without it set up yet), it fails
// open — better to run unthrottled than to hard-fail every request.
func (l *RateLimiter) Allow(ctx context.Context, remoteAddr string) bool {
	if !l.redis.Enabled() {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	// Bucket the window itself into the key so it resets cleanly instead
	// of relying on precise TTL races.
	windowID := time.Now().Unix() / int64(l.window.Seconds())
	key := "rl:" + host + ":" + strconv.FormatInt(windowID, 10)

	count, err := l.redis.IncrWithExpiry(ctx, key, l.window)
	if err != nil {
		// Redis hiccup: don't take the whole app down over rate limiting.
		return true
	}
	return count <= int64(l.limit)
}

// Middleware wraps a handler, rejecting over-limit requests with 429.
func (l *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(r.Context(), httpx.ClientIP(r)) {
			w.Header().Set("Retry-After", "2")
			http.Error(w, "limite de requisições excedido, aguarde um instante", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
