package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

func TestSnapshotPublicationLocksAllConfigSnapshotsInStableOrder(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectQuery(regexp.QuoteMeta(lockSnapshotsForConfigSQL)).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "config_id", "config_key", "status", "version"}).
			AddRow(11, 7, 9, "feature", model.SnapshotStatusUnreleased, 2).
			AddRow(12, 7, 9, "feature", model.SnapshotStatusReleased, 4))
	mock.ExpectCommit()

	items, err := NewSnapshotPublicationRepository(engine).LockForConfig(context.Background(), tx, 5, 9)

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, int64(11), items[0].ID)
	assert.Equal(t, int64(12), items[1].ID)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSnapshotPublicationRequiresTransaction(t *testing.T) {
	engine, _ := newRepositoryMockEngine(t)
	_, err := NewSnapshotPublicationRepository(engine).LockForConfig(context.Background(), nil, 5, 9)
	assert.ErrorIs(t, err, base.ErrInvalidArgument)
}

func TestSnapshotPublicationWritesAreEnvironmentVersionAndStatusGuarded(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	changedAt := time.Date(2026, time.August, 17, 4, 5, 6, 0, time.UTC)
	mock.ExpectExec(`(?s)UPDATE snapshots AS s.*SET s\.status = \?, s\.published_at = \?.*WHERE p\.environment_id = \? AND c\.id = \? AND s\.id = \?.*s\.status = \? AND s\.version = \?`).
		WithArgs(model.SnapshotStatusReleased, sqlmock.AnyArg(), int64(6), int64(6), sqlmock.AnyArg(),
			int64(5), int64(9), int64(12), model.SnapshotStatusUnreleased, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE snapshots AS s.*SET s\.status = \?, s\.published_at = NULL.*WHERE p\.environment_id = \? AND c\.id = \? AND s\.id = \?.*s\.status = \? AND s\.version = \?`).
		WithArgs(model.SnapshotStatusUnreleased, int64(6), sqlmock.AnyArg(),
			int64(5), int64(9), int64(12), model.SnapshotStatusReleased, int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repository := NewSnapshotPublicationRepository(engine)
	require.NoError(t, repository.SetReleased(context.Background(), tx, 5, 9, 12, 3, 6, changedAt))
	require.NoError(t, repository.SetUnreleased(context.Background(), tx, 5, 9, 12, 4, 6, changedAt))
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSnapshotPublicationRejectsConcurrentGuardFailure(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	changedAt := time.Date(2026, time.August, 17, 4, 5, 6, 0, time.UTC)
	mock.ExpectExec(`(?s)UPDATE snapshots AS s.*p\.environment_id = \?.*s\.version = \?`).
		WithArgs(model.SnapshotStatusReleased, sqlmock.AnyArg(), int64(6), int64(6), sqlmock.AnyArg(),
			int64(5), int64(9), int64(12), model.SnapshotStatusUnreleased, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := NewSnapshotPublicationRepository(engine).SetReleased(context.Background(), tx, 5, 9, 12, 3, 6, changedAt)

	assert.ErrorIs(t, err, base.ErrNoRowsAffected)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSnapshotPublicationIncrementsOnlyScopedRuntimeVersion(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectExec(`(?s)UPDATE configs AS c.*SET c\.runtime_version = c\.runtime_version \+ 1.*p\.environment_id = \? AND c\.id = \?.*runtime_version < 9223372036854775807`).
		WithArgs(int64(5), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := NewSnapshotPublicationRepository(engine).IncrementRuntimeVersion(context.Background(), tx, 5, 9)

	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
