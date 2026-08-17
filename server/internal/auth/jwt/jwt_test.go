package jwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/yvvlee/kirby/server/internal/config"
)

func jwtConfig(active string) config.JWTConfig {
	return config.JWTConfig{
		Issuer:    "kirby-test",
		ActiveKID: active,
		AccessTTL: config.Duration{Duration: 15 * time.Minute},
		Keys: map[string]config.Secret{
			"old": config.NewSecret("old-012345678901234567890123456789"),
			"new": config.NewSecret("new-012345678901234567890123456789"),
		},
	}
}

func TestIssueHasOnlyIdentityAndRegisteredClaims(t *testing.T) {
	now := time.Date(2026, 8, 17, 1, 2, 3, 0, time.UTC)
	manager, err := newManager(jwtConfig("new"), Audience, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, err := manager.Issue(42, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(encoded, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"iss": true, "sub": true, "aud": true, "exp": true, "nbf": true, "iat": true, "jti": true, "sid": true}
	if len(values) != len(want) {
		t.Fatalf("unexpected JWT claims: %v", values)
	}
	for key := range values {
		if !want[key] {
			t.Fatalf("JWT contains unauthorized claim %q", key)
		}
	}
	claims, err := manager.Verify(encoded)
	if err != nil || claims.SessionID != "session-1" {
		t.Fatalf("verify issued token: claims=%+v err=%v", claims, err)
	}
}

func TestVerifyRejectsExpiryIssuerAudienceAndUnknownKey(t *testing.T) {
	issuedAt := time.Date(2026, 8, 17, 1, 2, 3, 0, time.UTC)
	issuer, err := newManager(jwtConfig("new"), Audience, func() time.Time { return issuedAt })
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, err := issuer.Issue(7, "session")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		config   config.JWTConfig
		audience string
		now      time.Time
	}{
		{name: "expired", config: jwtConfig("new"), audience: Audience, now: issuedAt.Add(16 * time.Minute)},
		{name: "wrong issuer", config: func() config.JWTConfig { cfg := jwtConfig("new"); cfg.Issuer = "other"; return cfg }(), audience: Audience, now: issuedAt},
		{name: "wrong audience", config: jwtConfig("new"), audience: "other", now: issuedAt},
		{name: "unknown kid", config: func() config.JWTConfig { cfg := jwtConfig("old"); delete(cfg.Keys, "new"); return cfg }(), audience: Audience, now: issuedAt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier, err := newManager(test.config, test.audience, func() time.Time { return test.now })
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifier.Verify(encoded); err != ErrInvalidToken {
				t.Fatalf("Verify() error = %v", err)
			}
		})
	}
}

func TestKeyRotationKeepsOldVerificationKey(t *testing.T) {
	now := time.Date(2026, 8, 17, 1, 2, 3, 0, time.UTC)
	oldManager, err := newManager(jwtConfig("old"), Audience, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	oldToken, _, err := oldManager.Issue(9, "session")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := newManager(jwtConfig("new"), Audience, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rotated.Verify(oldToken); err != nil {
		t.Fatalf("rotated key ring rejected old token: %v", err)
	}
	newToken, _, err := rotated.Issue(9, "session")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _, err := jwtlib.NewParser().ParseUnverified(newToken, jwtlib.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header["kid"] != "new" {
		t.Fatalf("new token kid = %v", parsed.Header["kid"])
	}
}
