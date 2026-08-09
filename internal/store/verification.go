package store

import (
	"context"
	"time"
)

const (
	keyVerifyPfx   = "verify:"
	VerifyTokenTTL = 24 * time.Hour
)

// CreateVerificationToken stores token -> userID with a 24h expiry. The
// token itself (see auth.NewSessionToken, reused for this) is the only
// thing standing between "signed up" and "verified", so it's generated
// with the same crypto/rand-backed randomness as session tokens.
func (r *Redis) CreateVerificationToken(ctx context.Context, token, userID string) error {
	return r.SetString(ctx, keyVerifyPfx+token, userID, VerifyTokenTTL)
}

// ConsumeVerificationToken resolves and immediately deletes a token —
// one-time use, so a leaked/replayed link can't re-trigger anything.
func (r *Redis) ConsumeVerificationToken(ctx context.Context, token string) (string, bool) {
	userID, ok := r.GetString(ctx, keyVerifyPfx+token)
	if !ok {
		return "", false
	}
	_, _ = r.command(ctx, "DEL", keyVerifyPfx+token)
	return userID, true
}
