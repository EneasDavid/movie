package store

import (
	"context"
	"strconv"
	"time"
)

const (
	keyLoginAttemptsPfx = "loginattempts:"
	maxLoginAttempts    = 10
	loginLockoutWindow  = 15 * time.Minute
)

// TooManyLoginAttempts checks (without incrementing) whether an email has
// already hit the failed-attempt ceiling for the current window. Kept
// separate from RecordFailedLogin so a successful login never counts
// against the limit — only failures do.
func (r *Redis) TooManyLoginAttempts(ctx context.Context, email string) bool {
	s, ok := r.GetString(ctx, keyLoginAttemptsPfx+email)
	if !ok {
		return false
	}
	count, err := strconv.Atoi(s)
	return err == nil && count >= maxLoginAttempts
}

// RecordFailedLogin increments the failed-attempt counter for an email,
// arming a 15-minute expiry on the first failure so the count naturally
// resets once the window passes.
func (r *Redis) RecordFailedLogin(ctx context.Context, email string) {
	_, _ = r.IncrWithExpiry(ctx, keyLoginAttemptsPfx+email, loginLockoutWindow)
}

// ClearLoginAttempts resets the counter — called on a successful login so
// a real login doesn't stay one mistyped-password away from lockout.
func (r *Redis) ClearLoginAttempts(ctx context.Context, email string) {
	_, _ = r.command(ctx, "DEL", keyLoginAttemptsPfx+email)
}
