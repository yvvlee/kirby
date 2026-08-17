package importer

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	exporter "github.com/yvvlee/kirby/server/internal/logic/export"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/safeint"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type ConflictStrategy int

const (
	StrategyFail    ConflictStrategy = 1
	StrategyReplace ConflictStrategy = 2
)

type Request struct {
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

type Result struct {
	Snapshot *model.Snapshot
	Replayed bool
}

type ImportRepository interface {
	ClaimTx(context.Context, *xorm.Session, *model.ImportRecord) (*model.ImportRecord, bool, error)
	CompleteTx(context.Context, *xorm.Session, int64, int64, string) error
}

type ProjectRepository interface {
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Project, error)
}

type ConfigRepository interface {
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.Config) error
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
	UpdateTx(context.Context, *xorm.Session, int64, int64, repository.ConfigUpdate) error
	UpdateValueTx(context.Context, *xorm.Session, int64, int64, repository.ConfigValueUpdate) error
}

type StructureRepository interface {
	ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.Structure, int64) error
}

type EnumRepository interface {
	ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.ConfigEnum, int64) error
}

type SnapshotRepository interface {
	FindByID(context.Context, int64, int64) (*model.Snapshot, error)
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	CreateTx(context.Context, *xorm.Session, int64, int64, int64, *model.Snapshot) error
	SetCurrent(context.Context, *xorm.Session, int64, int64, int64, int64) error
}

type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}

type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type CacheCleaner interface {
	DeletePublishedConfigVersion(context.Context, int64, int64, string, int64) error
}

type Logic struct {
	imports      ImportRepository
	projects     ProjectRepository
	configs      ConfigRepository
	structures   StructureRepository
	enums        EnumRepository
	snapshots    SnapshotRepository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
	cache        CacheCleaner
}

func New(imports ImportRepository, projects ProjectRepository, configs ConfigRepository, structures StructureRepository, enums EnumRepository, snapshots SnapshotRepository, permissions Authorizer, audits AuditRepository, transactions database.Transactor, cache CacheCleaner) (*Logic, error) {
	if imports == nil || projects == nil || configs == nil || structures == nil || enums == nil || snapshots == nil || permissions == nil || audits == nil || transactions == nil || cache == nil {
		return nil, fmt.Errorf("snapshot import dependencies are incomplete")
	}
	return &Logic{imports: imports, projects: projects, configs: configs, structures: structures, enums: enums, snapshots: snapshots, permissions: permissions, audits: audits, transactions: transactions, cache: cache}, nil
}

func (l *Logic) Import(ctx context.Context, actor permission.Actor, request Request) (*Result, error) {
	requestHash, normalizedTags, err := validateAndHash(request)
	if err != nil {
		return nil, err
	}
	request.Tags = normalizedTags
	if err := exporter.RequireSourceRead(ctx, l.permissions, actor.UserID, request.SourceEnvironmentID); err != nil {
		return nil, err
	}
	if err := l.permissions.Require(ctx, actor.UserID, request.TargetEnvironmentID,
		permission.SnapshotImport, permission.SnapshotWrite,
		permission.ConfigWrite, permission.StructureWrite, permission.EnumWrite,
	); err != nil {
		return nil, err
	}

	var targetSnapshotID int64
	var replayed bool
	var cleanup *cacheTarget
	claimValue := uuid.NewString()
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		record := &model.ImportRecord{
			UserID: actor.UserID, SourceEnvironmentID: request.SourceEnvironmentID,
			TargetEnvironmentID: request.TargetEnvironmentID, SourceSnapshotID: request.SourceSnapshotID,
			TargetProjectID: request.TargetProjectID, IdempotencyKey: request.IdempotencyKey,
			RequestHash: requestHash[:], Status: model.ImportStatusPending, ErrorMessage: claimValue,
			WorkflowMeta: model.WorkflowMeta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID},
		}
		claimed, created, err := l.imports.ClaimTx(ctx, tx, record)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare(claimed.RequestHash, requestHash[:]) != 1 {
			return entity.Conflict("idempotency key was used for a different request")
		}
		if claimed.Status == model.ImportStatusSucceeded && claimed.TargetSnapshotID != nil {
			targetSnapshotID = *claimed.TargetSnapshotID
			replayed = true
			return nil
		}
		if !created || claimed.Status != model.ImportStatusPending {
			return entity.Conflict("matching snapshot import is still pending")
		}

		sourceSnapshot, err := l.snapshots.LockByID(ctx, tx, request.SourceEnvironmentID, request.SourceSnapshotID)
		if err != nil {
			return err
		}
		sourceContent, err := exporter.ValidateSnapshot(sourceSnapshot)
		if err != nil {
			return err
		}
		targetProject, err := l.projects.LockByID(ctx, tx, request.TargetEnvironmentID, request.TargetProjectID)
		if err != nil {
			return err
		}
		if targetProject == nil {
			return fmt.Errorf("target project repository returned nil project")
		}

		targetConfig, err := l.writeConfig(ctx, tx, actor.UserID, request, sourceContent)
		if err != nil {
			return err
		}
		structures, enums, err := resourceModels(sourceContent, targetConfig.ID, actor.UserID)
		if err != nil {
			return err
		}
		if err := l.structures.ReconcileTx(ctx, tx, request.TargetEnvironmentID, targetConfig.ID, structures, actor.UserID); err != nil {
			return err
		}
		if err := l.enums.ReconcileTx(ctx, tx, request.TargetEnvironmentID, targetConfig.ID, enums, actor.UserID); err != nil {
			return err
		}
		content, err := canonicalTargetContent(sourceContent, targetConfig, structures, enums)
		if err != nil {
			return err
		}
		encoded, err := entity.EncodeConfigSnapshot(content)
		if err != nil {
			return err
		}
		tagsJSON, err := converter.EncodeTags(request.Tags)
		if err != nil {
			return err
		}
		targetSnapshot := &model.Snapshot{
			ProjectID: targetProject.ID, ConfigID: targetConfig.ID, ConfigKey: targetConfig.Key,
			Description: request.Description, Content: encoded, TagsJSON: tagsJSON,
			Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID},
		}
		if err := l.snapshots.CreateTx(ctx, tx, request.TargetEnvironmentID, targetProject.ID, targetConfig.ID, targetSnapshot); err != nil {
			return err
		}
		if err := l.snapshots.SetCurrent(ctx, tx, request.TargetEnvironmentID, targetConfig.ID, targetSnapshot.ID, actor.UserID); err != nil {
			return err
		}
		resultJSON, err := json.Marshal(struct {
			TargetSnapshotID int64 `json:"target_snapshot_id"`
		}{TargetSnapshotID: targetSnapshot.ID})
		if err != nil {
			return err
		}
		if err := l.imports.CompleteTx(ctx, tx, claimed.ID, targetSnapshot.ID, string(resultJSON)); err != nil {
			return err
		}
		if err := l.audits.RecordForEnvironmentTx(ctx, tx, request.TargetEnvironmentID, importAudit(actor, request, targetSnapshot.ID)); err != nil {
			return err
		}
		targetSnapshotID = targetSnapshot.ID
		cleanup = &cacheTarget{environmentID: request.TargetEnvironmentID, projectID: targetProject.ID, configKey: targetConfig.Key, runtimeVersion: targetConfig.RuntimeVersion}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		_ = l.cache.DeletePublishedConfigVersion(ctx, cleanup.environmentID, cleanup.projectID, cleanup.configKey, cleanup.runtimeVersion)
	}
	snapshot, err := l.snapshots.FindByID(ctx, request.TargetEnvironmentID, targetSnapshotID)
	if err != nil {
		return nil, err
	}
	return &Result{Snapshot: snapshot, Replayed: replayed}, nil
}

func (l *Logic) writeConfig(ctx context.Context, tx *xorm.Session, actorID int64, request Request, source *entity.ConfigSnapshot) (*model.Config, error) {
	typeJSON, err := converter.EncodeFieldType(source.Config.Type)
	if err != nil {
		return nil, err
	}
	if request.ConflictStrategy == StrategyFail {
		item := &model.Config{
			ProjectID: request.TargetProjectID, Key: source.Config.Key, Description: source.Config.Description,
			IsArray: source.Config.IsArray, TypeJSON: typeJSON, Value: source.Config.Value,
			Meta: model.Meta{CreatedBy: actorID, UpdatedBy: actorID},
		}
		if err := l.configs.CreateTx(ctx, tx, request.TargetEnvironmentID, request.TargetProjectID, item); err != nil {
			return nil, err
		}
		return item, nil
	}
	target, err := l.configs.LockByID(ctx, tx, request.TargetEnvironmentID, *request.TargetConfigID)
	if err != nil {
		return nil, err
	}
	if target.ProjectID != request.TargetProjectID {
		return nil, base.Missing("target config")
	}
	if err := l.configs.UpdateTx(ctx, tx, request.TargetEnvironmentID, target.ID, repository.ConfigUpdate{
		Description: source.Config.Description, IsArray: source.Config.IsArray, TypeJSON: typeJSON,
		UpdatedBy: actorID, Version: target.Version,
	}); err != nil {
		return nil, err
	}
	if err := l.configs.UpdateValueTx(ctx, tx, request.TargetEnvironmentID, target.ID, repository.ConfigValueUpdate{
		Value: source.Config.Value, UpdatedBy: actorID, Version: target.Version + 1,
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

func validateAndHash(request Request) ([sha256.Size]byte, []commonv1.Snapshot_Tag, error) {
	var empty [sha256.Size]byte
	if request.SourceEnvironmentID <= 0 || request.SourceSnapshotID <= 0 || request.TargetEnvironmentID <= 0 || request.TargetProjectID <= 0 || request.TargetConfigID != nil && *request.TargetConfigID <= 0 {
		return empty, nil, entity.Invalid("snapshot import scope is invalid")
	}
	if request.ConflictStrategy == StrategyFail && request.TargetConfigID != nil || request.ConflictStrategy == StrategyReplace && request.TargetConfigID == nil || request.ConflictStrategy != StrategyFail && request.ConflictStrategy != StrategyReplace {
		return empty, nil, entity.Invalid("snapshot import conflict strategy does not match target config")
	}
	if length := utf8.RuneCountInString(request.Description); length < 2 || length > 255 {
		return empty, nil, entity.Invalid("snapshot import description is invalid")
	}
	if length := utf8.RuneCountInString(request.IdempotencyKey); length < 16 || length > 128 || !asciiGraphic(request.IdempotencyKey) {
		return empty, nil, entity.Invalid("snapshot import idempotency key is invalid")
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
	}{request.SourceEnvironmentID, request.SourceSnapshotID, request.TargetEnvironmentID, request.TargetProjectID, request.TargetConfigID, request.Description, tags, request.ConflictStrategy}
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
		structures = append(structures, &model.Structure{ConfigID: configID, Key: item.Key, Name: item.Name, Description: item.Description, FieldsJSON: fields, Meta: model.Meta{CreatedBy: actorID, UpdatedBy: actorID}})
	}
	enums := make([]*model.ConfigEnum, 0, len(content.Enums))
	for _, item := range content.Enums {
		values, err := converter.EncodeOptions(item.Values)
		if err != nil {
			return nil, nil, err
		}
		enums = append(enums, &model.ConfigEnum{ConfigID: configID, Key: item.Key, Name: item.Name, Description: item.Description, ValuesJSON: values, Meta: model.Meta{CreatedBy: actorID, UpdatedBy: actorID}})
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
	environmentID  int64
	projectID      int64
	configKey      string
	runtimeVersion int64
}

func importAudit(actor permission.Actor, request Request, snapshotID int64) *model.AuditLog {
	actorID := actor.UserID
	details, _ := json.Marshal(struct {
		SourceEnvironmentID int64 `json:"source_environment_id"`
		SourceSnapshotID    int64 `json:"source_snapshot_id"`
		TargetProjectID     int64 `json:"target_project_id"`
	}{request.SourceEnvironmentID, request.SourceSnapshotID, request.TargetProjectID})
	detailsJSON := string(details)
	return &model.AuditLog{ActorUserID: &actorID, Action: "snapshot.import", ResourceType: "snapshot", ResourceID: strconv.FormatInt(snapshotID, 10), Result: model.AuditResultSucceeded, RequestID: actor.RequestID, DetailsJSON: &detailsJSON}
}

func asciiGraphic(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}
