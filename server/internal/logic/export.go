package logic

import (
	"context"
	"unicode/utf8"

	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type ExportLogic struct {
	snapshots   repository.SnapshotRepository
	permissions environmentPermissionChecker
}

// NewExportLogic creates ExportLogic with its configuration dependencies.
//
// Parameters: snapshots reads and writes snapshot rows; permissions checks the caller permissions.
// Returns: *ExportLogic.
// Throws: none.
func NewExportLogic(
	snapshots repository.SnapshotRepository,
	permissions environmentPermissionChecker,
) *ExportLogic {
	return &ExportLogic{
		snapshots:   snapshots,
		permissions: permissions,
	}
}

// Export returns validated snapshot content after checking source permissions.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; sourceEnvironmentID selects the source environment; snapshotID selects the snapshot.
// Returns: the validated source snapshot, or a source-permission, missing-resource, or content-validation error.
// Throws: none.
func (l *ExportLogic) Export(ctx context.Context, actor permission.Actor, sourceEnvironmentID, snapshotID int64) (*model.Snapshot, error) {
	if err := RequireSourceRead(ctx, l.permissions, actor.UserID, sourceEnvironmentID); err != nil {
		return nil, err
	}
	snapshot, err := l.snapshots.FindByID(ctx, sourceEnvironmentID, snapshotID)
	if err != nil {
		return nil, err
	}
	content, err := ValidateSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	canonical, err := entity.EncodeConfigSnapshot(content)
	if err != nil {
		return nil, err
	}
	result := *snapshot
	result.Content = canonical
	return &result, nil
}

// RequireSourceRead requires every permission needed to expose snapshot content.
//
// Parameters: ctx carries cancellation and transaction state; permissions checks the caller permissions; userID identifies the caller; environmentID selects the authorized environment.
// Returns: nil when all five read/export permissions are granted, otherwise the permission-check error.
// Throws: none.
func RequireSourceRead(ctx context.Context, permissions environmentPermissionChecker, userID, environmentID int64) error {
	return permissions.Require(ctx, userID, environmentID,
		permission.SnapshotExport, permission.SnapshotRead,
		permission.ConfigRead, permission.StructureRead, permission.EnumRead,
	)
}

// ValidateSnapshot decodes only the fixed snapshot contract and rebuilds its
// derived tree. No user, credential or storage records can enter this format.
//
// Parameters: snapshot contains the persisted scope and serialized configuration content.
// Returns: decoded content with its derived tree rebuilt, or an error for malformed content, mismatched scope, or an invalid schema.
// Throws: none.
func ValidateSnapshot(snapshot *model.Snapshot) (*entity.ConfigSnapshot, error) {
	content, err := entity.DecodeConfigSnapshot(snapshot.Content)
	if err != nil {
		return nil, err
	}
	if content.Config.GetId() != snapshot.ConfigID || content.Config.GetProjectId() != snapshot.ProjectID || content.Config.GetKey() != snapshot.ConfigKey {
		return nil, entity.Conflict("snapshot content scope does not match snapshot")
	}
	if utf8.RuneCountInString(content.Config.Description) > 255 {
		return nil, entity.Invalid("snapshot config description is invalid")
	}
	for _, item := range content.Structures {
		if item == nil || item.ConfigId != snapshot.ConfigID {
			return nil, entity.Invalid("snapshot structure belongs to another config")
		}
	}
	for _, item := range content.Enums {
		if item == nil || item.ConfigId != snapshot.ConfigID {
			return nil, entity.Invalid("snapshot enum belongs to another config")
		}
	}
	schema, err := entity.NewSchema(content.Structures, content.Enums)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateConfig(content.Config); err != nil {
		return nil, err
	}
	tree, err := schema.BuildTree(content.Config)
	if err != nil {
		return nil, err
	}
	content.Tree = tree
	return content, nil
}
