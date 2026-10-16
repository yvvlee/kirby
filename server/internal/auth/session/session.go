// Package session creates opaque refresh tokens and their browser cookies.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yvvlee/kirby/server/internal/conf"
)

const (
	CookieName  = "kirby_refresh"
	CookiePath  = "/api/auth"
	tokenLength = 32
)

var ErrInvalidOrigin = errors.New("invalid request origin")

// Generate creates a high-entropy bearer value and the only representation
// that may be persisted in MySQL.
//
// Parameters: none.
// Returns: the bearer token and its SHA-256 hash.
// Throws: an error if cryptographic randomness cannot be read.
func Generate() (string, []byte, error) {
	return generate(rand.Reader)
}

func generate(random io.Reader) (string, []byte, error) {
	value := make([]byte, tokenLength)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(value)
	return plain, Hash(plain), nil
}

// Hash returns the fixed-size value stored in refresh_tokens.token_hash.
//
// Parameters: plain is the refresh credential.
// Returns: a copy of its SHA-256 digest.
// Throws: none.
func Hash(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return append([]byte(nil), sum[:]...)
}

// Cookie creates the refresh-token cookie. secure must be true whenever the
// public browser origin uses HTTPS.
//
// Parameters: token is the bearer value; expiresAt is its expiry; secure reflects the validated public origin.
// Returns: an HttpOnly, SameSite=Lax refresh cookie.
// Throws: none.
func Cookie(token string, expiresAt time.Time, secure bool) *http.Cookie {
	// #nosec G124 -- secure follows the validated public origin; HTTP local development requires false.
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
//
// Parameters: secure reflects the validated public origin.
// Returns: an expired refresh cookie.
// Throws: none.
func ClearCookie(secure bool) *http.Cookie {
	// #nosec G124 -- secure follows the validated public origin; HTTP local development requires false.
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

// OriginValidator compares browser origins with the public request address.
// Forwarding headers are trusted only when the immediate peer is configured.
type OriginValidator struct {
	trusted []netip.Prefix
}

// NewOriginValidator constructs a same-origin policy without deployment domains.
// Parameters: trustedCIDRs identifies reverse proxies allowed to supply the public address.
// A nil list trusts all proxies; an explicit empty list uses the direct request address.
// Returns: the origin policy.
// Throws: an error when a trusted proxy CIDR is invalid.
func NewOriginValidator(trustedCIDRs []string) (*OriginValidator, error) {
	trusted, err := conf.ParseTrustedProxyCIDRs(trustedCIDRs)
	if err != nil {
		return nil, err
	}
	return &OriginValidator{trusted: trusted}, nil
}

// Validate requires an explicit browser origin matching the public request address.
// Parameters: request is the incoming HTTP request, including its immediate peer.
// Returns: nil for a same-origin request.
// Throws: ErrInvalidOrigin for missing, opaque, malformed, or different origins.
func (v *OriginValidator) Validate(request *http.Request) error {
	origins := request.Header.Values("Origin")
	if len(origins) != 1 {
		return ErrInvalidOrigin
	}
	origin, err := normalizedOrigin(origins[0])
	if err != nil {
		return err
	}
	expected, err := v.requestOrigin(request)
	if err != nil || origin != expected {
		return ErrInvalidOrigin
	}
	return nil
}

func (v *OriginValidator) requestOrigin(request *http.Request) (string, error) {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	host := request.Host
	if v.trustedPeer(request.RemoteAddr) {
		return forwardedOrigin(request, scheme, host)
	}
	return normalizedOrigin(scheme + "://" + host)
}

// PublicOrigin resolves the address used by browsers without deployment domains.
// Parameters: request contains the direct address and any trusted proxy headers.
// Returns: an HTTP or HTTPS origin with non-default ports preserved.
// Throws: ErrInvalidOrigin when the request address or forwarding headers are invalid.
func (v *OriginValidator) PublicOrigin(request *http.Request) (string, error) {
	origin, err := v.requestOrigin(request)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", ErrInvalidOrigin
	}
	if parsed.Scheme == "http" && parsed.Port() == "80" || parsed.Scheme == "https" && parsed.Port() == "443" {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":"+parsed.Port())
	}
	return parsed.String(), nil
}

func (v *OriginValidator) trustedPeer(remoteAddr string) bool {
	addressPort, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return false
	}
	address := addressPort.Addr().Unmap()
	for _, prefix := range v.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func forwardedOrigin(request *http.Request, scheme, host string) (string, error) {
	forwardedScheme, err := singleForwardedValue(request.Header, "X-Forwarded-Proto")
	if err != nil {
		return "", err
	}
	forwardedHost, err := singleForwardedValue(request.Header, "X-Forwarded-Host")
	if err != nil {
		return "", err
	}
	if forwardedScheme != "" {
		scheme = forwardedScheme
	}
	if forwardedHost != "" {
		host = forwardedHost
	}
	return normalizedOrigin(scheme + "://" + host)
}

func singleForwardedValue(header http.Header, name string) (string, error) {
	values := header.Values(name)
	if len(values) == 0 {
		return "", nil
	}
	if len(values) != 1 || values[0] == "" || strings.Contains(values[0], ",") || values[0] != strings.TrimSpace(values[0]) {
		return "", ErrInvalidOrigin
	}
	return values[0], nil
}

func normalizedOrigin(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return "", ErrInvalidOrigin
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidOrigin
	}
	if value != parsed.Scheme+"://"+parsed.Host {
		return "", ErrInvalidOrigin
	}
	port := parsed.Port()
	if port == "" {
		port = "80"
		if parsed.Scheme == "https" {
			port = "443"
		}
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 || parsed.Hostname() == "" {
		return "", ErrInvalidOrigin
	}
	return parsed.Scheme + "://" + net.JoinHostPort(strings.ToLower(parsed.Hostname()), strconv.Itoa(portNumber)), nil
}

// SecureForOrigin reports whether a cookie associated with origin must carry
// the Secure attribute.
//
// Parameters: origin is the validated public browser origin.
// Returns: whether its scheme is HTTPS.
// Throws: none.
func SecureForOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Scheme, "https")
}
