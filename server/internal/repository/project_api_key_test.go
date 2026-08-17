package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestProjectAPIKeyCreateClassifiesPublicIDCollision(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	session := engine.NewSession()
	t.Cleanup(func() { _ = session.Close() })
	require.NoError(t, session.Begin())
	mock.ExpectExec(`(?s)INSERT INTO project_api_keys`).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate"})
	mock.ExpectRollback()
	now := time.Now().UTC()
	item := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{CreatedAt: now, UpdatedAt: now}, ProjectID: 20,
		PublicID: "kirby_pk_public", Name: "production", SecretHash: make([]byte, 32), SecretSuffix: "abcd", CreatedBy: 9,
	}

	err := NewProjectAPIKeyRepository(engine).CreateTx(context.Background(), session, 5, 20, item)
	assert.ErrorIs(t, err, ErrKeyConflict)
	require.NoError(t, session.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRuntimeCredentialLookupAndUsageAreDatabaseBacked(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	session := engine.NewSession()
	t.Cleanup(func() { _ = session.Close() })
	require.NoError(t, session.Begin())
	mock.ExpectQuery(`(?s)SELECT k\.\*.*k\.public_id = \?.*FOR SHARE`).WithArgs("kirby_pk_public").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "public_id", "secret_hash", "revoked_at"}).AddRow(30, 20, "kirby_pk_public", make([]byte, 32), nil))
	mock.ExpectCommit()

	item, err := NewProjectAPIKeyRepository(engine).LockRuntimeCredential(context.Background(), session, "kirby_pk_public")
	require.NoError(t, err)
	assert.Equal(t, int64(20), item.ProjectID)
	require.NoError(t, session.Commit())

	usedAt := time.Now().UTC()
	mock.ExpectExec(`(?s)UPDATE project_api_keys.*last_used_at = IF\(last_used_at IS NULL OR last_used_at < \?, \?, last_used_at\).*WHERE public_id = \? AND revoked_at IS NULL`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "kirby_pk_public").WillReturnResult(sqlmock.NewResult(0, 0))
	err = NewProjectAPIKeyRepository(engine).MarkUsed(context.Background(), "kirby_pk_public", usedAt)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
