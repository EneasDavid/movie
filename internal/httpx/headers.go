// Package httpx holds small HTTP helpers shared by every /api function.
package httpx

import (
	"net/http"
	"strings"
)

// ClientIP resolves the real client IP for a request that may have come
// through Vercel's edge proxy. Behind that proxy, r.RemoteAddr is the
// *proxy's* connection to our server — the same handful of addresses for
// every visitor, not the visitor's own IP. Using it for per-client rate
// limiting bucketed unrelated users (and devices) together: a burst of
// requests from one phone could 429 the next person's login attempt from
// a completely different device/network, which is exactly what looked
// like a broken login on mobile. Vercel sets X-Forwarded-For (client IP
// first, then each proxy hop) on every request it forwards, so prefer
// that; fall back to X-Real-Ip, then to RemoteAddr for local dev where
// neither header is present.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-Ip")); xrip != "" {
		return xrip
	}
	return r.RemoteAddr
}

// SetSecurityHeaders applies a conservative baseline to every response:
// no framing by other sites, no MIME sniffing, no referrer leakage, and a
// CSP that only allows same-origin resources (this is a single-user app
// with no third-party scripts, so there's no reason to allow anything
// else). Applied uniformly rather than per-handler so nothing forgets it.
func SetSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
	h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	// Never let intermediaries cache a JSON/API response that might change.
	// Individual handlers (thumbnail, stream) override this with their own
	// appropriate Cache-Control since those ARE meant to be cached.
	h.Set("X-Robots-Tag", "noindex")
}

// WriteJSONError writes a small JSON error body — avoids leaking internal
// error details (stack traces, upstream error text) to the client while
// still giving the frontend something structured to show the user.
func WriteJSONError(w http.ResponseWriter, status int, publicMessage string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + jsonEscape(publicMessage) + `"}`))
}

func jsonEscape(s string) string {
	out := make([]byte, 0, len(s)+8)
	for _, r := range s {
		switch r {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, string(r)...)
		}
	}
	return string(out)
}
