package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const (
	keyUserByEmailPfx = "user:byemail:"
	keyUserPfx        = "user:"
)

// No TTL on accounts — they persist until explicitly deleted (never, in
// this app). TTL only applies to sessions (see sessions.go).
const noExpiry = 0

type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	PasswordHash  string `json:"passwordHash"`
	EmailVerified bool   `json:"emailVerified"`
	HasAvatar     bool   `json:"hasAvatar"`
	CreatedAt     string `json:"createdAt"`
}

var ErrUserExists = errors.New("já existe uma conta com esse email")
var ErrUserNotFound = errors.New("usuário não encontrado")

// CreateUser stores a new account, indexed by both ID and normalized
// email. Fails with ErrUserExists if the email is already taken — checked
// via SETNX on the email-index key so two concurrent signups for the same
// email can't both succeed (the second one loses the race cleanly instead
// of silently overwriting the first).
func (r *Redis) CreateUser(ctx context.Context, email, firstName, lastName, passwordHash string) (*User, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           id,
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	claimed, err := r.setNX(ctx, keyUserByEmailPfx+email, id)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, ErrUserExists
	}

	if err := r.SetString(ctx, keyUserPfx+id, string(data), noExpiry); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *Redis) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	id, ok := r.GetString(ctx, keyUserByEmailPfx+email)
	if !ok {
		return nil, ErrUserNotFound
	}
	return r.GetUserByID(ctx, id)
}

func (r *Redis) GetUserByID(ctx context.Context, id string) (*User, error) {
	s, ok := r.GetString(ctx, keyUserPfx+id)
	if !ok {
		return nil, ErrUserNotFound
	}
	var u User
	if err := json.Unmarshal([]byte(s), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// MarkEmailVerified flips the flag and persists it — called once, when
// the confirmation link is clicked.
func (r *Redis) MarkEmailVerified(ctx context.Context, userID string) error {
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailVerified {
		return nil // already done, avoid a redundant write
	}
	user.EmailVerified = true
	return r.saveUser(ctx, user)
}

// saveUser persists an already-loaded, mutated User back to Redis —
// shared by every "load, flip a field, save" update (verification,
// avatar upload) so each of those isn't hand-rolling its own marshal.
func (r *Redis) saveUser(ctx context.Context, user *User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return r.SetString(ctx, keyUserPfx+user.ID, string(data), noExpiry)
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
