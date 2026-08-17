package exporter

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type SnapshotRepository interface {
	FindByID(context.Context, int64, int64) (*model.Snapshot, error)
}

type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}

type Logic struct {
	snapshots   SnapshotRepository
	permissions Authorizer
}

func New(snapshots SnapshotRepository, permissions Authorizer) (*Logic, error) {
	if snapshots == nil || permissions == nil {
		return nil, fmt.Errorf("snapshot export dependencies are incomplete")
	}
	return &Logic{snapshots: snapshots, permissions: permissions}, nil
}

func (l *Logic) Export(ctx context.Context, actor permission.Actor, sourceEnvironmentID, snapshotID int64) (*model.Snapshot, error) {
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

func RequireSourceRead(ctx context.Context, permissions Authorizer, userID, environmentID int64) error {
	return permissions.Require(ctx, userID, environmentID,
		permission.SnapshotExport, permission.SnapshotRead,
		permission.ConfigRead, permission.StructureRead, permission.EnumRead,
	)
}

// ValidateSnapshot decodes only the fixed snapshot contract and rebuilds its
// derived tree. No user, credential or storage records can enter this format.
func ValidateSnapshot(snapshot *model.Snapshot) (*entity.ConfigSnapshot, error) {
	if snapshot == nil {
		return nil, entity.Invalid("snapshot is missing")
	}
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
