package repository

import (
	"context"

	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type AuditLogRepository interface {
	RecordForEnvironment(ctx context.Context, environmentID int64, log *model.AuditLog) error
	RecordSystem(ctx context.Context, log *model.AuditLog) error
}

type AuditLogRepositoryImpl struct {
	*lorm.Repository[*model.AuditLog]
}

// NewAuditLogRepository creates AuditLogRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *AuditLogRepositoryImpl.
// Throws: none.
func NewAuditLogRepository(storage *infra.Storage) *AuditLogRepositoryImpl {
	return &AuditLogRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.AuditLog](),
	}
}

// RecordForEnvironment 记录环境级审计日志，Insert自动回填ID。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; log contains the immutable audit event.
// Returns: nil after assigning environmentID and inserting the audit event, or the insertion error.
// Throws: none.
func (r *AuditLogRepositoryImpl) RecordForEnvironment(ctx context.Context, environmentID int64, log *model.AuditLog) error {
	log.EnvironmentID = &environmentID
	log.PrepareInsert()
	_, err := r.Insert(ctx, log)
	return err
}

// RecordSystem 记录系统级审计日志。
//
// Parameters: ctx carries cancellation and transaction state; log contains the immutable audit event.
// Returns: nil after inserting the system audit event, or the insertion error.
// Throws: none.
func (r *AuditLogRepositoryImpl) RecordSystem(ctx context.Context, log *model.AuditLog) error {
	log.PrepareInsert()
	_, err := r.Insert(ctx, log)
	return err
}

var _ AuditLogRepository = (*AuditLogRepositoryImpl)(nil)
