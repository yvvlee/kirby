package logic

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
)

type ConflictStrategy int

const (
	StrategyFail    ConflictStrategy = 1
	StrategyReplace ConflictStrategy = 2
)

type ImportRequest struct {
	SourceEnvironmentID int64
	SourceSnapshotID    int64
	TargetEnvironmentID int64
	TargetProjectID     int64
	TargetConfigID      *int64
	Description         string
	Tags                []commonv1.Snapshot_Tag
	IdempotencyKey      string
	ConflictStrategy    ConflictStrategy
}

type ImportResult struct {
	Snapshot *model.Snapshot
	Replayed bool
}

type ImportLogic struct {
	logger       *infra.Logger
	imports      repository.ImportRecordRepository
	projects     repository.ProjectRepository
	configs      repository.ConfigRepository
	structures   repository.StructureRepository
	enums        repository.ConfigEnumRepository
	snapshots    repository.SnapshotRepository
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
	cache        publishedContentCleaner
}

// NewImportLogic creates ImportLogic with its configuration dependencies.
//
// Parameters: imports claims and completes idempotency records; projects checks project ownership; configs reads and writes configuration rows; structures locks and reconciles structure definitions; enums locks and reconciles enum definitions; snapshots reads and writes snapshot rows; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together; cache removes stale published-content cache entries; logger records operational failures.
// Returns: *ImportLogic.
// Throws: none.
func NewImportLogic(
	imports repository.ImportRecordRepository,
	projects repository.ProjectRepository,
	configs repository.ConfigRepository,
	structures repository.StructureRepository,
	enums repository.ConfigEnumRepository,
	snapshots repository.SnapshotRepository,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
	cache publishedContentCleaner,
	logger *infra.Logger,
) *ImportLogic {
	return &ImportLogic{
		logger:       logger,
		imports:      imports,
		projects:     projects,
		configs:      configs,
		structures:   structures,
		enums:        enums,
		snapshots:    snapshots,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
		cache:        cache,
	}
}

// Import copies a snapshot atomically and replays matching idempotent requests.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; request contains the validated request fields.
// Returns: the target snapshot and whether the request was replayed; conflicting idempotency keys or failed writes abort the transaction.
// Throws: none.
func (l *ImportLogic) Import(ctx context.Context, actor permission.Actor, request ImportRequest) (*ImportResult, error) {
	requestHash, normalizedTags, err := validateAndHash(request)
	if err != nil {
		return nil, err
	}
	request.Tags = normalizedTags
	if err := RequireSourceRead(ctx, l.permissions, actor.UserID, request.SourceEnvironmentID); err != nil {
		return nil, err
	}
	if err := l.permissions.Require(ctx, actor.UserID, request.TargetEnvironmentID,
		permission.SnapshotImport, permission.SnapshotWrite,
		permission.ConfigWrite, permission.StructureWrite, permission.EnumWrite,
	); err != nil {
		return nil, err
	}

	var outcome importOutcome
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		outcome, err = l.importSnapshot(txctx, actor, request, requestHash)
		return err
	})
	if err != nil {
		return nil, err
	}
	l.cleanupImport(ctx, outcome.cleanup)
	snapshot, err := l.snapshots.FindByID(ctx, request.TargetEnvironmentID, outcome.snapshotID)
	if err != nil {
		return nil, err
	}
	return &ImportResult{
		Snapshot: snapshot,
		Replayed: outcome.replayed,
	}, nil
}

type importOutcome struct {
	snapshotID int64
	replayed   bool
	cleanup    *cacheTarget
}

type importTarget struct {
	config   *model.Config
	snapshot *model.Snapshot
}

func (l *ImportLogic) importSnapshot(ctx context.Context, actor permission.Actor, request ImportRequest, requestHash [sha256.Size]byte) (importOutcome, error) {
	record, replayed, err := l.claimImport(ctx, actor, request, requestHash)
	if err != nil {
		return importOutcome{}, err
	}
	if replayed {
		return importOutcome{
			snapshotID: *record.TargetSnapshotID,
			replayed:   true,
		}, nil
	}
	target, err := l.prepareImport(ctx, actor, request)
	if err != nil {
		return importOutcome{}, err
	}
	if err := l.completeImport(ctx, actor, request, record.ID, target.snapshot); err != nil {
		return importOutcome{}, err
	}
	return importOutcome{
		snapshotID: target.snapshot.ID,
		cleanup: &cacheTarget{
			projectID:      target.config.ProjectID,
			configKey:      target.config.Key,
			runtimeVersion: target.config.RuntimeVersion,
		},
	}, nil
}

func (l *ImportLogic) claimImport(ctx context.Context, actor permission.Actor, request ImportRequest, requestHash [sha256.Size]byte) (*model.ImportRecord, bool, error) {
	record := &model.ImportRecord{
		UserID:              actor.UserID,
		SourceEnvironmentID: request.SourceEnvironmentID,
		TargetEnvironmentID: request.TargetEnvironmentID,
		SourceSnapshotID:    request.SourceSnapshotID,
		TargetProjectID:     request.TargetProjectID,
		IdempotencyKey:      request.IdempotencyKey,
		RequestHash:         requestHash[:],
		Status:              model.ImportStatusPending,
		ErrorMessage:        uuid.NewString(),
		WorkflowMeta: model.WorkflowMeta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	claimed, created, err := l.imports.Claim(ctx, record)
	if err != nil {
		return nil, false, err
	}
	if subtle.ConstantTimeCompare(claimed.RequestHash, requestHash[:]) != 1 {
		return nil, false, entity.Conflict("idempotency key was used for a different request")
	}
	if claimed.Status == model.ImportStatusSucceeded && claimed.TargetSnapshotID != nil {
		return claimed, true, nil
	}
	if !created || claimed.Status != model.ImportStatusPending {
		return nil, false, entity.Conflict("matching snapshot import is still pending")
	}
	return claimed, false, nil
}

func (l *ImportLogic) prepareImport(ctx context.Context, actor permission.Actor, request ImportRequest) (*importTarget, error) {
	sourceSnapshot, err := l.snapshots.LockByID(ctx, request.SourceEnvironmentID, request.SourceSnapshotID)
	if err != nil {
		return nil, err
	}
	source, err := ValidateSnapshot(sourceSnapshot)
	if err != nil {
		return nil, err
	}
	if _, err := l.projects.LockByID(ctx, request.TargetEnvironmentID, request.TargetProjectID); err != nil {
		return nil, err
	}
	config, err := l.writeConfig(ctx, actor.UserID, request, source)
	if err != nil {
		return nil, err
	}
	content, err := l.importResources(ctx, actor.UserID, request.TargetEnvironmentID, config, source)
	if err != nil {
		return nil, err
	}
	tagsJSON, err := converter.EncodeTags(request.Tags)
	if err != nil {
		return nil, err
	}
	return &importTarget{
		config: config,
		snapshot: &model.Snapshot{
			ProjectID:   config.ProjectID,
			ConfigID:    config.ID,
			ConfigKey:   config.Key,
			Description: request.Description,
			Content:     content,
			TagsJSON:    tagsJSON,
			Meta: model.Meta{
				CreatedBy: actor.UserID,
				UpdatedBy: actor.UserID,
			},
		},
	}, nil
}

func (l *ImportLogic) importResources(ctx context.Context, actorID, environmentID int64, config *model.Config, source *entity.ConfigSnapshot) (string, error) {
	structures, enums, err := resourceModels(source, config.ID, actorID)
	if err != nil {
		return "", err
	}
	if err := l.structures.Reconcile(ctx, environmentID, config.ID, structures, actorID); err != nil {
		return "", err
	}
	if err := l.enums.Reconcile(ctx, environmentID, config.ID, enums, actorID); err != nil {
		return "", err
	}
	content, err := canonicalTargetContent(source, config, structures, enums)
	if err != nil {
		return "", err
	}
	return entity.EncodeConfigSnapshot(content)
}

func (l *ImportLogic) completeImport(ctx context.Context, actor permission.Actor, request ImportRequest, recordID int64, snapshot *model.Snapshot) error {
	if err := l.snapshots.Create(ctx, request.TargetEnvironmentID, snapshot.ProjectID, snapshot.ConfigID, snapshot); err != nil {
		return err
	}
	if err := l.snapshots.SetCurrent(ctx, request.TargetEnvironmentID, snapshot.ConfigID, snapshot.ID, actor.UserID); err != nil {
		return err
	}
	resultJSON := fmt.Sprintf(`{"target_snapshot_id":%d}`, snapshot.ID)
	if err := l.imports.Complete(ctx, recordID, snapshot.ID, resultJSON); err != nil {
		return err
	}
	return l.audits.RecordForEnvironment(ctx, request.TargetEnvironmentID, importAudit(actor, request, snapshot.ID))
}

func (l *ImportLogic) cleanupImport(ctx context.Context, target *cacheTarget) {
	if target == nil {
		return
	}
	if err := l.cache.DeletePublishedConfigVersion(ctx, target.projectID, target.configKey, target.runtimeVersion); err != nil {
		l.logger.WithContext(ctx).Errorw("msg", "清理导入后的配置缓存失败", "project_id", target.projectID, "config_key", target.configKey, "runtime_version", target.runtimeVersion, "error", err)
	}
}

func (l *ImportLogic) writeConfig(ctx context.Context, actorID int64, request ImportRequest, source *entity.ConfigSnapshot) (*model.Config, error) {
	txctx := ctx
	typeJSON, err := converter.EncodeFieldType(source.Config.Type)
	if err != nil {
		return nil, err
	}
	if request.ConflictStrategy == StrategyFail {
		item := &model.Config{
			ProjectID:   request.TargetProjectID,
			Key:         source.Config.Key,
			Description: source.Config.Description,
			IsArray:     source.Config.IsArray,
			TypeJSON:    typeJSON,
			Value:       source.Config.Value,
			Meta: model.Meta{
				CreatedBy: actorID,
				UpdatedBy: actorID,
			},
		}
		if err := l.configs.Create(txctx, request.TargetEnvironmentID, request.TargetProjectID, item); err != nil {
			return nil, err
		}
		return item, nil
	}
	target, err := l.configs.LockByID(txctx, request.TargetEnvironmentID, *request.TargetConfigID)
	if err != nil {
		return nil, err
	}
	if target.ProjectID != request.TargetProjectID {
		return nil, repository.Missing("target config")
	}
	if err := l.configs.Update(txctx, request.TargetEnvironmentID, target.ID, repository.ConfigUpdate{
		Description: source.Config.Description,
		IsArray:     source.Config.IsArray,
		TypeJSON:    typeJSON,
		UpdatedBy:   actorID,
		Version:     target.Version,
	}); err != nil {
		return nil, err
	}
	if err := l.configs.UpdateValue(txctx, request.TargetEnvironmentID, target.ID, repository.ConfigValueUpdate{
		Value:     source.Config.Value,
		UpdatedBy: actorID,
		Version:   target.Version + 1,
	}); err != nil {
		return nil, err
	}
	target.Description = source.Config.Description
	target.IsArray = source.Config.IsArray
	target.TypeJSON = typeJSON
	target.Value = source.Config.Value
	target.Version += 2
	target.UpdatedBy = actorID
	return target, nil
}

func validateAndHash(request ImportRequest) ([sha256.Size]byte, []commonv1.Snapshot_Tag, error) {
	var empty [sha256.Size]byte
	if request.ConflictStrategy == StrategyFail && request.TargetConfigID != nil || request.ConflictStrategy == StrategyReplace && request.TargetConfigID == nil || request.ConflictStrategy != StrategyFail && request.ConflictStrategy != StrategyReplace {
		return empty, nil, entity.Invalid("snapshot import conflict strategy does not match target config")
	}
	tags := append([]commonv1.Snapshot_Tag(nil), request.Tags...)
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	for index, tag := range tags {
		if tag < commonv1.Snapshot_RELEASE || tag > commonv1.Snapshot_REUSE || index > 0 && tags[index-1] == tag {
			return empty, nil, entity.Invalid("snapshot import tags are invalid")
		}
	}
	canonical := struct {
		SourceEnvironmentID int64                   `json:"source_environment_id"`
		SourceSnapshotID    int64                   `json:"source_snapshot_id"`
		TargetEnvironmentID int64                   `json:"target_environment_id"`
		TargetProjectID     int64                   `json:"target_project_id"`
		TargetConfigID      *int64                  `json:"target_config_id"`
		Description         string                  `json:"description"`
		Tags                []commonv1.Snapshot_Tag `json:"tags"`
		ConflictStrategy    ConflictStrategy        `json:"conflict_strategy"`
	}{
		request.SourceEnvironmentID,
		request.SourceSnapshotID,
		request.TargetEnvironmentID,
		request.TargetProjectID,
		request.TargetConfigID,
		request.Description,
		tags,
		request.ConflictStrategy,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return empty, nil, fmt.Errorf("encode canonical import request: %w", err)
	}
	return sha256.Sum256(encoded), tags, nil
}

func resourceModels(content *entity.ConfigSnapshot, configID, actorID int64) ([]*model.Structure, []*model.ConfigEnum, error) {
	structures := make([]*model.Structure, 0, len(content.Structures))
	for _, item := range content.Structures {
		fields, err := converter.EncodeFields(item.Fields)
		if err != nil {
			return nil, nil, err
		}
		structures = append(structures, &model.Structure{
			ConfigID:    configID,
			Key:         item.Key,
			Name:        item.Name,
			Description: item.Description,
			FieldsJSON:  fields,
			Meta: model.Meta{
				CreatedBy: actorID,
				UpdatedBy: actorID,
			},
		})
	}
	enums := make([]*model.ConfigEnum, 0, len(content.Enums))
	for _, item := range content.Enums {
		values, err := converter.EncodeOptions(item.Values)
		if err != nil {
			return nil, nil, err
		}
		enums = append(enums, &model.ConfigEnum{
			ConfigID:    configID,
			Key:         item.Key,
			Name:        item.Name,
			Description: item.Description,
			ValuesJSON:  values,
			Meta: model.Meta{
				CreatedBy: actorID,
				UpdatedBy: actorID,
			},
		})
	}
	return structures, enums, nil
}

func canonicalTargetContent(source *entity.ConfigSnapshot, config *model.Config, structures []*model.Structure, enums []*model.ConfigEnum) (*entity.ConfigSnapshot, error) {
	runtimeVersion, err := safeint.Uint64FromInt64(config.RuntimeVersion)
	if err != nil {
		return nil, fmt.Errorf("target config runtime version is invalid")
	}
	result := &entity.ConfigSnapshot{Config: proto.Clone(source.Config).(*commonv1.Config)}
	result.Config.Id, result.Config.ProjectId, result.Config.Key = config.ID, config.ProjectID, config.Key
	result.Config.RuntimeVersion = runtimeVersion
	result.Config.CreatedBy, result.Config.UpdatedBy, result.Config.CreatedAt, result.Config.UpdatedAt, result.Config.Version = "", "", "", "", 0
	result.Structures = make([]*commonv1.Structure, 0, len(source.Structures))
	structureByKey := make(map[string]*model.Structure, len(structures))
	for _, item := range structures {
		structureByKey[item.Key] = item
	}
	for _, sourceItem := range source.Structures {
		item := proto.Clone(sourceItem).(*commonv1.Structure)
		modelItem := structureByKey[item.Key]
		if modelItem == nil || modelItem.ID <= 0 {
			return nil, fmt.Errorf("target structure was not persisted")
		}
		item.Id, item.ConfigId = modelItem.ID, config.ID
		item.CreatedBy, item.UpdatedBy, item.CreatedAt, item.UpdatedAt, item.Version = "", "", "", "", 0
		result.Structures = append(result.Structures, item)
	}
	result.Enums = make([]*commonv1.ConfigEnum, 0, len(source.Enums))
	enumByKey := make(map[string]*model.ConfigEnum, len(enums))
	for _, item := range enums {
		enumByKey[item.Key] = item
	}
	for _, sourceItem := range source.Enums {
		item := proto.Clone(sourceItem).(*commonv1.ConfigEnum)
		modelItem := enumByKey[item.Key]
		if modelItem == nil || modelItem.ID <= 0 {
			return nil, fmt.Errorf("target enum was not persisted")
		}
		item.Id, item.ConfigId = modelItem.ID, config.ID
		item.CreatedBy, item.UpdatedBy, item.CreatedAt, item.UpdatedAt, item.Version = "", "", "", "", 0
		result.Enums = append(result.Enums, item)
	}
	schema, err := entity.NewSchema(result.Structures, result.Enums)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateConfig(result.Config); err != nil {
		return nil, err
	}
	result.Tree, err = schema.BuildTree(result.Config)
	return result, err
}

type cacheTarget struct {
	projectID      int64
	configKey      string
	runtimeVersion int64
}

func importAudit(actor permission.Actor, request ImportRequest, snapshotID int64) *model.AuditLog {
	actorID := actor.UserID
	details, _ := json.Marshal(struct {
		SourceEnvironmentID int64 `json:"source_environment_id"`
		SourceSnapshotID    int64 `json:"source_snapshot_id"`
		TargetProjectID     int64 `json:"target_project_id"`
	}{
		request.SourceEnvironmentID,
		request.SourceSnapshotID,
		request.TargetProjectID,
	})
	detailsJSON := string(details)
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       "snapshot.import",
		ResourceType: "snapshot",
		ResourceID:   strconv.FormatInt(snapshotID, 10),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
		DetailsJSON:  &detailsJSON,
	}
}
