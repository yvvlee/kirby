// Package password hashes administrator passwords with Argon2id.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// AlgorithmVersion identifies Kirby's current password-hash policy. The
	// Argon2 version and all work parameters are also embedded in every hash.
	AlgorithmVersion = 1
	maxMemoryKiB     = 256 * 1024
	maxIterations    = 10
	maxParallelism   = 16
	maxDecodedPart   = 128
)

var ErrInvalidHash = errors.New("invalid password hash")

// Params contains the work factors used for new password hashes.
type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var defaultParams = Params{
	MemoryKiB:   64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// Hasher hashes and verifies passwords using one policy.
type Hasher struct {
	params Params
	random io.Reader
}

// New returns an Argon2id hasher after validating its resource limits.
func New(params Params) (*Hasher, error) {
	return newHasher(params, rand.Reader)
}

// NewDefault returns the production password hasher.
func NewDefault() (*Hasher, error) {
	return New(defaultParams)
}

// CurrentParams returns a copy of the fixed production password policy.
func CurrentParams() Params { return defaultParams }

func newHasher(params Params, random io.Reader) (*Hasher, error) {
	if err := validateParams(params); err != nil {
		return nil, err
	}
	if random == nil {
		return nil, fmt.Errorf("password salt source is nil")
	}
	return &Hasher{params: params, random: random}, nil
}

func validateParams(params Params) error {
	if params.MemoryKiB < 8*1024 || params.MemoryKiB > maxMemoryKiB {
		return fmt.Errorf("Argon2id memory must be between 8192 and %d KiB", maxMemoryKiB)
	}
	if params.Iterations < 1 || params.Iterations > maxIterations {
		return fmt.Errorf("Argon2id iterations must be between 1 and %d", maxIterations)
	}
	if params.Parallelism < 1 || params.Parallelism > maxParallelism {
		return fmt.Errorf("Argon2id parallelism must be between 1 and %d", maxParallelism)
	}
	if params.SaltLength < 16 || params.SaltLength > maxDecodedPart {
		return fmt.Errorf("Argon2id salt length must be between 16 and %d bytes", maxDecodedPart)
	}
	if params.KeyLength < 16 || params.KeyLength > maxDecodedPart {
		return fmt.Errorf("Argon2id key length must be between 16 and %d bytes", maxDecodedPart)
	}
	return nil
}

// Hash creates a self-describing PHC-format Argon2id hash.
func (h *Hasher) Hash(plain string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("password hasher is nil")
	}
	salt := make([]byte, h.params.SaltLength)
	if _, err := io.ReadFull(h.random, salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, h.params.Iterations, h.params.MemoryKiB, h.params.Parallelism, h.params.KeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.MemoryKiB,
		h.params.Iterations,
		h.params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify compares a password with a stored hash. needsRehash reports that a
// valid password uses an older work policy and should be upgraded on login.
func (h *Hasher) Verify(encoded, plain string) (match bool, needsRehash bool, err error) {
	if h == nil {
		return false, false, fmt.Errorf("password hasher is nil")
	}
	params, salt, expected, err := parse(encoded)
	if err != nil {
		return false, false, err
	}
	actual := argon2.IDKey([]byte(plain), salt, params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(expected)))
	match = subtle.ConstantTimeCompare(actual, expected) == 1
	return match, params != h.params, nil
}

func parse(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return Params{}, nil, nil, ErrInvalidHash
	}

	var params Params
	count, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.MemoryKiB, &params.Iterations, &params.Parallelism)
	if err != nil || count != 3 || parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", params.MemoryKiB, params.Iterations, params.Parallelism) {
		return Params{}, nil, nil, ErrInvalidHash
	}
	salt, err := decodePart(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	expected, err := decodePart(parts[5])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(expected))
	if err := validateParams(params); err != nil {
		return Params{}, nil, nil, errors.Join(ErrInvalidHash, err)
	}
	return params, salt, expected, nil
}

func decodePart(value string) ([]byte, error) {
	if value == "" || len(value) > base64.RawStdEncoding.EncodedLen(maxDecodedPart) {
		return nil, ErrInvalidHash
	}
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(decoded) == 0 || len(decoded) > maxDecodedPart {
		return nil, ErrInvalidHash
	}
	return decoded, nil
}
