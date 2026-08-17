package session

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestGenerateStoresOnlyHash(t *testing.T) {
	plain, hash, err := generate(bytes.NewReader(bytes.Repeat([]byte{0x7f}, tokenLength)))
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) < 40 || len(hash) != 32 {
		t.Fatalf("plain length=%d hash length=%d", len(plain), len(hash))
	}
	if bytes.Contains(hash, []byte(plain)) {
		t.Fatal("stored refresh hash contains plaintext")
	}
	if !bytes.Equal(hash, Hash(plain)) {
		t.Fatal("refresh-token hash is not deterministic")
	}
}

func TestRefreshCookieSecurityContract(t *testing.T) {
	if CookiePath != "/api/auth" {
		t.Fatalf("unexpected refresh cookie path: %q", CookiePath)
	}
	cookie := Cookie("opaque", time.Now().Add(time.Hour), true)
	if cookie.Name != CookieName || cookie.Path != CookiePath || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Domain != "" {
		t.Fatalf("unsafe refresh cookie: %+v", cookie)
	}
	cleared := ClearCookie(true)
	if cleared.MaxAge != -1 || cleared.Value != "" {
		t.Fatalf("refresh cookie was not cleared: %+v", cleared)
	}
}

func TestOriginValidatorUsesExactMatches(t *testing.T) {
	validator, err := NewOriginValidator([]string{"https://kirby.example.com", "http://localhost:8000"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validator.Validate("https://kirby.example.com"); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "https://kirby.example.com.evil", "https://kirby.example.com/", "null"} {
		if err := validator.Validate(origin); !errors.Is(err, ErrInvalidOrigin) {
			t.Fatalf("Validate(%q) error = %v", origin, err)
		}
	}
	if !SecureForOrigin("https://kirby.example.com") || SecureForOrigin("http://localhost:8000") {
		t.Fatal("origin cookie security detection failed")
	}
}
