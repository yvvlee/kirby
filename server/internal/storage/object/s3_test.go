package object

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeS3 struct {
	presignURL     *url.URL
	presignErr     error
	statInfo       minio.ObjectInfo
	statErr        error
	removeErr      error
	bucketExists   bool
	bucketErr      error
	presignHeaders http.Header
	removedKey     string
}

func (fake *fakeS3) PresignHeader(_ context.Context, _, _, _ string, _ time.Duration, _ url.Values, headers http.Header) (*url.URL, error) {
	fake.presignHeaders = headers.Clone()
	return fake.presignURL, fake.presignErr
}

func (fake *fakeS3) StatObject(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return fake.statInfo, fake.statErr
}

func (fake *fakeS3) RemoveObject(_ context.Context, _ string, key string, _ minio.RemoveObjectOptions) error {
	fake.removedKey = key
	return fake.removeErr
}

func (fake *fakeS3) BucketExists(context.Context, string) (bool, error) {
	return fake.bucketExists, fake.bucketErr
}

func TestS3PresignAndComplete(t *testing.T) {
	uploadURL, err := url.Parse("https://objects.example.test/bucket/key?X-Amz-Signature=secret-signature")
	require.NoError(t, err)
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	fake := &fakeS3{
		presignURL: uploadURL,
		statInfo: minio.ObjectInfo{
			Size:        4,
			ContentType: "image/png",
			UserMetadata: minio.StringMap{
				"Kirby-Declared-Size":         "4",
				"Kirby-Declared-Content-Type": "image/png",
			},
		},
	}
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	storage := newS3(fake, "bucket", "objects.example.test", "https", func() time.Time { return now })

	ticket, err := storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)
	assert.Equal(t, uploadURL.String(), ticket.URL)
	assert.Equal(t, "image/png", ticket.Headers["Content-Type"])
	assert.Equal(t, "4", ticket.Headers[declaredSizeHeader])
	assert.Empty(t, ticket.Headers["Authorization"])
	assert.Empty(t, ticket.Headers["Cookie"])
	assert.Empty(t, ticket.Headers["X-Kirby-API-Key"])
	assert.Equal(t, "image/png", fake.presignHeaders.Get(declaredContentTypeHeader))

	metadata, err := storage.CompleteUpload(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, uint64(4), metadata.Size)
	assert.Equal(t, "https://objects.example.test/bucket/"+key, metadata.URL)
}

func TestS3CompleteRejectsMetadataMismatch(t *testing.T) {
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	fake := &fakeS3{statInfo: minio.ObjectInfo{
		Size:        5,
		ContentType: "image/jpeg",
		UserMetadata: minio.StringMap{
			"Kirby-Declared-Size":         "4",
			"Kirby-Declared-Content-Type": "image/png",
		},
	}}
	storage := newS3(fake, "bucket", "objects.example.test", "https", time.Now)
	_, err = storage.CompleteUpload(context.Background(), key)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrObjectIntegrity))
}

func TestS3ErrorsDoNotLeakEndpointOrSignedURL(t *testing.T) {
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	leaking := errors.New("request failed https://internal-storage:9000/bucket/key?X-Amz-Signature=secret")
	fake := &fakeS3{presignErr: leaking}
	storage := newS3(fake, "bucket", "internal-storage:9000", "http", time.Now)
	_, err = storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrStorageUnavailable))
	assert.NotContains(t, err.Error(), "internal-storage")
	assert.NotContains(t, strings.ToLower(err.Error()), "signature")
}

func TestS3DeleteIncompleteValidatesKey(t *testing.T) {
	fake := &fakeS3{}
	storage := newS3(fake, "bucket", "objects.example.test", "https", time.Now)
	err := storage.DeleteIncomplete(context.Background(), "../../forged")
	require.Error(t, err)
	assert.Empty(t, fake.removedKey)
}
