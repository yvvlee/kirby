package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/infra"
)

func newPostgresClaimMockEngine(t *testing.T) (*infra.Storage, sqlmock.Sqlmock) {
	t.Helper()
	return newDialectSmokeEngine(t, "pgx")
}

func TestPostgresImportClaimOnlyIgnoresItsIdempotencyConflict(t *testing.T) {
	engine, mock := newPostgresClaimMockEngine(t)
	record := claimRecordForSmoke()
	mock.ExpectQuery(`(?s)INSERT INTO import_records.*ON CONFLICT \(user_id, target_environment_id, idempotency_key\) DO NOTHING.*RETURNING \*`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT .* FROM "import_records" WHERE .* LIMIT 1 FOR UPDATE`).
		WillReturnRows(smokeModelRows(t, importRecordForReplay(501, record.IdempotencyKey, nil)))
	claimed, created, err := NewImportRecordRepository(engine).Claim(context.Background(), record)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(501), claimed.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresImportClaimDoesNotSwallowOtherDatabaseErrors(t *testing.T) {
	engine, mock := newPostgresClaimMockEngine(t)
	failure := errors.New("database write unavailable")
	mock.ExpectQuery(`(?s)INSERT INTO import_records.*ON CONFLICT .*DO NOTHING.*RETURNING \*`).WillReturnError(failure)
	_, _, err := NewImportRecordRepository(engine).Claim(context.Background(), claimRecordForSmoke())
	require.ErrorContains(t, err, failure.Error())
	require.NoError(t, mock.ExpectationsWereMet())
}
