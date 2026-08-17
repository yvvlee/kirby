package api_key

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/config"
)

func TestGenerateStoresOnlyVerifiableDigestMaterial(t *testing.T) {
	randomBytes := append(bytes.Repeat([]byte{0x11}, publicEntropyBytes), bytes.Repeat([]byte{0x22}, secretEntropyBytes)...)
	manager, err := newManager([]byte("01234567890123456789012345678901"), bytes.NewReader(randomBytes))
	require.NoError(t, err)

	generated, err := manager.Generate()

	require.NoError(t, err)
	assert.Contains(t, generated.Full, generated.PublicID+".")
	assert.Len(t, generated.Hash, sha256DigestBytes)
	assert.NotContains(t, string(generated.Hash), generated.Full)
	assert.Equal(t, generated.Full[len(generated.Full)-secretSuffixLength:], generated.SecretSuffix)
	assert.True(t, manager.Verify(generated.Full, generated.PublicID, generated.Hash))
	assert.False(t, manager.Verify(generated.Full+"x", generated.PublicID, generated.Hash))
	assert.False(t, manager.Verify(generated.Full, generated.PublicID+"x", generated.Hash))
	assert.False(t, manager.Verify(generated.Full, generated.PublicID, bytes.Repeat([]byte{0}, sha256DigestBytes)))
}

const sha256DigestBytes = 32

func TestPublicIDRejectsMalformedCredentials(t *testing.T) {
	manager, err := New(config.NewSecret("01234567890123456789012345678901"))
	require.NoError(t, err)
	for _, value := range []string{"", " key ", "wrong.value", "kirby_pk_bad.bad", "kirby_pk_a.b.c"} {
		_, err := manager.PublicID(value)
		assert.Error(t, err, value)
	}
}

func TestManagerFailsFastOnUnsafeDependencies(t *testing.T) {
	_, err := newManager([]byte("short"), bytes.NewReader(nil))
	assert.Error(t, err)
	_, err = newManager(bytes.Repeat([]byte{'p'}, 32), nil)
	assert.Error(t, err)

	manager, err := newManager(bytes.Repeat([]byte{'p'}, 32), errorReader{})
	require.NoError(t, err)
	_, err = manager.Generate()
	assert.Error(t, err)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("random failed") }
