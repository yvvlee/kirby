package exporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type snapshotFake struct{ item *model.Snapshot }

func (f snapshotFake) FindByID(context.Context, int64, int64) (*model.Snapshot, error) {
	return f.item, nil
}

type permissionFake struct {
	required []string
	err      error
}

func (f *permissionFake) Require(_ context.Context, _, _ int64, required ...string) error {
	f.required = append(f.required, required...)
	return f.err
}

func exportSnapshot(t *testing.T) *model.Snapshot {
	t.Helper()
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	content, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config: &commonv1.Config{Id: 10, ProjectId: 11, Key: "feature", Type: fieldType, Value: `"on"`},
		Tree:   &commonv1.TreeNode{Value: &commonv1.Field{Key: "tampered", Name: "Tampered", Type: fieldType}},
	})
	require.NoError(t, err)
	return &model.Snapshot{Meta: model.Meta{ID: 12}, ProjectID: 11, ConfigID: 10, ConfigKey: "feature", Content: content}
}

func TestExportRequiresAllSourceReadPermissionsAndRebuildsTree(t *testing.T) {
	permissions := &permissionFake{}
	logicLayer, err := New(snapshotFake{item: exportSnapshot(t)}, permissions)
	require.NoError(t, err)

	result, err := logicLayer.Export(context.Background(), permission.Actor{UserID: 9}, 1, 12)
	require.NoError(t, err)
	assert.Equal(t, []string{permission.SnapshotExport, permission.SnapshotRead, permission.ConfigRead, permission.StructureRead, permission.EnumRead}, permissions.required)
	decoded, err := entity.DecodeConfigSnapshot(result.Content)
	require.NoError(t, err)
	assert.Equal(t, "feature", decoded.Tree.Value.Key)
}

func TestValidateSnapshotRejectsMismatchedEmbeddedScope(t *testing.T) {
	snapshot := exportSnapshot(t)
	snapshot.ConfigID = 999
	_, err := ValidateSnapshot(snapshot)
	assert.ErrorIs(t, err, entity.ErrConflict)
}

func TestExportStopsBeforeReadingWhenPermissionIsDenied(t *testing.T) {
	permissions := &permissionFake{err: permission.ErrForbidden}
	logicLayer, err := New(snapshotFake{item: exportSnapshot(t)}, permissions)
	require.NoError(t, err)
	_, err = logicLayer.Export(context.Background(), permission.Actor{UserID: 9}, 1, 12)
	assert.ErrorIs(t, err, permission.ErrForbidden)
}

func TestExportRejectsFieldsOutsideFixedSnapshotContract(t *testing.T) {
	snapshot := exportSnapshot(t)
	snapshot.Content = `{"config":{},"structures":[],"enums":[],"tree":{},"api_key":"secret"}`
	_, err := ValidateSnapshot(snapshot)
	assert.Error(t, err)
}
