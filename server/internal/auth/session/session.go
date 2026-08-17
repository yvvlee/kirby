// Package session creates opaque refresh tokens and their browser cookies.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	CookieName  = "kirby_refresh"
	CookiePath  = "/auth"
	tokenLength = 32
)

var ErrInvalidOrigin = errors.New("invalid request origin")

// Generate creates a high-entropy bearer value and the only representation
// that may be persisted in MySQL.
func Generate() (string, []byte, error) {
	return generate(rand.Reader)
}

func generate(random io.Reader) (string, []byte, error) {
	if random == nil {
		return "", nil, fmt.Errorf("refresh-token random source is nil")
	}
	value := make([]byte, tokenLength)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(value)
	return plain, Hash(plain), nil
}

// Hash returns the fixed-size value stored in refresh_tokens.token_hash.
func Hash(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return append([]byte(nil), sum[:]...)
}

// Cookie creates the refresh-token cookie. secure must be true whenever the
// public browser origin uses HTTPS.
func Cookie(token string, expiresAt time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     CookiePath,
		Expires:  expiresAt.UTC(),
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearCookie removes the browser's refresh-token cookie.
func ClearCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     CookiePath,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// OriginValidator accepts only exact origins from the validated server config.
type OriginValidator struct {
	allowed map[string]struct{}
}

// NewOriginValidator copies the configured allow list.
func NewOriginValidator(origins []string) (*OriginValidator, error) {
	if len(origins) == 0 {
		return nil, fmt.Errorf("allowed origins are required")
	}
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || origin != parsed.Scheme+"://"+parsed.Host {
			return nil, fmt.Errorf("invalid allowed origin %q", origin)
		}
		allowed[origin] = struct{}{}
	}
	return &OriginValidator{allowed: allowed}, nil
}

// Validate rejects missing, opaque, wildcard, and non-exact browser origins.
func (v *OriginValidator) Validate(origin string) error {
	if v == nil || strings.TrimSpace(origin) == "" {
		return ErrInvalidOrigin
	}
	if _, ok := v.allowed[origin]; !ok {
		return ErrInvalidOrigin
	}
	return nil
}

// SecureForOrigin reports whether a cookie associated with origin must carry
// the Secure attribute.
func SecureForOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Scheme, "https")
}
