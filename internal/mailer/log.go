package mailer

import (
	"context"
	"log"
)

// LogMailer "sends" an email by writing it to the server log instead of
// an inbox. This is the default when no real provider is configured
// (RESEND_API_KEY unset) — the signup flow still works end-to-end for
// testing: the verification link is right there in the Vercel/local logs.
type LogMailer struct{}

func (LogMailer) Send(_ context.Context, to, subject, htmlBody string) error {
	log.Printf("mailer: [SEM PROVEDOR CONFIGURADO] email para %s | assunto: %s | corpo: %s", to, subject, htmlBody)
	return nil
}
