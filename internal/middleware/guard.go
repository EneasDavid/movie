// Package middleware holds the cross-cutting request handling shared by
// every /api handler, so each one only has to express its own logic
// instead of repeating security headers, method checks, and rate limiting
// three times over.
package middleware

import (
	"log"
	"net/http"
	"strings"

	"movie/internal/httpx"
	"movie/internal/store"
)

// Guard wraps a handler with the baseline every endpoint needs:
//  1. security headers on every response, success or failure
//  2. reject methods outside the allow-list
//  3. per-IP rate limiting (Redis-backed, works across serverless instances)
//
// Handlers that pass Guard can assume all three are already satisfied.
func Guard(rl *store.RateLimiter, allowedMethods []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.SetSecurityHeaders(w)

		allowed := false
		for _, m := range allowedMethods {
			if r.Method == m {
				allowed = true
				break
			}
		}
		if !allowed {
			w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
			httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "método não permitido")
			return
		}

		if ip := httpx.ClientIP(r); !rl.Allow(r.Context(), ip) {
			log.Printf("ratelimit: blocked ip=%s path=%s method=%s", ip, r.URL.Path, r.Method)
			w.Header().Set("Retry-After", "2")
			httpx.WriteJSONError(w, http.StatusTooManyRequests, "muitas requisições, aguarde um instante")
			return
		}

		next(w, r)
	}
}
