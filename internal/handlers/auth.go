package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"movie/internal/appctx"
	"movie/internal/auth"
	"movie/internal/httpx"
	"movie/internal/middleware"
	"movie/internal/store"
)

type credentialsBody struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type userResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	EmailVerified bool   `json:"emailVerified"`
	HasAvatar     bool   `json:"hasAvatar"`
}

// pendingVerificationResponse is what Signup and an unverified Login
// attempt both return: no session, just "check your inbox". The frontend
// uses this exact shape to render the "confirme seu email" screen.
type pendingVerificationResponse struct {
	Email               string `json:"email"`
	PendingVerification bool   `json:"pendingVerification"`
}

func writePendingVerification(w http.ResponseWriter, status int, email string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(pendingVerificationResponse{Email: email, PendingVerification: true})
}

// Signup serves POST /api/auth/signup — creates the account and sends the
// confirmation email, but deliberately does NOT start a session. Logging
// in requires a verified email (see Login), so signing up just moves
// straight to "check your inbox" instead of a session that would be
// useless until verification anyway.
func Signup(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	if !a.Redis.Enabled() {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "servidor não configurado: contas exigem Redis (UPSTASH_REDIS_REST_URL/TOKEN)")
		return
	}

	var body credentialsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	email := auth.NormalizeEmail(body.Email)
	firstName := strings.TrimSpace(body.FirstName)
	lastName := strings.TrimSpace(body.LastName)

	if err := auth.ValidateCredentials(email, body.Password); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidateName(firstName, lastName); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		log.Printf("signup: hash password failed: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "erro interno")
		return
	}

	user, err := a.Redis.CreateUser(ctx, email, firstName, lastName, hash)
	if err != nil {
		if errors.Is(err, store.ErrUserExists) {
			httpx.WriteJSONError(w, http.StatusConflict, "já existe uma conta com esse email")
			return
		}
		log.Printf("signup: CreateUser failed: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "não foi possível criar a conta")
		return
	}

	if err := sendVerificationEmail(ctx, a, r, user); err != nil {
		// Unlike login/resend, this one IS worth surfacing: signup just
		// succeeded, so silently failing to send the only way to unlock
		// the account would leave the user stuck with no explanation.
		log.Printf("signup: sendVerificationEmail(%s) failed: %v", user.Email, err)
	} else {
		log.Printf("signup: ok user=%s email=%s verification email sent", user.ID, user.Email)
	}

	writePendingVerification(w, http.StatusCreated, user.Email)
}

// Login serves POST /api/auth/login. Requires a verified email — an
// unverified account gets a fresh confirmation link instead of a session,
// same response shape as Signup so the frontend shows the same screen
// either way.
func Login(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	if !a.Redis.Enabled() {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "servidor não configurado")
		return
	}

	var body credentialsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	email := auth.NormalizeEmail(body.Email)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Per-email lockout: independent of the per-IP rate limit on this
	// route, so a credential-stuffing attempt spread across many IPs
	// still gets stopped once it targets one specific account too hard.
	if a.Redis.TooManyLoginAttempts(ctx, email) {
		httpx.WriteJSONError(w, http.StatusTooManyRequests, "muitas tentativas de login para este email — aguarde alguns minutos")
		return
	}

	// Same generic message whether the email doesn't exist or the password
	// is wrong — distinguishing the two just helps an attacker enumerate
	// registered emails.
	invalidCreds := func() {
		a.Redis.RecordFailedLogin(ctx, email)
		log.Printf("login: failed email=%s reason=invalid_credentials", email)
		httpx.WriteJSONError(w, http.StatusUnauthorized, "email ou senha inválidos")
	}

	user, err := a.Redis.GetUserByEmail(ctx, email)
	if err != nil {
		invalidCreds()
		return
	}
	if !auth.VerifyPassword(user.PasswordHash, body.Password) {
		invalidCreds()
		return
	}
	a.Redis.ClearLoginAttempts(ctx, email)

	if !user.EmailVerified {
		// Correct credentials, just not usable yet — resend the link
		// (best-effort) rather than making the user hunt for a "resend"
		// button on a screen they can't otherwise get past.
		if err := sendVerificationEmail(ctx, a, r, user); err != nil {
			log.Printf("login: blocked user=%s email=%s reason=unverified, resend also failed: %v", user.ID, user.Email, err)
		} else {
			log.Printf("login: blocked user=%s email=%s reason=unverified, verification email resent", user.ID, user.Email)
		}
		writePendingVerification(w, http.StatusForbidden, user.Email)
		return
	}

	if err := startSession(ctx, w, r, a, user.ID); err != nil {
		log.Printf("login: startSession failed for user=%s: %v", user.ID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "erro ao iniciar sessão")
		return
	}

	log.Printf("login: ok user=%s email=%s", user.ID, user.Email)
	writeUser(w, user)
}

// Logout serves POST /api/auth/logout.
func Logout(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	if token, ok := auth.SessionTokenFromRequest(r); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if userID, ok := a.Redis.GetSession(ctx, token); ok {
			log.Printf("logout: ok user=%s", userID)
		}
		_ = a.Redis.DeleteSession(ctx, token)
	}
	auth.ClearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// Me serves GET /api/auth/me — returns the current user, or 401 if not
// logged in. Used by the frontend on load to know whether to show the
// app or the login screen. A valid session here always implies a
// verified email (Login refuses to create one otherwise), so callers
// never need to re-check EmailVerified themselves.
func Me(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteJSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := a.Redis.GetUserByID(ctx, userID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusUnauthorized, "sessão inválida")
		return
	}
	writeUser(w, user)
}

func startSession(ctx context.Context, w http.ResponseWriter, r *http.Request, a *appctx.App, userID string) error {
	token, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	if err := a.Redis.CreateSession(ctx, token, userID, auth.SessionTTL); err != nil {
		return err
	}
	auth.SetSessionCookie(w, r, token)
	return nil
}

// writeUser sends the public-safe user shape — never the password hash.
func writeUser(w http.ResponseWriter, u *store.User) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(userResponse{
		ID:            u.ID,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		EmailVerified: u.EmailVerified,
		HasAvatar:     u.HasAvatar,
	})
}

// VerifyEmail serves GET /api/auth/verify?token=... — the link clicked
// from the confirmation email. Public/unauthenticated by design: the
// token itself, a 256-bit random value only the recipient's inbox ever
// saw, IS the credential. Redirects to /login either way (there's never a
// session at this point — verifying and logging in are separate steps)
// so a human clicking the link always lands on a page, never a bare JSON
// blob.
func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	token := r.URL.Query().Get("token")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID, ok := a.Redis.ConsumeVerificationToken(ctx, token)
	if !ok {
		log.Printf("verify: failed reason=invalid_or_expired_token")
		http.Redirect(w, r, "/login?verify=invalid", http.StatusFound)
		return
	}
	if err := a.Redis.MarkEmailVerified(ctx, userID); err != nil {
		log.Printf("verify: failed user=%s: %v", userID, err)
		http.Redirect(w, r, "/login?verify=error", http.StatusFound)
		return
	}
	log.Printf("verify: ok user=%s", userID)
	http.Redirect(w, r, "/login?verify=success", http.StatusFound)
}

// ResendVerification serves POST /api/auth/resend-verification. Takes an
// email in the body rather than requiring a session — by the time someone
// needs this, they can't log in yet, so there's no session to require.
// Always responds the same way regardless of whether the email exists or
// is already verified, so this can't be used to probe which emails have
// accounts.
func ResendVerification(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	email := auth.NormalizeEmail(body.Email)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if user, err := a.Redis.GetUserByEmail(ctx, email); err == nil && !user.EmailVerified {
		if err := sendVerificationEmail(ctx, a, r, user); err != nil {
			log.Printf("resend-verification: failed user=%s: %v", user.ID, err)
		} else {
			log.Printf("resend-verification: ok user=%s", user.ID)
		}
	} else {
		log.Printf("resend-verification: no-op email=%s (not found or already verified)", email)
	}
	w.WriteHeader(http.StatusNoContent)
}

func sendVerificationEmail(ctx context.Context, a *appctx.App, r *http.Request, user *store.User) error {
	token, err := auth.NewSessionToken() // same 256-bit random token shape as sessions
	if err != nil {
		return err
	}
	if err := a.Redis.CreateVerificationToken(ctx, token, user.ID); err != nil {
		return err
	}

	link := baseURL(a, r) + "/api/auth/verify?token=" + token
	subject := "Confirme seu email — df-orfeu"
	html := `<p>Bem-vindo(a)! Clique no link abaixo para confirmar seu email e poder entrar na sua conta:</p>` +
		`<p><a href="` + link + `">` + link + `</a></p>` +
		`<p>Este link expira em 30 minutos.</p>` +
		`<p>Se você não criou essa conta, pode ignorar este email.</p>`

	return a.Mailer.Send(ctx, user.Email, subject, html)
}

// ForgotPassword serves POST /api/auth/forgot-password. Always responds
// 204 regardless of whether the email exists — the response can't be used
// to enumerate accounts. If it does exist, a reset link (30min TTL,
// one-time use) goes out.
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	email := auth.NormalizeEmail(body.Email)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if user, err := a.Redis.GetUserByEmail(ctx, email); err == nil {
		if err := sendPasswordResetEmail(ctx, a, r, user); err != nil {
			log.Printf("forgot-password: failed user=%s: %v", user.ID, err)
		} else {
			log.Printf("forgot-password: ok user=%s", user.ID)
		}
	} else {
		log.Printf("forgot-password: no-op email=%s (not found)", email)
	}
	w.WriteHeader(http.StatusNoContent)
}

func sendPasswordResetEmail(ctx context.Context, a *appctx.App, r *http.Request, user *store.User) error {
	token, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	if err := a.Redis.CreatePasswordResetToken(ctx, token, user.ID); err != nil {
		return err
	}

	link := baseURL(a, r) + "/reset-password?token=" + token
	subject := "Redefinir senha — df-orfeu"
	html := `<p>Recebemos um pedido para redefinir a senha da sua conta.</p>` +
		`<p><a href="` + link + `">` + link + `</a></p>` +
		`<p>Este link expira em 30 minutos e só pode ser usado uma vez.</p>` +
		`<p>Se você não pediu isso, pode ignorar este email — sua senha continua a mesma.</p>`

	return a.Mailer.Send(ctx, user.Email, subject, html)
}

// ResetPassword serves POST /api/auth/reset-password {token, password}.
// Public/unauthenticated (the token is the credential). Consumes the
// token unconditionally on a successful reset — one-time use.
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID, ok := a.Redis.ConsumePasswordResetToken(ctx, body.Token)
	if !ok {
		log.Printf("reset-password: failed reason=invalid_or_expired_token")
		httpx.WriteJSONError(w, http.StatusBadRequest, "link inválido ou expirado — solicite um novo")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		log.Printf("reset-password: hash password failed for user=%s: %v", userID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "erro interno")
		return
	}
	if err := a.Redis.UpdatePassword(ctx, userID, hash); err != nil {
		log.Printf("reset-password: failed user=%s: %v", userID, err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "não foi possível redefinir a senha")
		return
	}

	log.Printf("reset-password: ok user=%s", userID)
	w.WriteHeader(http.StatusNoContent)
}

// baseURL prefers the explicitly configured PUBLIC_BASE_URL (most
// reliable — no dependency on proxy headers being forwarded correctly),
// falling back to reconstructing it from the request.
func baseURL(a *appctx.App, r *http.Request) string {
	if a.Config.PublicBaseURL != "" {
		return a.Config.PublicBaseURL
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}
