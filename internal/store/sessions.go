package store

import (
	"context"
	"time"
)

const keySessionPfx = "session:"

// CreateSession stores sessionID -> userID with the given TTL.
func (r *Redis) CreateSession(ctx context.Context, sessionID, userID string, ttl time.Duration) error {
	return r.SetString(ctx, keySessionPfx+sessionID, userID, ttl)
}

// GetSession resolves a session token to its userID.
func (r *Redis) GetSession(ctx context.Context, sessionID string) (string, bool) {
	return r.GetString(ctx, keySessionPfx+sessionID)
}

// TouchSession slides the session's expiry forward on activity, so an
// actively-used session doesn't log the user out mid-session while a
// truly abandoned one still expires naturally after the TTL.
func (r *Redis) TouchSession(ctx context.Context, sessionID string, ttl time.Duration) {
	_, _ = r.command(ctx, "EXPIRE", keySessionPfx+sessionID, int(ttl.Seconds()))
}

// DeleteSession revokes a session immediately (logout).
func (r *Redis) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := r.command(ctx, "DEL", keySessionPfx+sessionID)
	return err
}
