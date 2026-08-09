package store

import (
	"context"
	"time"
)

const (
	keyVerifyPfx = "verify:"
	keyResetPfx  = "reset:"

	// EmailLinkTTL applies to both the signup-verification link and the
	// password-reset link — 30 minutes is long enough to get to an inbox
	// and back, short enough that a leaked/old email stops being useful
	// quickly. Both tokens are one-time use regardless (consumed on
	// first click), so this is a ceiling, not the expected lifetime.
	EmailLinkTTL = 30 * time.Minute
)

// CreateVerificationToken stores token -> userID for the signup
// confirmation link. The token itself (see auth.NewSessionToken, reused
// for this) is the only thing standing between "signed up" and
// "verified", so it's generated with the same crypto/rand-backed
// randomness as session tokens.
func (r *Redis) CreateVerificationToken(ctx context.Context, token, userID string) error {
	return r.SetString(ctx, keyVerifyPfx+token, userID, EmailLinkTTL)
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

// CreatePasswordResetToken stores token -> userID for a "forgot password"
// link. Kept under a distinct key prefix from verification tokens so one
// can never be replayed as the other even though both are random strings
// of the same shape.
func (r *Redis) CreatePasswordResetToken(ctx context.Context, token, userID string) error {
	return r.SetString(ctx, keyResetPfx+token, userID, EmailLinkTTL)
}

// ConsumePasswordResetToken resolves and immediately deletes a reset
// token — one-time use, same reasoning as verification tokens.
func (r *Redis) ConsumePasswordResetToken(ctx context.Context, token string) (string, bool) {
	userID, ok := r.GetString(ctx, keyResetPfx+token)
	if !ok {
		return "", false
	}
	_, _ = r.command(ctx, "DEL", keyResetPfx+token)
	return userID, true
}
