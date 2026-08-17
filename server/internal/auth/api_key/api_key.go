// Package api_key creates and verifies project credentials without retaining
// plaintext secrets.
package api_key

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/yvvlee/kirby/server/internal/config"
)

const (
	publicPrefix       = "kirby_pk_"
	publicEntropyBytes = 16
	secretEntropyBytes = 32
	secretSuffixLength = 4
)

type Generated struct {
	PublicID     string
	Full         string
	Hash         []byte
	SecretSuffix string
}

type Manager struct {
	pepper []byte
	random io.Reader
}

func New(pepper config.Secret) (*Manager, error) {
	return newManager([]byte(pepper.Value()), rand.Reader)
}

func newManager(pepper []byte, random io.Reader) (*Manager, error) {
	if len(pepper) < 32 {
		return nil, fmt.Errorf("API key pepper must contain at least 32 bytes")
	}
	if random == nil {
		return nil, fmt.Errorf("API key random source is nil")
	}
	return &Manager{pepper: append([]byte(nil), pepper...), random: random}, nil
}

func (m *Manager) Generate() (*Generated, error) {
	publicEntropy := make([]byte, publicEntropyBytes)
	secretEntropy := make([]byte, secretEntropyBytes)
	if _, err := io.ReadFull(m.random, publicEntropy); err != nil {
		return nil, fmt.Errorf("generate API key public id: %w", err)
	}
	if _, err := io.ReadFull(m.random, secretEntropy); err != nil {
		return nil, fmt.Errorf("generate API key secret: %w", err)
	}
	publicID := publicPrefix + base64.RawURLEncoding.EncodeToString(publicEntropy)
	secret := base64.RawURLEncoding.EncodeToString(secretEntropy)
	full := publicID + "." + secret
	return &Generated{
		PublicID: publicID, Full: full, Hash: m.hash(full),
		SecretSuffix: secret[len(secret)-secretSuffixLength:],
	}, nil
}

// PublicID validates the full credential and returns only its queryable part.
func (m *Manager) PublicID(full string) (string, error) {
	publicID, _, err := parse(full)
	return publicID, err
}

// Verify compares both the public id and HMAC in constant time.
func (m *Manager) Verify(full, expectedPublicID string, expectedHash []byte) bool {
	publicID, _, err := parse(full)
	if err != nil {
		return false
	}
	publicMatches := subtle.ConstantTimeCompare([]byte(publicID), []byte(expectedPublicID))
	hashMatches := subtle.ConstantTimeCompare(m.hash(full), expectedHash)
	return publicMatches&hashMatches == 1
}

func (m *Manager) hash(full string) []byte {
	digest := hmac.New(sha256.New, m.pepper)
	_, _ = digest.Write([]byte(full))
	return digest.Sum(nil)
}

func parse(full string) (string, string, error) {
	if strings.TrimSpace(full) != full || strings.Count(full, ".") != 1 {
		return "", "", fmt.Errorf("invalid API key format")
	}
	publicID, secret, _ := strings.Cut(full, ".")
	if !strings.HasPrefix(publicID, publicPrefix) {
		return "", "", fmt.Errorf("invalid API key public id")
	}
	publicEncoded := strings.TrimPrefix(publicID, publicPrefix)
	publicBytes, err := base64.RawURLEncoding.DecodeString(publicEncoded)
	if err != nil || len(publicBytes) != publicEntropyBytes || base64.RawURLEncoding.EncodeToString(publicBytes) != publicEncoded {
		return "", "", fmt.Errorf("invalid API key public id")
	}
	secretBytes, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(secretBytes) != secretEntropyBytes || base64.RawURLEncoding.EncodeToString(secretBytes) != secret {
		return "", "", fmt.Errorf("invalid API key secret")
	}
	return publicID, secret, nil
}
