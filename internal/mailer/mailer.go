// Package mailer sends transactional email (currently just the signup
// verification link) behind a small interface, so the real provider
// (Resend) is a drop-in and the app still runs — with the link visible in
// server logs instead of an inbox — before that's configured.
package mailer

import "context"

type Mailer interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}
