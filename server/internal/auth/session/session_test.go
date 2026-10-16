package session

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestOriginValidatorMatchesPublicRequestWithoutDomainConfig(t *testing.T) {
	validator, err := NewOriginValidator(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://kirby.example.com/api/auth/login", nil)
	for _, origin := range []string{"https://kirby.example.com", "https://kirby.example.com:443"} {
		request.Header.Set("Origin", origin)
		if err := validator.Validate(request); err != nil {
			t.Fatalf("same origin %q: %v", origin, err)
		}
	}
	for _, origin := range []string{
		"",
		"null",
		"https://kirby.example.com.evil",
		"https://kirby.example.com/",
		"http://kirby.example.com",
		"https://kirby.example.com:8443",
		"https://kirby.example.com?",
		"https://kirby.example.com#",
		"https://user@kirby.example.com",
		"https://kirby.example.com:invalid",
		"https://kirby.example.com:65536",
	} {
		request.Header.Set("Origin", origin)
		if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
			t.Fatalf("Validate(%q) error = %v", origin, err)
		}
	}
	request.Header.Set("Origin", "https://kirby.example.com")
	request.Header.Add("Origin", "https://evil.example.com")
	if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
		t.Fatalf("duplicate Origin error = %v", err)
	}
	if !SecureForOrigin("https://kirby.example.com") || SecureForOrigin("http://localhost:8000") {
		t.Fatal("origin cookie security detection failed")
	}
}

func TestOriginValidatorTrustsForwardedAddressOnlyFromConfiguredProxies(t *testing.T) {
	validator, err := NewOriginValidator([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://kirby:8080/api/auth/login", nil)
	request.RemoteAddr = "10.0.0.2:12345"
	request.Header.Set("Origin", "https://kirby.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "kirby.example.com")
	if err := validator.Validate(request); err != nil {
		t.Fatalf("trusted proxy origin: %v", err)
	}
	request.RemoteAddr = "192.0.2.1:12345"
	if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
		t.Fatalf("untrusted peer forged public address: %v", err)
	}
	request.Header.Set("Origin", "http://kirby:8080")
	if err := validator.Validate(request); err != nil {
		t.Fatalf("untrusted forwarding headers affected direct origin: %v", err)
	}
}

func TestOriginValidatorRejectsAmbiguousTrustedProxyHeaders(t *testing.T) {
	validator, err := NewOriginValidator([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"https,http", " https", "ftp", ""} {
		request := httptest.NewRequest(http.MethodPost, "http://kirby:8080/api/auth/login", nil)
		request.RemoteAddr = "10.0.0.2:12345"
		request.Header.Set("Origin", "https://kirby.example.com")
		request.Header.Set("X-Forwarded-Proto", value)
		request.Header.Set("X-Forwarded-Host", "kirby.example.com")
		if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
			t.Fatalf("forwarded scheme %q accepted: %v", value, err)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "http://kirby:8080/api/auth/login", nil)
	request.RemoteAddr = "10.0.0.2:12345"
	request.Header.Set("Origin", "https://kirby.example.com")
	request.Header.Add("X-Forwarded-Proto", "https")
	request.Header.Add("X-Forwarded-Proto", "http")
	request.Header.Set("X-Forwarded-Host", "kirby.example.com")
	if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
		t.Fatalf("duplicate forwarded scheme accepted: %v", err)
	}
}

func TestOriginValidatorSupportsDirectDevelopmentProxyAndNonDefaultPort(t *testing.T) {
	validator, err := NewOriginValidator(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost:15173/api/auth/login", nil)
	request.Header.Set("Origin", "http://localhost:15173")
	if err := validator.Validate(request); err != nil {
		t.Fatalf("development proxy retaining browser Host: %v", err)
	}
	request.Header.Set("Origin", "http://localhost")
	if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
		t.Fatalf("different port accepted: %v", err)
	}
}

func TestOriginValidatorDefaultsToTrustingIPv4AndIPv6Proxies(t *testing.T) {
	validator, err := NewOriginValidator(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range []string{"172.20.0.2:12345", "[2001:db8::2]:12345"} {
		request := httptest.NewRequest(http.MethodPost, "http://kirby:8080/api/auth/login", nil)
		request.RemoteAddr = peer
		request.Header.Set("Origin", "https://kirby.example.com")
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("X-Forwarded-Host", "kirby.example.com")
		if err := validator.Validate(request); err != nil {
			t.Fatalf("default proxy policy for %q: %v", peer, err)
		}
	}
}

func TestOriginValidatorExplicitEmptyProxyListIgnoresForwardedOrigin(t *testing.T) {
	validator, err := NewOriginValidator([]string{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://kirby:8080/api/auth/login", nil)
	request.RemoteAddr = "172.20.0.2:12345"
	request.Header.Set("Origin", "https://kirby.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "kirby.example.com")
	if err := validator.Validate(request); !errors.Is(err, ErrInvalidOrigin) {
		t.Fatalf("explicit direct policy trusted forwarded origin: %v", err)
	}
}

type publicOriginTestCase struct {
	name           string
	address        string
	trusted        []string
	forwardedHost  string
	forwardedProto string
	want           string
}

func TestPublicOriginUsesTrustedRequestAddress(t *testing.T) {
	tests := []publicOriginTestCase{
		{
			name:    "direct nondefault port",
			address: "http://localhost:18173",
			want:    "http://localhost:18173",
		},
		{
			name:    "direct TLS",
			address: "https://kirby.example.com:443",
			want:    "https://kirby.example.com",
		},
		{
			name:           "trusted HTTPS proxy",
			address:        "http://kirby:8080",
			forwardedHost:  "config.example.com",
			forwardedProto: "https",
			want:           "https://config.example.com",
		},
		{
			name:           "untrusted proxy headers ignored",
			address:        "http://localhost:18173",
			trusted:        []string{},
			forwardedHost:  "config.example.com",
			forwardedProto: "https",
			want:           "http://localhost:18173",
		},
		{
			name:    "IPv6 default port",
			address: "http://[::1]:80",
			want:    "http://[::1]",
		},
		{
			name:           "IPv6 proxy nondefault port",
			address:        "http://kirby:8080",
			forwardedHost:  "[2001:db8::1]:8443",
			forwardedProto: "https",
			want:           "https://[2001:db8::1]:8443",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func (tt publicOriginTestCase) run(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, tt.address, nil)
	request.RemoteAddr = "192.0.2.1:54321"
	if tt.forwardedHost != "" {
		request.Header.Set("X-Forwarded-Host", tt.forwardedHost)
		request.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
	}
	policy, err := NewOriginValidator(tt.trusted)
	if err != nil {
		t.Fatal(err)
	}
	got, err := policy.PublicOrigin(request)
	if err != nil || got != tt.want {
		t.Fatalf("PublicOrigin() = %q, %v; want %q", got, err, tt.want)
	}
}
