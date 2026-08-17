package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestImportRecordClaimBindsSourceAndTargetScope(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	session := engine.NewSession()
	t.Cleanup(func() { _ = session.Close() })
	require.NoError(t, session.Begin())
	claimToken := "00000000-0000-4000-8000-000000000001"
	hash := make([]byte, 32)
	record := &model.ImportRecord{
		WorkflowMeta: model.WorkflowMeta{CreatedBy: 9, UpdatedBy: 9}, UserID: 9,
		SourceEnvironmentID: 1, TargetEnvironmentID: 2, SourceSnapshotID: 12, TargetProjectID: 20,
		IdempotencyKey: "request-00000001", RequestHash: hash, Status: model.ImportStatusPending, ErrorMessage: claimToken,
	}
	mock.ExpectExec(`(?s)INSERT INTO import_records.*source_snapshot\.id.*source_project\.environment_id = source_environment\.id.*target_project\.environment_id = target_environment\.id.*ON DUPLICATE KEY UPDATE`).
		WithArgs(record.IdempotencyKey, hash, model.ImportStatusPending, claimToken, int64(9), int64(9), int64(1), int64(12), int64(2), int64(20), int64(9)).
		WillReturnResult(sqlmock.NewResult(501, 1))
	mock.ExpectQuery(`(?s)SELECT r\.id.*r\.user_id = \?.*r\.target_environment_id = \?.*r\.idempotency_key = \?.*FOR UPDATE`).
		WithArgs(int64(9), int64(2), record.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_environment_id", "target_environment_id", "source_snapshot_id", "target_project_id", "idempotency_key", "request_hash", "status", "error_message", "created_by", "updated_by"}).
			AddRow(501, 9, 1, 2, 12, 20, record.IdempotencyKey, hash, model.ImportStatusPending, claimToken, 9, 9))
	mock.ExpectCommit()

	claimed, created, err := NewImportRecordRepository().ClaimTx(context.Background(), session, record)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, int64(501), claimed.ID)
	require.NoError(t, session.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportRecordClaimUsesMarkerInsteadOfRowsAffected(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	session := engine.NewSession()
	t.Cleanup(func() { _ = session.Close() })
	require.NoError(t, session.Begin())
	claimToken := "00000000-0000-4000-8000-000000000001"
	targetSnapshotID := int64(400)
	hash := make([]byte, 32)
	record := &model.ImportRecord{WorkflowMeta: model.WorkflowMeta{CreatedBy: 9, UpdatedBy: 9}, UserID: 9, SourceEnvironmentID: 1, TargetEnvironmentID: 2, SourceSnapshotID: 12, TargetProjectID: 20, IdempotencyKey: "request-00000001", RequestHash: hash, Status: model.ImportStatusPending, ErrorMessage: claimToken}
	mock.ExpectExec(`(?s)INSERT INTO import_records`).WillReturnResult(sqlmock.NewResult(501, 1))
	mock.ExpectQuery(`(?s)SELECT r\.id.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_environment_id", "target_environment_id", "source_snapshot_id", "target_project_id", "target_snapshot_id", "idempotency_key", "request_hash", "status", "error_message", "created_by", "updated_by"}).
			AddRow(501, 9, 1, 2, 12, 20, targetSnapshotID, record.IdempotencyKey, hash, model.ImportStatusSucceeded, "", 9, 9))
	mock.ExpectRollback()

	claimed, created, err := NewImportRecordRepository().ClaimTx(context.Background(), session, record)
	require.NoError(t, err)
	assert.False(t, created, "a duplicate must not be treated as new even when the driver reports one affected row")
	assert.Equal(t, model.ImportStatusSucceeded, claimed.Status)
	require.NoError(t, session.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportRecordCompletionIsGuardedByPendingStatus(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	session := engine.NewSession()
	t.Cleanup(func() { _ = session.Close() })
	require.NoError(t, session.Begin())
	result := `{"target_snapshot_id":400}`
	mock.ExpectExec(`(?s)UPDATE import_records.*target_snapshot_id = \?.*status = \?.*WHERE id = \? AND status = \?`).
		WithArgs(int64(400), model.ImportStatusSucceeded, result, int64(501), model.ImportStatusPending).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, NewImportRecordRepository().CompleteTx(context.Background(), session, 501, 400, result))
	require.NoError(t, session.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
