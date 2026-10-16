package logic

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

// publishedContentCleaner removes only an obsolete versioned entry. Runtime correctness
// comes from configs.runtime_version, so cleanup errors must not undo commits.
type publishedContentCleaner interface {
	DeletePublishedConfigVersion(ctx context.Context, projectID int64, configKey string, runtimeVersion int64) error
}

type PublishLogic struct {
	logger       *infra.Logger
	configs      repository.ConfigRepository
	structures   repository.StructureRepository
	enums        repository.ConfigEnumRepository
	snapshots    repository.SnapshotRepository
	publications repository.SnapshotPublicationRepository
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
	cache        publishedContentCleaner
	now          func() time.Time
}

// NewPublishLogic creates PublishLogic with its configuration dependencies.
//
// Parameters: configs reads and writes configuration rows; structures locks and reconciles structure definitions; enums locks and reconciles enum definitions; snapshots reads and writes snapshot rows; publications guards snapshot publication transitions; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together; cache removes stale published-content cache entries; logger records operational failures.
// Returns: *PublishLogic.
// Throws: none.
func NewPublishLogic(
	configs repository.ConfigRepository,
	structures repository.StructureRepository,
	enums repository.ConfigEnumRepository,
	snapshots repository.SnapshotRepository,
	publications repository.SnapshotPublicationRepository,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
	cache publishedContentCleaner,
	logger *infra.Logger,
) *PublishLogic {
	return &PublishLogic{
		logger:       logger,
		configs:      configs,
		structures:   structures,
		enums:        enums,
		snapshots:    snapshots,
		publications: publications,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
		cache:        cache,
		now:          time.Now,
	}
}

// Publish switches the released snapshot and advances the runtime cache generation.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; snapshotID selects the snapshot; expectedVersion guards against concurrent changes.
// Returns: the committed released snapshot; stale versions, invalid content, and transaction failures are returned. Cache cleanup failures are logged.
// Throws: none.
func (l *PublishLogic) Publish(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64, expectedVersion uint32) (*model.Snapshot, error) {
	return l.changePublication(ctx, publicationRequest{
		actor:           actor,
		environmentID:   environmentID,
		snapshotID:      snapshotID,
		expectedVersion: expectedVersion,
		status:          model.SnapshotStatusReleased,
	})
}

// Unpublish clears the release and advances the runtime cache generation.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; snapshotID selects the snapshot; expectedVersion guards against concurrent changes.
// Returns: the committed unreleased snapshot; stale versions and transaction failures are returned. Cache cleanup failures are logged.
// Throws: none.
func (l *PublishLogic) Unpublish(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64, expectedVersion uint32) (*model.Snapshot, error) {
	return l.changePublication(ctx, publicationRequest{
		actor:           actor,
		environmentID:   environmentID,
		snapshotID:      snapshotID,
		expectedVersion: expectedVersion,
		status:          model.SnapshotStatusUnreleased,
	})
}

type publicationRequest struct {
	actor           permission.Actor
	environmentID   int64
	snapshotID      int64
	expectedVersion uint32
	status          model.SnapshotStatus
}

type publicationChange struct {
	request   publicationRequest
	config    *model.Config
	target    *model.Snapshot
	released  *model.Snapshot
	changedAt time.Time
}

func (l *PublishLogic) changePublication(ctx context.Context, request publicationRequest) (*model.Snapshot, error) {
	found, err := l.authorizedSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}
	var change *publicationChange
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		change, err = l.preparePublication(txctx, request, found.ConfigID)
		if err != nil {
			return err
		}
		return l.persistPublication(txctx, change)
	})
	if err != nil {
		return nil, err
	}
	l.cleanup(ctx, change.cleanupKey())
	return change.result(), nil
}

func (l *PublishLogic) authorizedSnapshot(ctx context.Context, request publicationRequest) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, request.actor.UserID, request.environmentID, permission.SnapshotPublish); err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, request.environmentID, request.snapshotID)
}

func (l *PublishLogic) preparePublication(ctx context.Context, request publicationRequest, configID int64) (*publicationChange, error) {
	config, snapshots, err := l.lockResources(ctx, request.environmentID, configID)
	if err != nil {
		return nil, err
	}
	target, released, err := publicationState(snapshots, request.snapshotID)
	if err != nil {
		return nil, err
	}
	change := &publicationChange{
		request:  request,
		config:   config,
		target:   target,
		released: released,
	}
	if err := change.validate(); err != nil {
		return nil, err
	}
	change.changedAt = l.now().UTC()
	return change, nil
}

func (change *publicationChange) validate() error {
	if err := change.validateTarget(); err != nil {
		return err
	}
	if err := validateRuntimeVersion(change.config.RuntimeVersion); err != nil {
		return err
	}
	if change.request.status == model.SnapshotStatusUnreleased {
		return nil
	}
	return validateSnapshot(change.target, change.config)
}

func (change *publicationChange) validateTarget() error {
	if change.target.Status == change.request.status {
		return repeatedPublication(change.request.status)
	}
	if change.target.Version != int64(change.request.expectedVersion) {
		return entity.Conflict("snapshot version is stale")
	}
	if change.target.ConfigID != change.config.ID || change.target.ProjectID != change.config.ProjectID || change.target.ConfigKey != change.config.Key {
		return entity.Conflict("snapshot does not belong to config")
	}
	return nil
}

func repeatedPublication(status model.SnapshotStatus) error {
	if status == model.SnapshotStatusReleased {
		return entity.Conflict("snapshot is already released")
	}
	return entity.Conflict("snapshot is already unreleased")
}

func validateRuntimeVersion(version int64) error {
	if version < 0 {
		return entity.Conflict("config runtime version is invalid")
	}
	if version == math.MaxInt64 {
		return entity.Conflict("config runtime version is exhausted")
	}
	return nil
}

func (l *PublishLogic) persistPublication(ctx context.Context, change *publicationChange) error {
	if err := l.applyPublicationStatus(ctx, change); err != nil {
		return err
	}
	if err := l.publications.IncrementRuntimeVersion(ctx, change.request.environmentID, change.config.ID); err != nil {
		return err
	}
	return l.audits.RecordForEnvironment(ctx, change.request.environmentID, publishAudit(change.request.actor, change.auditAction(), change.target.ID))
}

func (l *PublishLogic) applyPublicationStatus(ctx context.Context, change *publicationChange) error {
	if change.request.status == model.SnapshotStatusUnreleased {
		return l.publications.SetUnreleased(ctx, change.updateFor(change.target))
	}
	if err := l.retireReleasedSnapshot(ctx, change); err != nil {
		return err
	}
	return l.publications.SetReleased(ctx, change.updateFor(change.target))
}

func (l *PublishLogic) retireReleasedSnapshot(ctx context.Context, change *publicationChange) error {
	if change.released == nil {
		return nil
	}
	if change.released.Version == math.MaxInt64 {
		return entity.Conflict("released snapshot version is exhausted")
	}
	return l.publications.SetUnreleased(ctx, change.updateFor(change.released))
}

func (change *publicationChange) updateFor(snapshot *model.Snapshot) repository.SnapshotPublicationUpdate {
	return repository.SnapshotPublicationUpdate{
		EnvironmentID:   change.request.environmentID,
		ConfigID:        change.config.ID,
		SnapshotID:      snapshot.ID,
		ExpectedVersion: snapshot.Version,
		UpdatedBy:       change.request.actor.UserID,
		ChangedAt:       change.changedAt,
	}
}

func (change *publicationChange) auditAction() string {
	if change.request.status == model.SnapshotStatusReleased {
		return "snapshot.publish"
	}
	return "snapshot.unpublish"
}

func (change *publicationChange) result() *model.Snapshot {
	copy := *change.target
	copy.Status = change.request.status
	copy.PublishedAt = nil
	copy.PublishedBy = nil
	if copy.Status == model.SnapshotStatusReleased {
		copy.PublishedAt = &change.changedAt
		publisher := change.request.actor.UserID
		copy.PublishedBy = &publisher
	}
	copy.UpdatedBy = change.request.actor.UserID
	copy.UpdatedAt = change.changedAt
	copy.Version++
	return &copy
}

func (change *publicationChange) cleanupKey() cleanupKey {
	return cleanupKey{
		projectID:      change.config.ProjectID,
		configKey:      change.config.Key,
		runtimeVersion: change.config.RuntimeVersion,
	}
}

func (l *PublishLogic) lockResources(ctx context.Context, environmentID, configID int64) (*model.Config, []model.Snapshot, error) {
	config, err := l.configs.LockByID(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, err
	}
	if err := l.lockPublicationSchema(ctx, environmentID, configID); err != nil {
		return nil, nil, err
	}
	snapshots, err := l.publications.LockForConfig(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, err
	}
	return config, snapshots, nil
}

func (l *PublishLogic) lockPublicationSchema(ctx context.Context, environmentID, configID int64) error {
	if _, err := l.structures.ListForConfig(ctx, environmentID, configID); err != nil {
		return err
	}
	_, err := l.enums.ListForConfig(ctx, environmentID, configID)
	return err
}

func publicationState(snapshots []model.Snapshot, targetID int64) (*model.Snapshot, *model.Snapshot, error) {
	var target *model.Snapshot
	var released *model.Snapshot
	for index := range snapshots {
		item := &snapshots[index]
		if item.Status != model.SnapshotStatusUnreleased && item.Status != model.SnapshotStatusReleased {
			return nil, nil, entity.Conflict("snapshot has unsupported status")
		}
		if item.Status == model.SnapshotStatusReleased && released != nil {
			return nil, nil, entity.Conflict("config has multiple released snapshots")
		}
		if item.Status == model.SnapshotStatusReleased {
			released = item
		}
		if item.ID == targetID {
			target = item
		}
	}
	if target == nil {
		return nil, nil, repository.Missing("snapshot")
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

func (l *PublishLogic) cleanup(ctx context.Context, key cleanupKey) {
	if err := l.cache.DeletePublishedConfigVersion(ctx, key.projectID, key.configKey, key.runtimeVersion); err != nil {
		l.logger.WithContext(ctx).Errorw("msg", "清理发布后的配置缓存失败", "project_id", key.projectID, "config_key", key.configKey, "runtime_version", key.runtimeVersion, "error", err)
	}
}

func publishAudit(actor permission.Actor, action string, snapshotID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "snapshot",
		ResourceID:   strconv.FormatInt(snapshotID, 10),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
