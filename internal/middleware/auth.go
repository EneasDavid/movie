package middleware

import (
	"context"
	"net/http"

	"movie/internal/auth"
	"movie/internal/httpx"
	"movie/internal/store"
)

type ctxKey string

const userIDKey ctxKey = "userID"

// RequireAuth resolves the session cookie to a user ID via Redis and
// rejects the request with 401 if there isn't a valid session. Handlers
// wrapped by this can assume UserIDFromContext always returns ok=true.
func RequireAuth(redis *store.Redis, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := resolveUser(redis, r)
		if !ok {
			httpx.WriteJSONError(w, http.StatusUnauthorized, "não autenticado")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	}
}

// OptionalAuth resolves the session if present but lets the request
// through either way — for endpoints that behave differently when logged
// in without requiring it (none currently, kept for symmetry/future use).
func OptionalAuth(redis *store.Redis, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userID, ok := resolveUser(redis, r); ok {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
		}
		next(w, r)
	}
}

func resolveUser(redis *store.Redis, r *http.Request) (string, bool) {
	token, ok := auth.SessionTokenFromRequest(r)
	if !ok {
		return "", false
	}
	userID, ok := redis.GetSession(r.Context(), token)
	if ok {
		// Sliding expiry: every authenticated request resets the clock,
		// so someone actively using the app never gets logged out
		// mid-session purely from the fixed TTL ticking down.
		redis.TouchSession(r.Context(), token, auth.SessionTTL)
	}
	return userID, ok
}

// UserIDFromContext retrieves the authenticated user's ID, set by
// RequireAuth/OptionalAuth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}
