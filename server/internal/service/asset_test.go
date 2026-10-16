package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra/object"
	"github.com/yvvlee/kirby/server/internal/logic"
)

type serviceStorage struct {
	metadata *object.Metadata
}

func (storage *serviceStorage) PresignUpload(_ context.Context, input object.PresignUploadInput) (*object.UploadTicket, error) {
	return &object.UploadTicket{
		Key:       input.Key,
		URL:       "https://upload.example.test/signed",
		Method:    object.UploadMethodPut,
		Headers:   map[string]string{"Content-Type": input.ContentType},
		ExpiresAt: time.Date(2026, 8, 17, 10, 15, 0, 0, time.UTC),
	}, nil
}

func (storage *serviceStorage) CompleteUpload(context.Context, string) (*object.Metadata, error) {
	return storage.metadata, nil
}

func (storage *serviceStorage) ReadMetadata(context.Context, string) (*object.Metadata, error) {
	return storage.metadata, nil
}

func (*serviceStorage) DeleteIncomplete(context.Context, string) error { return nil }
func (*serviceStorage) Close() error                                   { return nil }

type allowAll struct{}

func (allowAll) HasEnvironmentPermission(context.Context, int64, string) (bool, error) {
	return true, nil
}

func (allowAll) ProjectExists(context.Context, int64, int64) (bool, error) { return true, nil }

func newAssetTestService(t *testing.T, storage *serviceStorage) *AssetService {
	t.Helper()
	return newAssetTestServiceWithConfig(t, storage, &conf.Config{})
}

func newAssetTestServiceWithConfig(t *testing.T, storage *serviceStorage, cfg *conf.Config) *AssetService {
	t.Helper()
	logic, err := logic.NewAssetLogic(storage, allowAll{}, allowAll{})
	require.NoError(t, err)
	service, err := NewAssetService(cfg, logic)
	require.NoError(t, err)
	return service
}

func TestServicePresignAndComplete(t *testing.T) {
	storage := &serviceStorage{}
	service := newAssetTestService(t, storage)
	presign, err := service.PresignAsset(context.Background(), &adminv1.PresignAssetRequest{
		EnvironmentId: 1,
		ProjectId:     2,
		Filename:      "logo.png",
		ContentType:   "image/png",
		Size:          4,
	})
	require.NoError(t, err)
	assert.Equal(t, "image/png", presign.Headers["Content-Type"])
	assert.Equal(t, object.UploadMethodPut, presign.UploadMethod)
	assert.Equal(t, "2026-08-17T10:15:00Z", presign.ExpiresAt)

	storage.metadata = &object.Metadata{
		Key:                 presign.ObjectKey,
		URL:                 "https://objects.example.test/asset.png",
		ContentType:         "image/png",
		Size:                4,
		DeclaredContentType: "image/png",
		DeclaredSize:        4,
	}
	completed, err := service.CompleteAsset(context.Background(), &adminv1.CompleteAssetRequest{
		EnvironmentId: 1,
		ProjectId:     2,
		ObjectKey:     presign.ObjectKey,
	})
	require.NoError(t, err)
	assert.Equal(t, storage.metadata.URL, completed.Asset.Url)
	assert.Equal(t, uint64(4), completed.Asset.Size)
}

func TestS3AssetUsesPublicRequestAddress(t *testing.T) {
	key, err := object.BuildObjectKey(1, 2, "7a4c172a-0317-45c3-9638-f984ae15d552", ".png")
	require.NoError(t, err)
	storage := &serviceStorage{
		metadata: &object.Metadata{
			Key:         key,
			URL:         "/kirby/" + key,
			ContentType: "image/png",
			Size:        4,
		},
	}
	service := newAssetTestServiceWithConfig(t, storage, sameOriginS3Config())
	ctx, tr := requestContext(http.MethodPost, "https://unrelated.example.com", "")
	tr.request.Host = "kirby:8080"
	tr.request.TLS = nil
	tr.request.Header.Set("X-Forwarded-Proto", "https")
	tr.request.Header.Set("X-Forwarded-Host", "config.example.com:8443")
	completed, err := service.CompleteAsset(ctx, &adminv1.CompleteAssetRequest{
		EnvironmentId: 1,
		ProjectId:     2,
		ObjectKey:     key,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://config.example.com:8443/kirby/"+key, completed.Asset.Url)
	assert.Equal(t, "/kirby/"+key, storage.metadata.URL)
}

func TestS3AssetRejectsInvalidPublicAddressBeforePublishing(t *testing.T) {
	service := newAssetTestServiceWithConfig(t, &serviceStorage{}, sameOriginS3Config())
	ctx, tr := requestContext(http.MethodPost, "", "")
	tr.request.Header.Set("X-Forwarded-Host", "first.example.com,second.example.com")
	_, err := service.CompleteAsset(ctx, &adminv1.CompleteAssetRequest{})
	require.Error(t, err)
	assert.Equal(t, int32(400), errors.FromError(err).Code)
}

func sameOriginS3Config() *conf.Config {
	return &conf.Config{
		ObjectStorage: conf.ObjectStorageConfig{Driver: "s3"},
	}
}

func TestServiceRejectsInvalidRequestWithoutLeakingValidationDetails(t *testing.T) {
	service := newAssetTestService(t, &serviceStorage{})
	_, err := service.PresignAsset(context.Background(), &adminv1.PresignAssetRequest{})
	require.Error(t, err)
	kratosError := errors.FromError(err)
	assert.Equal(t, int32(400), kratosError.Code)
	assert.Equal(t, "InvalidParam", kratosError.Reason)
	assert.Equal(t, "asset request is invalid", kratosError.Message)
}

func TestServiceMapsImmutablePublishConflict(t *testing.T) {
	err := assetPublicError(logic.ErrAssetConflict)
	kratosError := errors.FromError(err)
	assert.Equal(t, int32(409), kratosError.Code)
	assert.Equal(t, "Conflict", kratosError.Reason)
}
