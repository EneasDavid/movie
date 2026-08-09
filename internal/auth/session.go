package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"
)

const (
	CookieName = "session"
	// 30 days: long enough that people don't have to log in constantly on
	// a personal/family catalog, short enough that a stolen cookie has a
	// bounded lifetime.
	SessionTTL = 30 * 24 * time.Hour
)

// NewSessionToken generates a cryptographically random, URL-safe session
// ID. 256 bits of entropy — infeasible to guess, and opaque (carries no
// information itself; the server looks it up in Redis to find the user).
func NewSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// SetSessionCookie writes the session cookie. Secure is conditional on the
// request actually being HTTPS (directly or via Vercel's proxy) so local
// `go run` over plain http://localhost still works — browsers refuse to
// even set a Secure cookie over plain HTTP.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie expires the cookie immediately (logout).
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// SessionTokenFromRequest reads the session cookie, if present.
func SessionTokenFromRequest(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Vercel (and most proxies/CDNs) terminate TLS upstream and forward
	// this header so the origin knows the original scheme was https.
	return r.Header.Get("X-Forwarded-Proto") == "https"
}
