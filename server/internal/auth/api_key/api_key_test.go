package apikey

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateStoresOnlyVerifiableDigestMaterial(t *testing.T) {
	randomBytes := bytes.Repeat([]byte{0x22}, secretEntropyBytes)
	manager, err := newManager([]byte("01234567890123456789012345678901"), bytes.NewReader(randomBytes))
	require.NoError(t, err)

	generated, err := manager.Generate()

	require.NoError(t, err)
	assert.Len(t, generated.Full, 43)
	decoded, err := base64.RawURLEncoding.DecodeString(generated.Full)
	require.NoError(t, err)
	assert.Len(t, decoded, 32)
	digest, err := manager.LookupHash(generated.Full)
	require.NoError(t, err)
	assert.Equal(t, generated.Hash, digest)
	assert.Len(t, generated.Hash, sha256DigestBytes)
	assert.NotContains(t, string(generated.Hash), generated.Full)
	assert.Equal(t, generated.Full[len(generated.Full)-secretSuffixLength:], generated.SecretSuffix)
	assert.True(t, manager.Verify(generated.Full, generated.Hash))
	assert.False(t, manager.Verify(generated.Full+"x", generated.Hash))
	assert.False(t, manager.Verify(generated.Full, bytes.Repeat([]byte{0}, sha256DigestBytes)))
}

const sha256DigestBytes = 32

func TestLookupHashRejectsMalformedAndLegacyCredentials(t *testing.T) {
	manager, err := New("01234567890123456789012345678901")
	require.NoError(t, err)
	for _, value := range []string{"", " key ", "wrong.value", "kirby_pk_bad.bad", "kirby_pk_a.b.c"} {
		_, err := manager.LookupHash(value)
		assert.Error(t, err, value)
	}
}

func TestManagerFailsFastOnUnsafeDependencies(t *testing.T) {
	_, err := newManager([]byte("short"), bytes.NewReader(nil))
	assert.Error(t, err)

	manager, err := newManager(bytes.Repeat([]byte{'p'}, 32), errorReader{})
	require.NoError(t, err)
	_, err = manager.Generate()
	assert.Error(t, err)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("random failed") }

func TestCredentialsAreIndependentRandomSecretsAndPepperBound(t *testing.T) {
	firstManager, err := New("01234567890123456789012345678901")
	require.NoError(t, err)
	secondManager, err := New("different-installation-pepper-000000")
	require.NoError(t, err)
	first, err := firstManager.Generate()
	require.NoError(t, err)
	second, err := firstManager.Generate()
	require.NoError(t, err)
	require.NotEqual(t, first.Full, second.Full)
	require.NotEqual(t, first.Hash, second.Hash)
	require.False(t, firstManager.Verify(second.Full, first.Hash))
	require.False(t, secondManager.Verify(first.Full, first.Hash))
	legacy := "kirby_pk_" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16)) + "." + first.Full
	_, err = firstManager.LookupHash(legacy)
	require.Error(t, err)
}
