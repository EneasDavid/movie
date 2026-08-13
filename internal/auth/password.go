// Package auth handles password hashing and session tokens — the two
// primitives everything else in the auth flow builds on.
package auth

import (
	"errors"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost trades hashing latency for resistance to offline brute-force.
// 12 is comfortably above the current baseline recommendation (10) while
// staying well under a second per hash on ordinary hardware.
const bcryptCost = 12

const MinPasswordLength = 8

const specialChars = `!@#$%^&*()_+-=[]{}|;:'",.<>/?~` + "`\\"

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// HashPassword salts and hashes a plaintext password. bcrypt generates and
// embeds a random salt per call, so identical passwords never produce the
// same hash.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword reports whether plaintext matches the stored bcrypt hash.
// Constant-time by construction (bcrypt.CompareHashAndPassword).
func VerifyPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

// NormalizeEmail lowercases and trims an email so "User@Example.com" and
// "user@example.com" are treated as the same account.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

var (
	ErrInvalidEmail        = errors.New("email inválido")
	ErrNameRequired        = errors.New("nome e sobrenome são obrigatórios")
	ErrPasswordTooShort    = errors.New("a senha precisa ter pelo menos 8 caracteres")
	ErrPasswordNeedsUpper  = errors.New("a senha precisa de ao menos uma letra maiúscula")
	ErrPasswordNeedsLower  = errors.New("a senha precisa de ao menos uma letra minúscula")
	ErrPasswordNeedsDigit  = errors.New("a senha precisa de ao menos um número")
	ErrPasswordNeedsSymbol = errors.New("a senha precisa de ao menos um caractere especial")
	ErrPasswordSequential  = errors.New("a senha não pode conter uma sequência numérica óbvia (ex: 1234)")
)

// ValidatePassword applies the full strength policy. Checked in order so
// the error message tells the user the single most relevant thing to fix
// first, rather than every problem at once — the frontend's live
// checklist (mirroring these same rules) is where "see everything at
// once" belongs.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case strings.ContainsRune(specialChars, r):
			hasSymbol = true
		}
	}
	if !hasUpper {
		return ErrPasswordNeedsUpper
	}
	if !hasLower {
		return ErrPasswordNeedsLower
	}
	if !hasDigit {
		return ErrPasswordNeedsDigit
	}
	if !hasSymbol {
		return ErrPasswordNeedsSymbol
	}
	if hasSequentialDigits(password) {
		return ErrPasswordSequential
	}
	return nil
}

// hasSequentialDigits catches runs like "1234", "4321", or "0000" — a
// password can satisfy every character-class rule above and still be
// "Password1234!", which this rejects.
func hasSequentialDigits(password string) bool {
	const runLength = 4
	digits := []rune{}
	for _, r := range password {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		} else {
			digits = nil // sequences don't span non-digit characters
		}
		if len(digits) >= runLength && isSequentialRun(digits[len(digits)-runLength:]) {
			return true
		}
	}
	return false
}

func isSequentialRun(digits []rune) bool {
	ascending, descending, repeated := true, true, true
	for i := 1; i < len(digits); i++ {
		diff := digits[i] - digits[i-1]
		if diff != 1 {
			ascending = false
		}
		if diff != -1 {
			descending = false
		}
		if diff != 0 {
			repeated = false
		}
	}
	return ascending || descending || repeated
}

// ValidateCredentials applies the minimum server-side checks before we ever
// touch the database — cheap to fail fast on obviously-bad input.
func ValidateCredentials(email, password string) error {
	if !emailPattern.MatchString(email) {
		return ErrInvalidEmail
	}
	return ValidatePassword(password)
}

// ValidateName rejects empty/whitespace-only names. Deliberately loose
// otherwise — real names vary too much (hyphens, apostrophes, non-Latin
// scripts) to validate more strictly without rejecting real users.
func ValidateName(firstName, lastName string) error {
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return ErrNameRequired
	}
	return nil
}
