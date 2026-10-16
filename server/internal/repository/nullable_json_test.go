package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestImportReplayReadsNullableJSONResultAsRawText(t *testing.T) {
	for _, rawJSON := range []any{`{"target_snapshot_id":42}`, nil} {
		engine, mock := newSmokeEngine(t)
		record := claimRecordForSmoke()
		mock.ExpectExec("INSERT INTO `import_records`").WillReturnError(duplicateErrorForSmoke())
		mock.ExpectQuery("SELECT .* FROM `import_records` WHERE .* LIMIT 1 FOR UPDATE").
			WillReturnRows(smokeModelRows(t, importRecordForReplay(501, record.IdempotencyKey, rawJSON)))
		claimed, created, err := NewImportRecordRepository(engine).Claim(context.Background(), record)
		require.NoError(t, err)
		require.False(t, created)
		assertRawJSONValue(t, claimed.ResultJSON, rawJSON)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestAuditDetailsReadNullableJSONObjectAsRawText(t *testing.T) {
	for _, rawJSON := range []any{`{"source_snapshot_id":42}`, nil} {
		engine, mock := newSmokeEngine(t)
		mock.ExpectQuery("SELECT .* FROM `audit_logs` WHERE .* LIMIT 1").
			WillReturnRows(smokeModelRows(t, &model.AuditLog{ID: 17, DetailsJSON: rawJSONPointer(rawJSON)}))
		audit, found, err := engine.CoreDB.Query[*model.AuditLog]().ID(17).Get(context.Background())
		require.NoError(t, err)
		require.True(t, found)
		assertRawJSONValue(t, audit.DetailsJSON, rawJSON)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func assertRawJSONValue(t *testing.T, actual *string, rawJSON any) {
	t.Helper()
	if rawJSON == nil {
		require.Nil(t, actual)
		return
	}
	require.NotNil(t, actual)
	require.JSONEq(t, rawJSON.(string), *actual)
}

func importRecordForReplay(id int64, key string, rawJSON any) *model.ImportRecord {
	return &model.ImportRecord{
		WorkflowMeta: model.WorkflowMeta{
			ID: id,
		},
		IdempotencyKey: key,
		ResultJSON:     rawJSONPointer(rawJSON),
	}
}

func rawJSONPointer(value any) *string {
	if value == nil {
		return nil
	}
	text := value.(string)
	return &text
}
