package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type ImportRecordRepository struct{}

func NewImportRecordRepository() *ImportRecordRepository {
	return &ImportRecordRepository{}
}

// ClaimTx serializes requests sharing the user, target environment and
// idempotency key. The returned boolean is true only for the transaction that
// inserted the pending record.
func (*ImportRecordRepository) ClaimTx(ctx context.Context, tx *xorm.Session, record *model.ImportRecord) (*model.ImportRecord, bool, error) {
	if tx == nil {
		return nil, false, base.InvalidArgument("transaction session is nil")
	}
	if err := validateImportRecord(record); err != nil {
		return nil, false, err
	}
	_, err := tx.Context(ctx).Exec(`
INSERT INTO import_records
    (user_id, source_environment_id, target_environment_id, source_snapshot_id,
     target_project_id, target_snapshot_id, idempotency_key, request_hash,
     status, result_json, error_message, created_by, updated_by)
SELECT u.id, source_environment.id, target_environment.id, source_snapshot.id,
       target_project.id, NULL, ?, ?, ?, NULL, ?, ?, ?
FROM users AS u
INNER JOIN environments AS source_environment
    ON source_environment.id = ? AND source_environment.enabled = TRUE
   AND source_environment.deleted_at IS NULL
INNER JOIN snapshots AS source_snapshot
    ON source_snapshot.id = ? AND source_snapshot.deleted_at IS NULL
INNER JOIN projects AS source_project
    ON source_project.id = source_snapshot.project_id
   AND source_project.environment_id = source_environment.id
   AND source_project.deleted_at IS NULL
INNER JOIN configs AS source_config
    ON source_config.id = source_snapshot.config_id
   AND source_config.project_id = source_project.id
   AND source_config.deleted_at IS NULL
INNER JOIN environments AS target_environment
    ON target_environment.id = ? AND target_environment.enabled = TRUE
   AND target_environment.deleted_at IS NULL
INNER JOIN projects AS target_project
    ON target_project.id = ? AND target_project.environment_id = target_environment.id
   AND target_project.deleted_at IS NULL
WHERE u.id = ? AND u.enabled = TRUE AND u.deleted_at IS NULL
ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(import_records.id)`,
		record.IdempotencyKey, record.RequestHash, model.ImportStatusPending,
		record.ErrorMessage, record.CreatedBy, record.UpdatedBy,
		record.SourceEnvironmentID, record.SourceSnapshotID,
		record.TargetEnvironmentID, record.TargetProjectID, record.UserID)
	if err != nil {
		return nil, false, base.Wrap("claim import record", err)
	}
	var claimed model.ImportRecord
	err = base.LockOne(ctx, tx, "import record", `
SELECT r.id, r.user_id, r.source_environment_id, r.target_environment_id,
       r.source_snapshot_id, r.target_project_id, r.target_snapshot_id,
       r.idempotency_key, r.request_hash, r.status, r.error_message,
       r.created_by, r.updated_by, r.created_at, r.updated_at, r.version
FROM import_records AS r
WHERE r.user_id = ? AND r.target_environment_id = ? AND r.idempotency_key = ?
LIMIT 1
FOR UPDATE`, []any{record.UserID, record.TargetEnvironmentID, record.IdempotencyKey}, &claimed)
	if err != nil {
		return nil, false, err
	}
	created := claimed.ErrorMessage != "" && claimed.ErrorMessage == record.ErrorMessage
	return &claimed, created, nil
}

func (*ImportRecordRepository) CompleteTx(ctx context.Context, tx *xorm.Session, recordID, targetSnapshotID int64, resultJSON string) error {
	if recordID <= 0 || targetSnapshotID <= 0 || !validJSONObject(resultJSON) {
		return base.InvalidArgument("completed import record is invalid")
	}
	_, err := base.ExecuteTx(ctx, tx, "import record", `
UPDATE import_records
SET target_snapshot_id = ?, status = ?, result_json = ?, error_message = '',
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND status = ?`,
		targetSnapshotID, model.ImportStatusSucceeded, resultJSON, recordID, model.ImportStatusPending)
	return err
}

func validJSONObject(value string) bool {
	var object map[string]any
	return json.Unmarshal([]byte(value), &object) == nil && object != nil
}

func validateImportRecord(record *model.ImportRecord) error {
	if record == nil {
		return base.InvalidArgument("import record is nil")
	}
	if record.UserID <= 0 || record.SourceEnvironmentID <= 0 || record.TargetEnvironmentID <= 0 || record.SourceSnapshotID <= 0 || record.TargetProjectID <= 0 {
		return base.InvalidArgument("import record scope is invalid")
	}
	length := utf8.RuneCountInString(record.IdempotencyKey)
	if length < 16 || length > 128 || !asciiGraphic(record.IdempotencyKey) {
		return base.InvalidArgument("idempotency key is invalid")
	}
	if len(record.RequestHash) != sha256.Size || record.CreatedBy != record.UserID || record.UpdatedBy != record.UserID || len(record.ErrorMessage) < 16 || len(record.ErrorMessage) > 128 || !asciiGraphic(record.ErrorMessage) {
		return base.InvalidArgument("import record metadata is invalid")
	}
	return nil
}

func asciiGraphic(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}
