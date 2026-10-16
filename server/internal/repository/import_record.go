package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type ImportRecordRepository interface {
	Claim(ctx context.Context, record *model.ImportRecord) (*model.ImportRecord, bool, error)
	Complete(ctx context.Context, recordID, targetSnapshotID int64, resultJSON string) error
}

type ImportRecordRepositoryImpl struct {
	*lorm.Repository[*model.ImportRecord]
}

// NewImportRecordRepository creates ImportRecordRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *ImportRecordRepositoryImpl.
// Throws: none.
func NewImportRecordRepository(storage *infra.Storage) *ImportRecordRepositoryImpl {
	return &ImportRecordRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.ImportRecord](),
	}
}

// Claim 通过(user_id, target_environment_id, idempotency_key)唯一键串行化导入请求。
// 返回的布尔值仅对成功插入pending记录的事务为真。
// Parameters: ctx carries the caller transaction; record contains the pending import.
// Returns: the claimed or existing record, and whether this transaction created it.
// Throws: a database error or missing existing import record.
func (r *ImportRecordRepositoryImpl) Claim(ctx context.Context, record *model.ImportRecord) (*model.ImportRecord, bool, error) {
	created, err := r.insertClaim(ctx, record)
	if err != nil {
		return nil, false, WrapDatabaseError(err)
	}
	if created {
		claimed := *record
		return &claimed, true, nil
	}
	return r.lockExistingClaim(ctx, record)
}

func (r *ImportRecordRepositoryImpl) insertClaim(ctx context.Context, record *model.ImportRecord) (bool, error) {
	record.PrepareInsert()
	if r.Engine.SupportsReturning() && !r.Engine.SupportsLastInsertId() {
		return r.insertPostgresClaim(ctx, record)
	}
	_, err := r.Insert(ctx, record)
	if isDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

func (r *ImportRecordRepositoryImpl) insertPostgresClaim(ctx context.Context, record *model.ImportRecord) (bool, error) {
	// A duplicate INSERT aborts PostgreSQL transactions before the locked replay
	// read. Limit conflict handling to this operation's idempotency constraint.
	const statement = `INSERT INTO import_records (
		user_id, source_environment_id, target_environment_id,
		source_snapshot_id, target_project_id, target_snapshot_id,
		idempotency_key, request_hash, status, result_json, error_message,
		created_by, updated_by, created_at, updated_at, version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	ON CONFLICT (user_id, target_environment_id, idempotency_key) DO NOTHING
	RETURNING *`
	rows, err := r.Engine.SQL(ctx, statement,
		record.UserID,
		record.SourceEnvironmentID,
		record.TargetEnvironmentID,
		record.SourceSnapshotID,
		record.TargetProjectID,
		record.TargetSnapshotID,
		record.IdempotencyKey,
		record.RequestHash,
		record.Status,
		record.ResultJSON,
		record.ErrorMessage,
		record.CreatedBy,
		record.UpdatedBy,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	err = lorm.ScanModel(rows, record)
	if errors.Is(err, sql.ErrNoRows) {
		return false, rows.Close()
	}
	if err != nil {
		return false, err
	}
	return true, rows.Close()
}

func (r *ImportRecordRepositoryImpl) lockExistingClaim(ctx context.Context, record *model.ImportRecord) (*model.ImportRecord, bool, error) {
	fields := new(model.ImportRecord).LormCols()
	existing, has, err := r.Engine.Query[*model.ImportRecord]().
		Where(builder.Eq{
			fields.UserID():              record.UserID,
			fields.TargetEnvironmentID(): record.TargetEnvironmentID,
			fields.IdempotencyKey():      record.IdempotencyKey,
		}).
		Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, false, WrapDatabaseError(err)
	}
	if !has {
		return nil, false, Missing("import record")
	}
	return existing, false, nil
}

// Complete 将pending记录标记为成功并写入结果。
//
// Parameters: ctx carries cancellation and transaction state; recordID identifies the pending import claim; targetSnapshotID identifies the snapshot created by this import; resultJSON stores the replay response.
// Returns: nil after a pending claim becomes successful; ErrNoRowsAffected if it is no longer pending.
// Throws: none.
func (r *ImportRecordRepositoryImpl) Complete(ctx context.Context, recordID, targetSnapshotID int64, resultJSON string) error {
	fields := new(model.ImportRecord).LormCols()
	affected, err := r.Engine.Update[*model.ImportRecord]().ID(recordID).
		Where(builder.Eq{fields.Status(): model.ImportStatusPending}).
		Set(fields.Version(), builder.Expr("version + 1")).
		SetMap(map[string]any{
			fields.TargetSnapshotID(): targetSnapshotID,
			fields.Status():           model.ImportStatusSucceeded,
			fields.ResultJSON():       resultJSON,
			fields.ErrorMessage():     "",
			fields.UpdatedAt():        time.Now().UTC(),
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("import record")
	}
	return nil
}

var _ ImportRecordRepository = (*ImportRecordRepositoryImpl)(nil)
