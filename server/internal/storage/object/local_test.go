package object

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/config"
)

func TestLocalDirectUploadAndComplete(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	storage, err := newLocal(t.TempDir(), bytes.Repeat([]byte{7}, 32), func() time.Time { return now })
	require.NoError(t, err)
	key, err := BuildObjectKey(1, 2, testObjectID, ".png")
	require.NoError(t, err)

	ticket, err := storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Content-Type": "image/png"}, ticket.Headers)
	assert.NotContains(t, strings.ToLower(ticket.URL), "authorization")
	assert.NotContains(t, strings.ToLower(ticket.URL), "api-key")

	request := httptest.NewRequest(http.MethodPut, ticket.URL, bytes.NewReader([]byte("data")))
	request.Header.Set("Content-Type", ticket.Headers["Content-Type"])
	response := httptest.NewRecorder()
	storage.ServeHTTP(response, request)
	assert.Equal(t, http.StatusNoContent, response.Code)

	metadata, err := storage.CompleteUpload(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, uint64(4), metadata.Size)
	assert.Equal(t, uint64(4), metadata.DeclaredSize)
	assert.Equal(t, "image/png", metadata.ContentType)
	assert.Equal(t, LocalObjectPathPrefix+key, metadata.URL)

	getRequest := httptest.NewRequest(http.MethodGet, metadata.URL, nil)
	getResponse := httptest.NewRecorder()
	storage.ServeHTTP(getResponse, getRequest)
	assert.Equal(t, http.StatusOK, getResponse.Code)
	assert.Equal(t, "data", getResponse.Body.String())
	assert.Equal(t, "nosniff", getResponse.Header().Get("X-Content-Type-Options"))
}

func TestLocalUploadRejectsCredentialForwardingAndMismatches(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	storage, err := newLocal(t.TempDir(), bytes.Repeat([]byte{8}, 32), func() time.Time { return now })
	require.NoError(t, err)
	key, err := BuildObjectKey(1, 2, testObjectID, ".png")
	require.NoError(t, err)
	ticket, err := storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)

	tests := []struct {
		name   string
		body   string
		type_  string
		header string
	}{
		{name: "wrong size", body: "too-long", type_: "image/png"},
		{name: "wrong MIME", body: "data", type_: "image/jpeg"},
		{name: "JWT forwarded", body: "data", type_: "image/png", header: "Authorization"},
		{name: "API key forwarded", body: "data", type_: "image/png", header: "X-Kirby-API-Key"},
		{name: "Cookie forwarded", body: "data", type_: "image/png", header: "Cookie"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, ticket.URL, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.type_)
			if test.header != "" {
				request.Header.Set(test.header, "must-not-cross-boundary")
			}
			response := httptest.NewRecorder()
			storage.ServeHTTP(response, request)
			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestLocalUploadTicketExpiresAndCannotBeTampered(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	storage, err := newLocal(t.TempDir(), bytes.Repeat([]byte{9}, 32), func() time.Time { return now })
	require.NoError(t, err)
	key, err := BuildObjectKey(1, 2, testObjectID, ".png")
	require.NoError(t, err)
	ticket, err := storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)

	tampered, err := url.Parse(ticket.URL)
	require.NoError(t, err)
	query := tampered.Query()
	query.Set("token", query.Get("token")+"x")
	tampered.RawQuery = query.Encode()
	for _, target := range []string{tampered.String(), ticket.URL} {
		if target == ticket.URL {
			now = now.Add(time.Minute)
		}
		request := httptest.NewRequest(http.MethodPut, target, strings.NewReader("data"))
		request.Header.Set("Content-Type", "image/png")
		response := httptest.NewRecorder()
		storage.ServeHTTP(response, request)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
	}
}

func TestLocalStorageRejectsSymlinkAndTraversal(t *testing.T) {
	root := t.TempDir()
	storage, err := newLocal(root, bytes.Repeat([]byte{5}, 32), time.Now)
	require.NoError(t, err)
	_, err = storage.ReadMetadata(context.Background(), "../../outside.png")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidInput))

	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(root, "environments")))
	key, err := BuildObjectKey(1, 2, testObjectID, ".png")
	require.NoError(t, err)
	_, err = storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)
	claims := localUploadClaims{Key: key, ContentType: "image/png", Size: 4, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	err = storage.writeUpload(context.Background(), claims, strings.NewReader("data"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectIntegrity))
}

func TestOpenRejectsLocalStorageInMultiInstanceMode(t *testing.T) {
	storage, err := Open(context.Background(), config.ModeMulti, config.ObjectStorageConfig{
		Driver: "local", Local: config.LocalConfig{Directory: t.TempDir()},
	})
	require.Error(t, err)
	assert.Nil(t, storage)
	assert.Contains(t, err.Error(), "mode=multi")
}
