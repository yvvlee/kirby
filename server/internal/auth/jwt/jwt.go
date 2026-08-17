// Package jwt signs and verifies short-lived administrator access tokens.
package jwt

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/yvvlee/kirby/server/internal/config"
)

const Audience = "kirby-admin"

const AccessTTL = 15 * time.Minute

var ErrInvalidToken = errors.New("invalid access token")

// Claims deliberately contains no environment, role, or permission data.
// Authorization is resolved from shared server-side state on every request.
type Claims struct {
	SessionID string `json:"sid"`
	jwtlib.RegisteredClaims
}

// UserID returns the positive database user ID stored in subject.
func (c *Claims) UserID() (int64, error) {
	if c == nil {
		return 0, ErrInvalidToken
	}
	value, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || value <= 0 {
		return 0, ErrInvalidToken
	}
	return value, nil
}

// Manager owns an immutable signing key ring shared by all server instances.
type Manager struct {
	issuer    string
	audience  string
	activeKID string
	accessTTL time.Duration
	keys      map[string][]byte
	now       func() time.Time
}

// New creates a manager from validated application configuration.
func New(cfg config.JWTConfig) (*Manager, error) {
	return newManager(cfg, Audience, time.Now)
}

func newManager(cfg config.JWTConfig, audience string, now func() time.Time) (*Manager, error) {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, fmt.Errorf("JWT issuer is required")
	}
	if strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("JWT audience is required")
	}
	if cfg.AccessTTL.Duration <= 0 {
		return nil, fmt.Errorf("JWT access TTL must be greater than zero")
	}
	if cfg.AccessTTL.Duration != AccessTTL {
		return nil, fmt.Errorf("JWT access TTL must be %s", AccessTTL)
	}
	if strings.TrimSpace(cfg.ActiveKID) == "" {
		return nil, fmt.Errorf("active JWT key id is required")
	}
	if now == nil {
		return nil, fmt.Errorf("JWT clock is nil")
	}
	keys := make(map[string][]byte, len(cfg.Keys))
	for kid, secret := range cfg.Keys {
		if strings.TrimSpace(kid) == "" || len(secret.Value()) < 32 {
			return nil, fmt.Errorf("each JWT key must have a non-empty id and at least 32 bytes")
		}
		keys[kid] = append([]byte(nil), secret.Value()...)
	}
	if _, ok := keys[cfg.ActiveKID]; !ok {
		return nil, fmt.Errorf("active JWT key %q is absent", cfg.ActiveKID)
	}
	return &Manager{
		issuer:    cfg.Issuer,
		audience:  audience,
		activeKID: cfg.ActiveKID,
		accessTTL: cfg.AccessTTL.Duration,
		keys:      keys,
		now:       now,
	}, nil
}

// AccessTTL returns the configured access-token lifetime.
func (m *Manager) AccessTTL() time.Duration {
	if m == nil {
		return 0
	}
	return m.accessTTL
}

// Issue signs one access token for a user and refresh session.
func (m *Manager) Issue(userID int64, sessionID string) (string, time.Time, error) {
	if m == nil {
		return "", time.Time{}, fmt.Errorf("JWT manager is nil")
	}
	if userID <= 0 || strings.TrimSpace(sessionID) == "" {
		return "", time.Time{}, fmt.Errorf("JWT user and session are required")
	}
	now := m.now().UTC()
	expiresAt := now.Add(m.accessTTL)
	claims := Claims{
		SessionID: sessionID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(userID, 10),
			Audience:  jwtlib.ClaimStrings{m.audience},
			ExpiresAt: jwtlib.NewNumericDate(expiresAt),
			NotBefore: jwtlib.NewNumericDate(now),
			IssuedAt:  jwtlib.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	token.Header["kid"] = m.activeKID
	signed, err := token.SignedString(m.keys[m.activeKID])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// Verify validates the algorithm, key id, signature, issuer, audience, time
// bounds, and all required identity claims.
func (m *Manager) Verify(encoded string) (*Claims, error) {
	if m == nil || strings.TrimSpace(encoded) == "" {
		return nil, ErrInvalidToken
	}
	claims := new(Claims)
	parser := jwtlib.NewParser(
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
		jwtlib.WithIssuer(m.issuer),
		jwtlib.WithAudience(m.audience),
		jwtlib.WithExpirationRequired(),
		jwtlib.WithIssuedAt(),
		jwtlib.WithTimeFunc(m.now),
	)
	token, err := parser.ParseWithClaims(encoded, claims, func(token *jwtlib.Token) (any, error) {
		if token.Method != jwtlib.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		kid, ok := token.Header["kid"].(string)
		if !ok || strings.TrimSpace(kid) == "" {
			return nil, ErrInvalidToken
		}
		key, ok := m.keys[kid]
		if !ok {
			return nil, ErrInvalidToken
		}
		return key, nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.IssuedAt == nil || claims.NotBefore == nil || claims.ExpiresAt == nil || claims.ID == "" || claims.SessionID == "" {
		return nil, ErrInvalidToken
	}
	if _, err := claims.UserID(); err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
