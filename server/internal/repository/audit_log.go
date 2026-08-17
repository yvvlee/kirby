package repository

import (
	"context"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type AuditLogRepository interface {
	RecordForEnvironment(context.Context, int64, *model.AuditLog) error
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
	RecordSystem(context.Context, *model.AuditLog) error
	RecordSystemTx(context.Context, *xorm.Session, *model.AuditLog) error
	ListByEnvironment(context.Context, int64, base.PageRequest) (base.PageResult[model.AuditLog], error)
}

type AuditLogRepositoryImpl struct {
	engine *xorm.Engine
}

func NewAuditLogRepository(engine *xorm.Engine) *AuditLogRepositoryImpl {
	return &AuditLogRepositoryImpl{engine: engine}
}

func (r *AuditLogRepositoryImpl) RecordForEnvironment(ctx context.Context, environmentID int64, log *model.AuditLog) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	if log == nil {
		return base.InvalidArgument("audit log is nil")
	}
	if log.EnvironmentID != nil && *log.EnvironmentID != environmentID {
		return base.InvalidArgument("audit_log.environment_id does not match environment_id")
	}
	log.EnvironmentID = &environmentID
	result, err := base.Execute(ctx, r.engine, "environment audit log", insertEnvironmentAuditLogSQL,
		environmentAuditLogArgs(environmentID, log)...)
	if err != nil {
		return err
	}
	log.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted environment audit log id", err)
	}
	return nil
}

func (r *AuditLogRepositoryImpl) RecordForEnvironmentTx(ctx context.Context, tx *xorm.Session, environmentID int64, log *model.AuditLog) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	if log == nil {
		return base.InvalidArgument("audit log is nil")
	}
	if log.EnvironmentID != nil && *log.EnvironmentID != environmentID {
		return base.InvalidArgument("audit_log.environment_id does not match environment_id")
	}
	log.EnvironmentID = &environmentID
	result, err := base.ExecuteTx(ctx, tx, "environment audit log", insertEnvironmentAuditLogSQL,
		environmentAuditLogArgs(environmentID, log)...)
	if err != nil {
		return err
	}
	log.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted environment audit log id", err)
	}
	return nil
}

const insertEnvironmentAuditLogSQL = `
INSERT INTO audit_logs
    (actor_user_id, environment_id, action, resource_type, resource_id, result, request_id, details_json)
SELECT ?, e.id, ?, ?, ?, ?, ?, ?
FROM environments AS e
WHERE e.id = ? AND e.deleted_at IS NULL`

func environmentAuditLogArgs(environmentID int64, log *model.AuditLog) []any {
	return []any{
		log.ActorUserID, log.Action, log.ResourceType, log.ResourceID, log.Result,
		log.RequestID, log.DetailsJSON, environmentID,
	}
}

func (r *AuditLogRepositoryImpl) RecordSystem(ctx context.Context, log *model.AuditLog) error {
	if log == nil {
		return base.InvalidArgument("audit log is nil")
	}
	if log.EnvironmentID != nil {
		return base.InvalidArgument("system audit log must not have environment_id")
	}
	result, err := base.Execute(ctx, r.engine, "system audit log", insertSystemAuditLogSQL,
		systemAuditLogArgs(log)...)
	if err != nil {
		return err
	}
	log.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted system audit log id", err)
	}
	return nil
}

func (r *AuditLogRepositoryImpl) RecordSystemTx(ctx context.Context, tx *xorm.Session, log *model.AuditLog) error {
	if log == nil {
		return base.InvalidArgument("audit log is nil")
	}
	if log.EnvironmentID != nil {
		return base.InvalidArgument("system audit log must not have environment_id")
	}
	result, err := base.ExecuteTx(ctx, tx, "system audit log", insertSystemAuditLogSQL,
		systemAuditLogArgs(log)...)
	if err != nil {
		return err
	}
	log.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted system audit log id", err)
	}
	return nil
}

const insertSystemAuditLogSQL = `
INSERT INTO audit_logs
    (actor_user_id, environment_id, action, resource_type, resource_id, result, request_id, details_json)
VALUES (?, NULL, ?, ?, ?, ?, ?, ?)`

func systemAuditLogArgs(log *model.AuditLog) []any {
	return []any{
		log.ActorUserID, log.Action, log.ResourceType, log.ResourceID, log.Result,
		log.RequestID, log.DetailsJSON,
	}
}

func (r *AuditLogRepositoryImpl) ListByEnvironment(ctx context.Context, environmentID int64, page base.PageRequest) (base.PageResult[model.AuditLog], error) {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return base.PageResult[model.AuditLog]{}, err
	}
	page = base.NormalizePage(page)
	total, err := base.Count(ctx, r.engine, "environment audit logs", `
SELECT COUNT(*) AS total
FROM audit_logs AS a
WHERE a.environment_id = ?`, environmentID)
	if err != nil {
		return base.PageResult[model.AuditLog]{}, err
	}
	result := base.PageResult[model.AuditLog]{Items: []model.AuditLog{}, Total: total, Offset: page.Offset, Limit: page.Limit}
	if total == 0 {
		return result, nil
	}
	err = base.FindAll(ctx, r.engine, "environment audit logs", `
SELECT a.*
FROM audit_logs AS a
WHERE a.environment_id = ?
ORDER BY a.id DESC
LIMIT ? OFFSET ?`, []any{environmentID, page.Limit, page.Offset}, &result.Items)
	return result, err
}

var _ AuditLogRepository = (*AuditLogRepositoryImpl)(nil)
