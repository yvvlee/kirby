package asset

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/storage/object"
)

const deterministicID = "123e4567-e89b-12d3-a456-426614174000"

type fakeStorage struct {
	presignInput  object.PresignUploadInput
	presignTicket *object.UploadTicket
	presignErr    error
	metadata      *object.Metadata
	completeErr   error
	completeCalls int
	deletedKey    string
	deleteErr     error
}

func (fake *fakeStorage) PresignUpload(_ context.Context, input object.PresignUploadInput) (*object.UploadTicket, error) {
	fake.presignInput = input
	if fake.presignTicket == nil {
		fake.presignTicket = &object.UploadTicket{Key: input.Key, URL: "https://upload.example.test", Headers: map[string]string{"Content-Type": input.ContentType}}
	}
	return fake.presignTicket, fake.presignErr
}

func (fake *fakeStorage) CompleteUpload(context.Context, string) (*object.Metadata, error) {
	fake.completeCalls++
	return fake.metadata, fake.completeErr
}

func (fake *fakeStorage) ReadMetadata(context.Context, string) (*object.Metadata, error) {
	return fake.metadata, nil
}

func (fake *fakeStorage) DeleteIncomplete(_ context.Context, key string) error {
	fake.deletedKey = key
	return fake.deleteErr
}

func (*fakeStorage) Close() error { return nil }

type fakeAuthorizer struct {
	allowed     bool
	err         error
	permission  string
	environment int64
}

func (fake *fakeAuthorizer) HasEnvironmentPermission(_ context.Context, environmentID int64, permission string) (bool, error) {
	fake.environment = environmentID
	fake.permission = permission
	return fake.allowed, fake.err
}

type fakeProjects struct {
	exists      bool
	err         error
	environment int64
	project     int64
}

func (fake *fakeProjects) ProjectExists(_ context.Context, environmentID, projectID int64) (bool, error) {
	fake.environment = environmentID
	fake.project = projectID
	return fake.exists, fake.err
}

func newTestLogic(t *testing.T, storage *fakeStorage, authorizer *fakeAuthorizer, projects *fakeProjects) *Logic {
	t.Helper()
	logic, err := newLogic(storage, authorizer, projects, DefaultPolicy(), func() (string, error) { return deterministicID, nil })
	require.NoError(t, err)
	return logic
}

func TestPresignScopesKeyAndChecksPermission(t *testing.T) {
	storage := &fakeStorage{}
	authorizer := &fakeAuthorizer{allowed: true}
	projects := &fakeProjects{exists: true}
	logic := newTestLogic(t, storage, authorizer, projects)

	ticket, err := logic.Presign(context.Background(), PresignInput{
		EnvironmentID: 10,
		ProjectID:     20,
		Filename:      "logo.PNG",
		ContentType:   "image/png",
		Size:          128,
	})
	require.NoError(t, err)
	assert.Equal(t, "environments/10/projects/20/assets/"+deterministicID+".png", ticket.Key)
	assert.Equal(t, PermissionAssetWrite, authorizer.permission)
	assert.Equal(t, int64(10), authorizer.environment)
	assert.Equal(t, int64(20), projects.project)
	assert.Equal(t, uint64(128), storage.presignInput.Size)
	assert.Equal(t, object.DefaultUploadTTL, storage.presignInput.ExpiresIn)
}

func TestPresignRejectsTraversalOversizeAndMIMEMismatchBeforeDependencies(t *testing.T) {
	authorizer := &fakeAuthorizer{allowed: true}
	projects := &fakeProjects{exists: true}
	logic := newTestLogic(t, &fakeStorage{}, authorizer, projects)
	tests := []PresignInput{
		{EnvironmentID: 1, ProjectID: 2, Filename: "../logo.png", ContentType: "image/png", Size: 4},
		{EnvironmentID: 1, ProjectID: 2, Filename: "logo.png", ContentType: "image/jpeg", Size: 4},
		{EnvironmentID: 1, ProjectID: 2, Filename: "logo.png", ContentType: "image/png", Size: DefaultMaxSize + 1},
		{EnvironmentID: 1, ProjectID: 2, Filename: "payload.exe", ContentType: "application/octet-stream", Size: 4},
	}
	for _, input := range tests {
		_, err := logic.Presign(context.Background(), input)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidInput))
	}
	assert.Empty(t, authorizer.permission)
	assert.Zero(t, projects.project)
}

func TestPresignRejectsMissingPermissionAndWrongProjectScope(t *testing.T) {
	base := PresignInput{EnvironmentID: 1, ProjectID: 2, Filename: "logo.png", ContentType: "image/png", Size: 4}

	logic := newTestLogic(t, &fakeStorage{}, &fakeAuthorizer{allowed: false}, &fakeProjects{exists: true})
	_, err := logic.Presign(context.Background(), base)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrForbidden))

	logic = newTestLogic(t, &fakeStorage{}, &fakeAuthorizer{allowed: true}, &fakeProjects{exists: false})
	_, err = logic.Presign(context.Background(), base)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProjectNotFound))
}

func TestCompleteRejectsForgedAndCrossEnvironmentKeysBeforeStorage(t *testing.T) {
	storage := &fakeStorage{}
	logic := newTestLogic(t, storage, &fakeAuthorizer{allowed: true}, &fakeProjects{exists: true})

	_, err := logic.Complete(context.Background(), 1, 2, "../../forged")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidInput))

	key, err := object.BuildObjectKey(9, 2, deterministicID, ".png")
	require.NoError(t, err)
	_, err = logic.Complete(context.Background(), 1, 2, key)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrForbidden))
	assert.Zero(t, storage.completeCalls)
}

func TestCompleteRevalidatesSizeAndMIMEAndDeletesInvalidObject(t *testing.T) {
	key, err := object.BuildObjectKey(1, 2, deterministicID, ".png")
	require.NoError(t, err)
	storage := &fakeStorage{metadata: &object.Metadata{
		Key:                 key,
		URL:                 "https://objects.example.test/" + key,
		ContentType:         "image/jpeg",
		Size:                4,
		DeclaredContentType: "image/jpeg",
		DeclaredSize:        4,
		LastModified:        time.Now(),
	}}
	logic := newTestLogic(t, storage, &fakeAuthorizer{allowed: true}, &fakeProjects{exists: true})

	_, err = logic.Complete(context.Background(), 1, 2, key)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAssetIntegrity))
	assert.Equal(t, key, storage.deletedKey)

	storage.metadata = nil
	storage.completeErr = object.ErrObjectIntegrity
	storage.deletedKey = ""
	_, err = logic.Complete(context.Background(), 1, 2, key)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAssetIntegrity))
	assert.Equal(t, key, storage.deletedKey)
}

func TestCompleteReturnsValidatedMetadata(t *testing.T) {
	key, err := object.BuildObjectKey(1, 2, deterministicID, ".png")
	require.NoError(t, err)
	want := &object.Metadata{Key: key, URL: "https://objects.example.test/" + key, ContentType: "image/png", Size: 4, DeclaredContentType: "image/png", DeclaredSize: 4}
	logic := newTestLogic(t, &fakeStorage{metadata: want}, &fakeAuthorizer{allowed: true}, &fakeProjects{exists: true})

	got, err := logic.Complete(context.Background(), 1, 2, key)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
