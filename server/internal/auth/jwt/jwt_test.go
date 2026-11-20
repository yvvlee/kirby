package jwt

import (
	"encoding/base64"
	stdjson "encoding/json"
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func jwtConfig(active string) Config {
	return Config{
		Issuer:    "kirby-test",
		ActiveKID: active,
		AccessTTL: 15 * time.Minute,
		Keys: map[string]string{
			"old": "old-012345678901234567890123456789",
			"new": "new-012345678901234567890123456789",
		},
	}
}

func TestIssueHasOnlyIdentityAndRegisteredClaims(t *testing.T) {
	manager, err := newManager(jwtConfig("new"), Audience)
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
	var values map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"iss": true,
		"sub": true,
		"aud": true,
		"exp": true,
		"nbf": true,
		"iat": true,
		"jti": true,
		"sid": true,
	}
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

// expiredToken 手工签发一个已过期的合法签名 token，用于验证过期拒绝路径。
func expiredToken(t *testing.T) string {
	t.Helper()
	past := time.Now().Add(-16 * time.Minute)
	claims := Claims{
		SessionID: "session",
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    "kirby-test",
			Subject:   "7",
			Audience:  jwtlib.ClaimStrings{Audience},
			ExpiresAt: jwtlib.NewNumericDate(past.Add(15 * time.Minute)),
			NotBefore: jwtlib.NewNumericDate(past),
			IssuedAt:  jwtlib.NewNumericDate(past),
			ID:        "expired",
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	token.Header["kid"] = "new"
	encoded, err := token.SignedString([]byte("new-012345678901234567890123456789"))
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestVerifyRejectsExpiryIssuerAudienceAndUnknownKey(t *testing.T) {
	issuer, err := newManager(jwtConfig("new"), Audience)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, err := issuer.Issue(7, "session")
	if err != nil {
		t.Fatal(err)
	}
	expired := expiredToken(t)

	tests := []struct {
		name     string
		config   Config
		audience string
		token    string
	}{
		{
			name:     "expired",
			config:   jwtConfig("new"),
			audience: Audience,
			token:    expired,
		},
		{
			name: "wrong issuer",
			config: func() Config {
				cfg := jwtConfig("new")
				cfg.Issuer = "other"
				return cfg
			}(),
			audience: Audience,
			token:    encoded,
		},
		{
			name:     "wrong audience",
			config:   jwtConfig("new"),
			audience: "other",
			token:    encoded,
		},
		{
			name: "unknown kid",
			config: func() Config {
				cfg := jwtConfig("old")
				delete(cfg.Keys, "new")
				return cfg
			}(),
			audience: Audience,
			token:    encoded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier, err := newManager(test.config, test.audience)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifier.Verify(test.token); err != ErrInvalidToken {
				t.Fatalf("Verify() error = %v", err)
			}
		})
	}
}

func TestKeyRotationKeepsOldVerificationKey(t *testing.T) {
	oldManager, err := newManager(jwtConfig("old"), Audience)
	if err != nil {
		t.Fatal(err)
	}
	oldToken, _, err := oldManager.Issue(9, "session")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := newManager(jwtConfig("new"), Audience)
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
