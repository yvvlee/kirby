package publish

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type ConfigRepository interface {
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
}

type StructureRepository interface {
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error)
}

type EnumRepository interface {
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error)
}

type SnapshotReader interface {
	FindByID(context.Context, int64, int64) (*model.Snapshot, error)
}

type Repository interface {
	LockForConfig(context.Context, *xorm.Session, int64, int64) ([]model.Snapshot, error)
	SetReleased(context.Context, *xorm.Session, int64, int64, int64, int64, int64, time.Time) error
	SetUnreleased(context.Context, *xorm.Session, int64, int64, int64, int64, int64, time.Time) error
	IncrementRuntimeVersion(context.Context, *xorm.Session, int64, int64) error
}

type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}

type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

// CacheCleaner removes only an obsolete versioned entry. Runtime correctness
// comes from configs.runtime_version, so cleanup errors must not undo commits.
type CacheCleaner interface {
	DeletePublishedConfigVersion(context.Context, int64, string, int64) error
}

type Logic struct {
	configs      ConfigRepository
	structures   StructureRepository
	enums        EnumRepository
	snapshots    SnapshotReader
	publications Repository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
	cache        CacheCleaner
	now          func() time.Time
}

func New(configs ConfigRepository, structures StructureRepository, enums EnumRepository, snapshots SnapshotReader, publications Repository, permissions Authorizer, audits AuditRepository, transactions database.Transactor, cache CacheCleaner) (*Logic, error) {
	if configs == nil || structures == nil || enums == nil || snapshots == nil || publications == nil || permissions == nil || audits == nil || transactions == nil || cache == nil {
		return nil, fmt.Errorf("publish logic dependencies are incomplete")
	}
	return &Logic{
		configs: configs, structures: structures, enums: enums, snapshots: snapshots,
		publications: publications, permissions: permissions, audits: audits,
		transactions: transactions, cache: cache, now: time.Now,
	}, nil
}

func (l *Logic) Publish(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64, expectedVersion uint32) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotPublish); err != nil {
		return nil, err
	}
	found, err := l.snapshots.FindByID(ctx, environmentID, snapshotID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("snapshot reader returned nil snapshot")
	}
	var result *model.Snapshot
	var cleanup cleanupKey
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		config, snapshots, err := l.lockResources(ctx, tx, environmentID, found.ConfigID)
		if err != nil {
			return err
		}
		target, released, err := publicationState(snapshots, snapshotID)
		if err != nil {
			return err
		}
		if target.Status == model.SnapshotStatusReleased {
			return entity.Conflict("snapshot is already released")
		}
		if target.Version != int64(expectedVersion) {
			return entity.Conflict("snapshot version is stale")
		}
		if target.ConfigID != config.ID || target.ProjectID != config.ProjectID || target.ConfigKey != config.Key {
			return entity.Conflict("snapshot does not belong to config")
		}
		if config.RuntimeVersion < 0 {
			return entity.Conflict("config runtime version is invalid")
		}
		if config.RuntimeVersion == math.MaxInt64 {
			return entity.Conflict("config runtime version is exhausted")
		}
		if err := validateSnapshot(target, config); err != nil {
			return err
		}

		changedAt := l.now().UTC()
		if released != nil {
			if released.Version == math.MaxInt64 {
				return entity.Conflict("released snapshot version is exhausted")
			}
			if err := l.publications.SetUnreleased(ctx, tx, environmentID, config.ID, released.ID, released.Version, actor.UserID, changedAt); err != nil {
				return err
			}
		}
		if err := l.publications.SetReleased(ctx, tx, environmentID, config.ID, target.ID, target.Version, actor.UserID, changedAt); err != nil {
			return err
		}
		if err := l.publications.IncrementRuntimeVersion(ctx, tx, environmentID, config.ID); err != nil {
			return err
		}
		if err := l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "snapshot.publish", target.ID)); err != nil {
			return err
		}

		copy := *target
		copy.Status = model.SnapshotStatusReleased
		copy.PublishedAt = &changedAt
		publisher := actor.UserID
		copy.PublishedBy = &publisher
		copy.UpdatedBy = actor.UserID
		copy.UpdatedAt = changedAt
		copy.Version++
		result = &copy
		cleanup = cleanupKey{projectID: config.ProjectID, configKey: config.Key, runtimeVersion: config.RuntimeVersion}
		return nil
	})
	if err != nil {
		return nil, err
	}
	l.cleanup(ctx, cleanup)
	return result, nil
}

func (l *Logic) Unpublish(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64, expectedVersion uint32) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotPublish); err != nil {
		return nil, err
	}
	found, err := l.snapshots.FindByID(ctx, environmentID, snapshotID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("snapshot reader returned nil snapshot")
	}
	var result *model.Snapshot
	var cleanup cleanupKey
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		config, snapshots, err := l.lockResources(ctx, tx, environmentID, found.ConfigID)
		if err != nil {
			return err
		}
		target, _, err := publicationState(snapshots, snapshotID)
		if err != nil {
			return err
		}
		if target.Status == model.SnapshotStatusUnreleased {
			return entity.Conflict("snapshot is already unreleased")
		}
		if target.Version != int64(expectedVersion) {
			return entity.Conflict("snapshot version is stale")
		}
		if target.ConfigID != config.ID || target.ProjectID != config.ProjectID || target.ConfigKey != config.Key {
			return entity.Conflict("snapshot does not belong to config")
		}
		if config.RuntimeVersion < 0 {
			return entity.Conflict("config runtime version is invalid")
		}
		if config.RuntimeVersion == math.MaxInt64 {
			return entity.Conflict("config runtime version is exhausted")
		}

		changedAt := l.now().UTC()
		if err := l.publications.SetUnreleased(ctx, tx, environmentID, config.ID, target.ID, target.Version, actor.UserID, changedAt); err != nil {
			return err
		}
		if err := l.publications.IncrementRuntimeVersion(ctx, tx, environmentID, config.ID); err != nil {
			return err
		}
		if err := l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "snapshot.unpublish", target.ID)); err != nil {
			return err
		}

		copy := *target
		copy.Status = model.SnapshotStatusUnreleased
		copy.PublishedAt = nil
		copy.PublishedBy = nil
		copy.UpdatedBy = actor.UserID
		copy.UpdatedAt = changedAt
		copy.Version++
		result = &copy
		cleanup = cleanupKey{projectID: config.ProjectID, configKey: config.Key, runtimeVersion: config.RuntimeVersion}
		return nil
	})
	if err != nil {
		return nil, err
	}
	l.cleanup(ctx, cleanup)
	return result, nil
}

func (l *Logic) lockResources(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*model.Config, []model.Snapshot, error) {
	config, err := l.configs.LockByID(ctx, tx, environmentID, configID)
	if err != nil {
		return nil, nil, err
	}
	if config == nil {
		return nil, nil, fmt.Errorf("config repository returned nil config")
	}
	if _, err := l.structures.ListForConfigTx(ctx, tx, environmentID, configID); err != nil {
		return nil, nil, err
	}
	if _, err := l.enums.ListForConfigTx(ctx, tx, environmentID, configID); err != nil {
		return nil, nil, err
	}
	snapshots, err := l.publications.LockForConfig(ctx, tx, environmentID, configID)
	if err != nil {
		return nil, nil, err
	}
	return config, snapshots, nil
}

func publicationState(snapshots []model.Snapshot, targetID int64) (*model.Snapshot, *model.Snapshot, error) {
	var target *model.Snapshot
	var released *model.Snapshot
	for index := range snapshots {
		item := &snapshots[index]
		switch item.Status {
		case model.SnapshotStatusUnreleased:
		case model.SnapshotStatusReleased:
			if released != nil {
				return nil, nil, entity.Conflict("config has multiple released snapshots")
			}
			released = item
		default:
			return nil, nil, entity.Conflict("snapshot has unsupported status")
		}
		if item.ID == targetID {
			target = item
		}
	}
	if target == nil {
		return nil, nil, base.Missing("snapshot")
	}
	return target, released, nil
}

func validateSnapshot(snapshot *model.Snapshot, config *model.Config) error {
	content, err := entity.DecodeConfigSnapshot(snapshot.Content)
	if err != nil {
		return entity.Invalid("snapshot content is invalid")
	}
	if content == nil || content.Config == nil || content.Config.Type == nil || config == nil {
		return entity.Invalid("snapshot content is incomplete")
	}
	if content.Config.Id != config.ID || content.Config.ProjectId != config.ProjectID || content.Config.Key != config.Key {
		return entity.Conflict("snapshot content does not belong to config")
	}
	for _, item := range content.Structures {
		if item == nil || item.ConfigId != config.ID {
			return entity.Invalid("snapshot structure belongs to another config")
		}
	}
	for _, item := range content.Enums {
		if item == nil || item.ConfigId != config.ID {
			return entity.Invalid("snapshot enum belongs to another config")
		}
	}
	schema, err := entity.NewSchema(content.Structures, content.Enums)
	if err != nil {
		return err
	}
	return schema.ValidateConfig(content.Config)
}

type cleanupKey struct {
	projectID      int64
	configKey      string
	runtimeVersion int64
}

func (l *Logic) cleanup(ctx context.Context, key cleanupKey) {
	_ = l.cache.DeletePublishedConfigVersion(ctx, key.projectID, key.configKey, key.runtimeVersion)
}

func audit(actor permission.Actor, action string, snapshotID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID: &actorID, Action: action, ResourceType: "snapshot",
		ResourceID: strconv.FormatInt(snapshotID, 10), Result: model.AuditResultSucceeded,
		RequestID: actor.RequestID,
	}
}
