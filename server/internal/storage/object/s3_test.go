package object

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/config"
)

type fakeS3Object struct {
	data []byte
	info minio.ObjectInfo
}

type fakeS3 struct {
	presignURL    *url.URL
	presignFields map[string]string
	presignErr    error
	presignPolicy string
	statInfo      minio.ObjectInfo
	statErr       error
	openData      []byte
	openErr       error
	putErr        error
	removeErr     error
	bucketExists  bool
	bucketErr     error
	objects       map[string]fakeS3Object
	removedKeys   []string
	openedETag    string
	putKey        string
	presignCalls  int
	statCalls     int
	bucketCalls   int
}

func (fake *fakeS3) PresignedPostPolicy(_ context.Context, policy *minio.PostPolicy) (*url.URL, map[string]string, error) {
	fake.presignCalls++
	fake.presignPolicy = policy.String()
	return fake.presignURL, cloneStringMap(fake.presignFields), fake.presignErr
}

func (fake *fakeS3) StatObject(_ context.Context, _ string, key string, _ minio.StatObjectOptions) (minio.ObjectInfo, error) {
	fake.statCalls++
	if fake.statErr != nil {
		return minio.ObjectInfo{}, fake.statErr
	}
	if fake.objects != nil {
		stored, found := fake.objects[key]
		if !found {
			return minio.ObjectInfo{}, minio.ErrorResponse{Code: "NoSuchKey", StatusCode: http.StatusNotFound}
		}
		return stored.info, nil
	}
	return fake.statInfo, nil
}

func (fake *fakeS3) OpenObject(_ context.Context, _ string, key, etag string) (io.ReadCloser, error) {
	fake.openedETag = etag
	if fake.openErr != nil {
		return nil, fake.openErr
	}
	if fake.objects != nil {
		stored, found := fake.objects[key]
		if !found {
			return nil, minio.ErrorResponse{Code: "NoSuchKey", StatusCode: http.StatusNotFound}
		}
		if stored.info.ETag != etag {
			return nil, minio.ErrorResponse{Code: "PreconditionFailed", StatusCode: http.StatusPreconditionFailed}
		}
		return io.NopCloser(bytes.NewReader(stored.data)), nil
	}
	return io.NopCloser(bytes.NewReader(fake.openData)), nil
}

func (fake *fakeS3) PutObjectIfAbsent(
	_ context.Context,
	_ string,
	key string,
	source io.Reader,
	size int64,
	contentType string,
	metadata map[string]string,
) (minio.UploadInfo, error) {
	fake.putKey = key
	if fake.putErr != nil {
		return minio.UploadInfo{}, fake.putErr
	}
	if fake.objects == nil {
		fake.objects = make(map[string]fakeS3Object)
	}
	if _, exists := fake.objects[key]; exists {
		return minio.UploadInfo{}, minio.ErrorResponse{Code: "PreconditionFailed", StatusCode: http.StatusPreconditionFailed}
	}
	contents, err := io.ReadAll(source)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	if int64(len(contents)) != size {
		return minio.UploadInfo{}, errors.New("source size changed")
	}
	etag := "published-etag"
	lastModified := time.Date(2026, 8, 17, 10, 1, 0, 0, time.UTC)
	fake.objects[key] = fakeS3Object{data: contents, info: minio.ObjectInfo{
		Key:          key,
		ETag:         etag,
		Size:         size,
		ContentType:  contentType,
		UserMetadata: minio.StringMap(metadata),
		LastModified: lastModified,
	}}
	return minio.UploadInfo{Key: key, ETag: etag, LastModified: lastModified}, nil
}

func (fake *fakeS3) RemoveObject(_ context.Context, _ string, key string, _ minio.RemoveObjectOptions) error {
	fake.removedKeys = append(fake.removedKeys, key)
	if fake.removeErr != nil {
		return fake.removeErr
	}
	delete(fake.objects, key)
	return nil
}

func (fake *fakeS3) BucketExists(context.Context, string) (bool, error) {
	fake.bucketCalls++
	return fake.bucketExists, fake.bucketErr
}

func TestS3PostPolicyEnforcesProviderMaximum(t *testing.T) {
	uploadURL, err := url.Parse("https://objects.example.test/bucket")
	require.NoError(t, err)
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	temporaryKey, err := BuildUploadObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	presign := &fakeS3{
		presignURL: uploadURL,
		presignFields: map[string]string{
			"key":                                    temporaryKey,
			"Content-Type":                           "image/png",
			"x-amz-meta-kirby-declared-size":         "4",
			"x-amz-meta-kirby-declared-content-type": "image/png",
			"policy":                                 "signed-policy",
			"x-amz-signature":                        "storage-signature",
		},
	}
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	publicBaseURL, err := url.Parse("https://cdn.example.test/public/kirby")
	require.NoError(t, err)
	storage := newS3(&fakeS3{}, presign, "bucket", publicBaseURL, func() time.Time { return now })

	ticket, err := storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)
	assert.Equal(t, temporaryKey, ticket.Key)
	assert.Equal(t, UploadMethodPost, ticket.Method)
	assert.Empty(t, ticket.Headers)
	assert.Equal(t, uploadURL.String(), ticket.URL)
	assert.Equal(t, "signed-policy", ticket.FormFields["policy"])
	assert.Contains(t, presign.presignPolicy, `["content-length-range", 1, 67108864]`)
	assert.Contains(t, presign.presignPolicy, temporaryKey)
	assert.Contains(t, presign.presignPolicy, declaredSizeMetadata)
	assert.NotContains(t, strings.ToLower(presign.presignPolicy), "authorization")
	assert.NotContains(t, strings.ToLower(presign.presignPolicy), "x-kirby-api-key")
	assert.Equal(t, 1, presign.presignCalls)

	_, err = storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: MaxUploadSize + 1, ExpiresIn: time.Minute,
	})
	require.ErrorIs(t, err, ErrInvalidInput)
	assert.Equal(t, 1, presign.presignCalls)
}

func TestS3PostTicketNeverContainsManagementOrSecretCredentials(t *testing.T) {
	presignClient, err := minio.New("objects.example.test", &minio.Options{
		Creds:  credentials.NewStaticV4("storage-access", "storage-secret-value", ""),
		Secure: true,
		Region: "us-east-1",
	})
	require.NoError(t, err)
	publicBaseURL, err := url.Parse("https://cdn.example.test/kirby")
	require.NoError(t, err)
	storage := newS3(&fakeS3{}, presignClient, "bucket", publicBaseURL, time.Now)
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)

	ticket, err := storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.NoError(t, err)
	encoded := strings.ToLower(ticket.URL)
	for name, value := range ticket.FormFields {
		encoded += "\n" + strings.ToLower(name) + "=" + strings.ToLower(value)
	}
	assert.Contains(t, encoded, "storage-access")
	assert.NotContains(t, encoded, "storage-secret-value")
	assert.NotContains(t, encoded, "authorization")
	assert.NotContains(t, encoded, "cookie")
	assert.NotContains(t, encoded, "x-kirby-api-key")
}

func TestS3CompletePublishesImmutableObjectAndReplayConflicts(t *testing.T) {
	finalKey, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	temporaryKey, err := BuildUploadObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	internal := &fakeS3{objects: map[string]fakeS3Object{
		temporaryKey: uploadedObject("first", "first-etag"),
	}}
	publicBaseURL, err := url.Parse("https://cdn.example.test/public/kirby")
	require.NoError(t, err)
	storage := newS3(internal, &fakeS3{}, "bucket", publicBaseURL, time.Now)

	metadata, err := storage.CompleteUpload(context.Background(), temporaryKey)
	require.NoError(t, err)
	assert.Equal(t, finalKey, metadata.Key)
	assert.Equal(t, "https://cdn.example.test/public/kirby/"+finalKey, metadata.URL)
	assert.Equal(t, "first-etag", internal.openedETag)
	assert.Equal(t, []byte("first"), internal.objects[finalKey].data)
	_, temporaryExists := internal.objects[temporaryKey]
	assert.False(t, temporaryExists)

	internal.objects[temporaryKey] = uploadedObject("other", "second-etag")
	_, err = storage.CompleteUpload(context.Background(), temporaryKey)
	require.ErrorIs(t, err, ErrObjectConflict)
	assert.Equal(t, []byte("first"), internal.objects[finalKey].data)
	assert.Equal(t, []string{temporaryKey}, internal.removedKeys)
}

func uploadedObject(contents, etag string) fakeS3Object {
	return fakeS3Object{data: []byte(contents), info: minio.ObjectInfo{
		Size:        int64(len(contents)),
		ETag:        etag,
		ContentType: "image/png",
		UserMetadata: minio.StringMap{
			"Kirby-Declared-Size":         strconv.Itoa(len(contents)),
			"Kirby-Declared-Content-Type": "image/png",
		},
	}}
}

func TestS3CleanupFailureKeepsPublishedObject(t *testing.T) {
	finalKey, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	temporaryKey, err := BuildUploadObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	internal := &fakeS3{
		objects:   map[string]fakeS3Object{temporaryKey: uploadedObject("first", "first-etag")},
		removeErr: errors.New("temporary cleanup unavailable"),
	}
	publicBaseURL, err := url.Parse("https://cdn.example.test/kirby")
	require.NoError(t, err)
	storage := newS3(internal, &fakeS3{}, "bucket", publicBaseURL, time.Now)

	metadata, err := storage.CompleteUpload(context.Background(), temporaryKey)
	require.NoError(t, err)
	assert.Equal(t, finalKey, metadata.Key)
	assert.Equal(t, []byte("first"), internal.objects[finalKey].data)
	assert.Equal(t, []string{temporaryKey}, internal.removedKeys)
}

func TestMinioConditionalPublisherSendsIfNoneMatch(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHeader = request.Header.Get("If-None-Match")
		if _, err := io.Copy(io.Discard, request.Body); err != nil {
			http.Error(writer, "read failed", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("ETag", `"published"`)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4("storage-access", "storage-secret", ""),
		Secure:       false,
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
	})
	require.NoError(t, err)
	internal := &minioInternalClient{Client: client}
	_, err = internal.PutObjectIfAbsent(
		context.Background(), "bucket", "environments/1/projects/2/assets/object.png",
		strings.NewReader("data"), 4, "image/png", map[string]string{"test": "value"},
	)
	require.NoError(t, err)
	assert.Equal(t, "*", receivedHeader)
}

func TestNewS3RequiresAndSeparatesPublicAddresses(t *testing.T) {
	presignSSL := true
	storage, err := NewS3(config.S3Config{
		Endpoint:        "minio:9000",
		PresignEndpoint: "files.example.test",
		Region:          "us-east-1",
		Bucket:          "kirby",
		AccessKey:       config.NewSecret("access-key"),
		SecretKey:       config.NewSecret("secret-key"),
		UseSSL:          false,
		PresignUseSSL:   &presignSSL,
		PublicBaseURL:   "https://cdn.example.test/public/kirby/",
	})
	require.NoError(t, err)
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.test/public/kirby/"+key, storage.publicURL(key))
	assert.NotContains(t, storage.publicURL(key), "minio:9000")
	assert.NotContains(t, storage.publicURL(key), "files.example.test")
}

func TestS3BucketCheckUsesOnlyInternalClient(t *testing.T) {
	internal := &fakeS3{bucketExists: true}
	presign := &fakeS3{bucketExists: false}
	publicBaseURL, err := url.Parse("https://cdn.example.test/kirby")
	require.NoError(t, err)
	storage := newS3(internal, presign, "bucket", publicBaseURL, time.Now)

	exists, err := storage.bucketExists(context.Background())
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 1, internal.bucketCalls)
	assert.Zero(t, presign.bucketCalls)
}

func TestS3CompleteRejectsMetadataMismatch(t *testing.T) {
	temporaryKey, err := BuildUploadObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	fake := &fakeS3{statInfo: minio.ObjectInfo{
		Size:        5,
		ETag:        "etag",
		ContentType: "image/jpeg",
		UserMetadata: minio.StringMap{
			"Kirby-Declared-Size":         "4",
			"Kirby-Declared-Content-Type": "image/png",
		},
	}}
	publicBaseURL, err := url.Parse("https://cdn.example.test/kirby")
	require.NoError(t, err)
	storage := newS3(fake, fake, "bucket", publicBaseURL, time.Now)
	_, err = storage.CompleteUpload(context.Background(), temporaryKey)
	require.ErrorIs(t, err, ErrObjectIntegrity)
}

func TestS3ErrorsDoNotLeakEndpointOrSignedPolicy(t *testing.T) {
	key, err := BuildObjectKey(5, 6, testObjectID, ".png")
	require.NoError(t, err)
	leaking := errors.New("request failed https://internal-storage:9000/bucket/key?policy=secret")
	fake := &fakeS3{presignErr: leaking}
	publicBaseURL, err := url.Parse("https://cdn.example.test/kirby")
	require.NoError(t, err)
	storage := newS3(fake, fake, "bucket", publicBaseURL, time.Now)
	_, err = storage.PresignUpload(context.Background(), PresignUploadInput{
		Key: key, ContentType: "image/png", Size: 4, ExpiresIn: time.Minute,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStorageUnavailable)
	assert.NotContains(t, err.Error(), "internal-storage")
	assert.NotContains(t, strings.ToLower(err.Error()), "policy")
}

func TestS3DeleteIncompleteValidatesKey(t *testing.T) {
	fake := &fakeS3{}
	publicBaseURL, err := url.Parse("https://cdn.example.test/kirby")
	require.NoError(t, err)
	storage := newS3(fake, fake, "bucket", publicBaseURL, time.Now)
	err = storage.DeleteIncomplete(context.Background(), "../../forged")
	require.Error(t, err)
	assert.Empty(t, fake.removedKeys)
}
