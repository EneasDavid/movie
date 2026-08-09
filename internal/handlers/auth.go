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

// Signup serves POST /api/auth/signup — creates the account, then logs
// the user in immediately (same as Login would).
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

	if err := startSession(ctx, w, r, a, user.ID); err != nil {
		log.Printf("signup: startSession failed: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "conta criada, mas houve erro ao iniciar sessão — tente entrar")
		return
	}

	// Email delivery is best-effort: a bounced/slow provider shouldn't
	// block account creation or login. Worst case, the user hits "reenviar
	// verificação" later.
	if err := sendVerificationEmail(ctx, a, r, user); err != nil {
		log.Printf("signup: sendVerificationEmail(%s) failed: %v", user.Email, err)
	}

	writeUser(w, user)
}

// Login serves POST /api/auth/login.
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

	if err := startSession(ctx, w, r, a, user.ID); err != nil {
		log.Printf("login: startSession failed: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "erro ao iniciar sessão")
		return
	}

	writeUser(w, user)
}

// Logout serves POST /api/auth/logout.
func Logout(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	if token, ok := auth.SessionTokenFromRequest(r); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_ = a.Redis.DeleteSession(ctx, token)
	}
	auth.ClearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// Me serves GET /api/auth/me — returns the current user, or 401 if not
// logged in. Used by the frontend on load to know whether to show the
// app or the login screen.
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
// saw, IS the credential. Redirects to the frontend either way so a
// human clicking the link always lands on a page, never a bare JSON blob.
func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	token := r.URL.Query().Get("token")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID, ok := a.Redis.ConsumeVerificationToken(ctx, token)
	if !ok {
		http.Redirect(w, r, "/login?verify=invalid", http.StatusFound)
		return
	}
	if err := a.Redis.MarkEmailVerified(ctx, userID); err != nil {
		log.Printf("verify: MarkEmailVerified(%s) failed: %v", userID, err)
		http.Redirect(w, r, "/login?verify=error", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/?verify=success", http.StatusFound)
}

// ResendVerification serves POST /api/auth/resend-verification.
// Authenticated — resends to whoever's session cookie this is, never to
// an arbitrary address someone else supplies.
func ResendVerification(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()
	userID, _ := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	user, err := a.Redis.GetUserByID(ctx, userID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusUnauthorized, "sessão inválida")
		return
	}
	if user.EmailVerified {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := sendVerificationEmail(ctx, a, r, user); err != nil {
		log.Printf("resend-verification: sendVerificationEmail(%s) failed: %v", user.Email, err)
		httpx.WriteJSONError(w, http.StatusBadGateway, "não foi possível enviar o email agora, tente novamente em instantes")
		return
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
	html := `<p>Bem-vindo(a)! Clique no link abaixo para confirmar seu email:</p>` +
		`<p><a href="` + link + `">` + link + `</a></p>` +
		`<p>Se você não criou essa conta, pode ignorar este email.</p>`

	return a.Mailer.Send(ctx, user.Email, subject, html)
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
