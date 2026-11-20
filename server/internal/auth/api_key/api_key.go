// Package apikey creates and verifies project credentials without retaining
// plaintext secrets.
package apikey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	secretEntropyBytes = 32
	secretSuffixLength = 4
)

type Generated struct {
	Full         string
	Hash         []byte
	SecretSuffix string
}

type Manager struct {
	pepper []byte
	random io.Reader
}

// New binds API-key hashing to the persisted installation pepper.
//
// Parameters: pepper is the installation-specific hashing secret.
// Returns: a credential manager.
// Throws: an error if the pepper is empty or too short.
func New(pepper string) (*Manager, error) {
	return newManager([]byte(pepper), rand.Reader)
}

func newManager(pepper []byte, random io.Reader) (*Manager, error) {
	if len(pepper) < 32 {
		return nil, fmt.Errorf("API key pepper must contain at least 32 bytes")
	}
	return &Manager{
		pepper: append([]byte(nil), pepper...),
		random: random,
	}, nil
}

// Generate creates one credential digest and secret; only its hash and suffix may be stored.
//
// Parameters: none.
// Returns: the full credential, credential digest, hash, and display suffix.
// Throws: an error when cryptographic randomness cannot be read.
func (m *Manager) Generate() (*Generated, error) {
	secretEntropy := make([]byte, secretEntropyBytes)
	if _, err := io.ReadFull(m.random, secretEntropy); err != nil {
		return nil, fmt.Errorf("generate API key: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(secretEntropy)
	full := secret
	return &Generated{
		Full:         full,
		Hash:         m.hash(full),
		SecretSuffix: secret[len(secret)-secretSuffixLength:],
	}, nil
}

// LookupHash validates canonical random secrets before deriving the database lookup digest.
// Parameters: full is the presented credential.
// Returns: its peppered HMAC digest.
// Throws: invalid encoding or entropy length errors.
func (m *Manager) LookupHash(full string) ([]byte, error) {
	if len(full) != 43 {
		return nil, fmt.Errorf("invalid API key format")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(full)
	if err != nil || len(decoded) != secretEntropyBytes || base64.RawURLEncoding.EncodeToString(decoded) != full {
		return nil, fmt.Errorf("invalid API key format")
	}
	return m.hash(full), nil
}

// Verify compares the presented secret to a stored digest in constant time.
// Parameters: full is the secret; expectedHash is the stored HMAC.
// Returns: whether the credential matches.
// Throws: none.
func (m *Manager) Verify(full string, expectedHash []byte) bool {
	hash, err := m.LookupHash(full)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1
}

func (m *Manager) hash(full string) []byte {
	digest := hmac.New(sha256.New, m.pepper)
	_, _ = digest.Write([]byte(full))
	return digest.Sum(nil)
}
