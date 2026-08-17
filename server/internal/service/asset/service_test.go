package asset

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	assetlogic "github.com/yvvlee/kirby/server/internal/logic/asset"
	"github.com/yvvlee/kirby/server/internal/storage/object"
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

func newService(t *testing.T, storage *serviceStorage) *Service {
	t.Helper()
	logic, err := assetlogic.New(storage, allowAll{}, allowAll{})
	require.NoError(t, err)
	service, err := New(logic)
	require.NoError(t, err)
	return service
}

func TestServicePresignAndComplete(t *testing.T) {
	storage := &serviceStorage{}
	service := newService(t, storage)
	presign, err := service.PresignAsset(context.Background(), &adminv1.PresignAssetRequest{
		EnvironmentId: 1, ProjectId: 2, Filename: "logo.png", ContentType: "image/png", Size: 4,
	})
	require.NoError(t, err)
	assert.Equal(t, "image/png", presign.Headers["Content-Type"])
	assert.Equal(t, object.UploadMethodPut, presign.UploadMethod)
	assert.Equal(t, "2026-08-17T10:15:00Z", presign.ExpiresAt)

	storage.metadata = &object.Metadata{
		Key: presign.ObjectKey, URL: "https://objects.example.test/asset.png", ContentType: "image/png", Size: 4,
		DeclaredContentType: "image/png", DeclaredSize: 4,
	}
	completed, err := service.CompleteAsset(context.Background(), &adminv1.CompleteAssetRequest{
		EnvironmentId: 1, ProjectId: 2, ObjectKey: presign.ObjectKey,
	})
	require.NoError(t, err)
	assert.Equal(t, storage.metadata.URL, completed.Asset.Url)
	assert.Equal(t, uint64(4), completed.Asset.Size)
}

func TestServiceRejectsInvalidRequestWithoutLeakingValidationDetails(t *testing.T) {
	service := newService(t, &serviceStorage{})
	_, err := service.PresignAsset(context.Background(), &adminv1.PresignAssetRequest{})
	require.Error(t, err)
	kratosError := errors.FromError(err)
	assert.Equal(t, int32(400), kratosError.Code)
	assert.Equal(t, "InvalidParam", kratosError.Reason)
	assert.Equal(t, "asset request is invalid", kratosError.Message)
}

func TestServiceMapsImmutablePublishConflict(t *testing.T) {
	err := publicError(assetlogic.ErrAssetConflict)
	kratosError := errors.FromError(err)
	assert.Equal(t, int32(409), kratosError.Code)
	assert.Equal(t, "Conflict", kratosError.Reason)
}
