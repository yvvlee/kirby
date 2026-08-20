package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/core"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

const repositoryMockDSN = "user:password@tcp(localhost:3306)/kirby"

func TestConfigFindRejectsResourceFromAnotherEnvironment(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectQuery(regexp.QuoteMeta(configByIDSQL)).
		WithArgs(int64(99), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id"}))

	config, err := NewConfigRepository(engine).FindByID(context.Background(), 99, 42)

	assert.Nil(t, config)
	assert.ErrorIs(t, err, base.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigUpdateCannotCrossEnvironment(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectExec(`(?s)UPDATE configs AS c.*e\.id = \?`).
		WithArgs("description", false, `{}`, int64(7), int64(42), int64(3), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewConfigRepository(engine).Update(context.Background(), 99, 42, ConfigUpdate{
		Description: "description",
		TypeJSON:    `{}`,
		UpdatedBy:   7,
		Version:     3,
	})

	assert.ErrorIs(t, err, base.ErrNoRowsAffected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigMetadataUpdateTxNeverWritesValue(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectExec(`(?s)UPDATE configs AS c\s+SET c\.description = \?, c\.is_array = \?, c\.type_json = \?, c\.updated_by = \?.*e\.id = \?`).
		WithArgs("description", true, `{"baseType":"INT"}`, int64(7), int64(42), int64(3), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := NewConfigRepository(engine).UpdateTx(context.Background(), tx, 99, 42, ConfigUpdate{
		Description: "description",
		IsArray:     true,
		TypeJSON:    `{"baseType":"INT"}`,
		UpdatedBy:   7,
		Version:     3,
	})

	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigCreateTxLocksProjectThenRestoresSoftDeletedKey(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectQuery(`(?s)SELECT p\.id.*e\.id = \?.*FOR UPDATE`).
		WithArgs(int64(5), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(`(?s)SELECT c\.\*.*c\.project_id = \?.*c\.`+"`key`"+` = \?.*FOR UPDATE`).
		WithArgs(int64(3), "message").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "key", "version", "deleted_at"}).
			AddRow(7, 3, "message", 4, time.Now().UTC()))
	mock.ExpectQuery(`(?s)SELECT s\.id.*s\.config_id = \?.*ORDER BY s\.id ASC.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(`(?s)SELECT e\.id.*e\.config_id = \?.*ORDER BY e\.id ASC.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
	mock.ExpectExec(`(?s)UPDATE structures\s+SET deleted_at = UTC_TIMESTAMP\(6\).*WHERE config_id = \? AND deleted_at IS NULL`).
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE config_enums\s+SET deleted_at = UTC_TIMESTAMP\(6\).*WHERE config_id = \? AND deleted_at IS NULL`).
		WithArgs(int64(9), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE configs\s+SET description = \?.*deleted_at = NULL`).
		WithArgs("new", false, `{"baseType":"STRING"}`, `""`, int64(0), int64(9), int64(9), int64(7), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	item := &model.Config{ProjectID: 3, Key: "message", Description: "new", TypeJSON: `{"baseType":"STRING"}`, Value: `""`, Meta: model.Meta{CreatedBy: 9, UpdatedBy: 9}}

	err := NewConfigRepository(engine).CreateTx(context.Background(), tx, 5, 3, item)

	require.NoError(t, err)
	assert.Equal(t, int64(7), item.ID)
	assert.Equal(t, int64(5), item.Version)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigCreateTxRejectsActiveKey(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectQuery(`(?s)SELECT p\.id.*FOR UPDATE`).WithArgs(int64(5), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(`(?s)SELECT c\.\*.*c\.project_id = \?.*FOR UPDATE`).WithArgs(int64(3), "message").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "key", "deleted_at"}).AddRow(7, 3, "message", nil))
	mock.ExpectRollback()

	err := NewConfigRepository(engine).CreateTx(context.Background(), tx, 5, 3, &model.Config{ProjectID: 3, Key: "message"})

	assert.ErrorIs(t, err, ErrKeyConflict)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigCreateTxClassifiesMySQLDuplicateAsConflict(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectQuery(`(?s)SELECT p\.id.*FOR UPDATE`).WithArgs(int64(5), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(`(?s)SELECT c\.\*.*c\.project_id = \?.*FOR UPDATE`).WithArgs(int64(3), "message").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`(?s)INSERT INTO configs`).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate"})
	mock.ExpectRollback()

	err := NewConfigRepository(engine).CreateTx(context.Background(), tx, 5, 3, &model.Config{ProjectID: 3, Key: "message"})

	assert.ErrorIs(t, err, ErrKeyConflict)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStructureCreateTxRestoresSoftDeletedKeyAndRejectsActiveKey(t *testing.T) {
	for _, test := range []struct {
		name      string
		deletedAt any
		conflict  bool
	}{
		{name: "restore", deletedAt: time.Now().UTC()},
		{name: "active", deletedAt: nil, conflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, mock := newRepositoryMockEngine(t)
			mock.ExpectBegin()
			tx := engine.NewSession()
			t.Cleanup(func() { _ = tx.Close() })
			require.NoError(t, tx.Begin())
			mock.ExpectQuery(`(?s)SELECT c\.id.*e\.id = \?.*FOR UPDATE`).WithArgs(int64(5), int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			mock.ExpectQuery(`(?s)SELECT s\.\*.*s\.config_id = \?.*FOR UPDATE`).WithArgs(int64(7), "User").
				WillReturnRows(sqlmock.NewRows([]string{"id", "config_id", "key", "version", "deleted_at"}).AddRow(8, 7, "User", 2, test.deletedAt))
			if test.conflict {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec(`(?s)UPDATE structures\s+SET name = \?.*deleted_at = NULL`).
					WithArgs("User", "new", "[]", int64(9), int64(9), int64(8), int64(7)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			item := &model.Structure{ConfigID: 7, Key: "User", Name: "User", Description: "new", FieldsJSON: "[]", Meta: model.Meta{CreatedBy: 9, UpdatedBy: 9}}
			err := NewStructureRepository(engine).CreateTx(context.Background(), tx, 5, 7, item)
			if test.conflict {
				assert.ErrorIs(t, err, ErrKeyConflict)
				require.NoError(t, tx.Rollback())
			} else {
				require.NoError(t, err)
				assert.Equal(t, int64(8), item.ID)
				assert.Equal(t, int64(3), item.Version)
				require.NoError(t, tx.Commit())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestConfigEnumCreateTxRestoresSoftDeletedKeyAndRejectsActiveKey(t *testing.T) {
	for _, test := range []struct {
		name      string
		deletedAt any
		conflict  bool
	}{
		{name: "restore", deletedAt: time.Now().UTC()},
		{name: "active", deletedAt: nil, conflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, mock := newRepositoryMockEngine(t)
			mock.ExpectBegin()
			tx := engine.NewSession()
			t.Cleanup(func() { _ = tx.Close() })
			require.NoError(t, tx.Begin())
			mock.ExpectQuery(`(?s)SELECT c\.id.*e\.id = \?.*FOR UPDATE`).WithArgs(int64(5), int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			mock.ExpectQuery(`(?s)SELECT e\.\*.*e\.config_id = \?.*FOR UPDATE`).WithArgs(int64(7), "Status").
				WillReturnRows(sqlmock.NewRows([]string{"id", "config_id", "key", "version", "deleted_at"}).AddRow(8, 7, "Status", 2, test.deletedAt))
			if test.conflict {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec(`(?s)UPDATE config_enums\s+SET name = \?.*deleted_at = NULL`).
					WithArgs("Status", "new", `[{"label":"Active","value":"ACTIVE"}]`, int64(9), int64(9), int64(8), int64(7)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			item := &model.ConfigEnum{ConfigID: 7, Key: "Status", Name: "Status", Description: "new", ValuesJSON: `[{"label":"Active","value":"ACTIVE"}]`, Meta: model.Meta{CreatedBy: 9, UpdatedBy: 9}}
			err := NewConfigEnumRepository(engine).CreateTx(context.Background(), tx, 5, 7, item)
			if test.conflict {
				assert.ErrorIs(t, err, ErrKeyConflict)
				require.NoError(t, tx.Rollback())
			} else {
				require.NoError(t, err)
				assert.Equal(t, int64(8), item.ID)
				assert.Equal(t, int64(3), item.Version)
				require.NoError(t, tx.Commit())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNestedCreateUsesEnvironmentInInsertQuery(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectExec(`(?s)INSERT INTO structures.*e\.id = \?`).
		WithArgs("User", "User", "", `[]`, int64(7), int64(7), int64(11), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewStructureRepository(engine).Create(context.Background(), 99, 11, &model.Structure{
		Meta:       model.Meta{CreatedBy: 7, UpdatedBy: 7},
		Key:        "User",
		Name:       "User",
		FieldsJSON: `[]`,
	})

	assert.ErrorIs(t, err, base.ErrNoRowsAffected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStructureReconcileSoftDeletesRowsMissingFromSnapshot(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectQuery(`(?s)SELECT c\.id.*e\.id = \?.*FOR UPDATE`).
		WithArgs(int64(5), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(`(?s)SELECT s\.\*.*WHERE s\.config_id = \?.*FOR UPDATE`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "config_id", "key", "name", "description", "fields_json", "version", "deleted_at"}).
			AddRow(12, 9, "User", "User", "", "[]", 2, nil))
	mock.ExpectExec(`(?s)UPDATE structures\s+SET deleted_at = UTC_TIMESTAMP`).
		WithArgs(int64(7), int64(12), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := NewStructureRepository(engine).ReconcileTx(context.Background(), tx, 5, 9, nil, 7)

	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectListCapsPageSizeAndOrdersDeterministically(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	countSQL := `
SELECT COUNT(*) AS total
FROM projects AS p
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND p.deleted_at IS NULL
  AND (p.name LIKE ? OR p.description LIKE ?)`
	listSQL := `
SELECT p.*
FROM projects AS p
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND p.deleted_at IS NULL
  AND (p.name LIKE ? OR p.description LIKE ?)
ORDER BY p.id DESC
LIMIT ? OFFSET ?`
	mock.ExpectQuery(regexp.QuoteMeta(countSQL)).
		WithArgs(int64(5), "%search%", "%search%").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(150))
	mock.ExpectQuery(regexp.QuoteMeta(listSQL)).
		WithArgs(int64(5), "%search%", "%search%", base.MaxPageSize, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name"}).
			AddRow(10, "demo", "Demo"))

	result, err := NewProjectRepository(engine).List(context.Background(), 5, "search", base.PageRequest{Limit: 1000})

	require.NoError(t, err)
	assert.Equal(t, base.MaxPageSize, result.Limit)
	assert.Equal(t, int64(150), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, int64(10), result.Items[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSnapshotLockUsesForUpdateInsideTransaction(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	tx := engine.NewSession()
	t.Cleanup(func() { _ = tx.Close() })
	require.NoError(t, tx.Begin())
	mock.ExpectQuery(regexp.QuoteMeta(snapshotByIDSQL+"\nFOR UPDATE")).
		WithArgs(int64(5), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "config_id", "status"}).
			AddRow(12, 8, 9, 1))
	mock.ExpectCommit()

	snapshot, err := NewSnapshotRepository(engine).LockByID(context.Background(), tx, 5, 12)

	require.NoError(t, err)
	assert.Equal(t, int64(12), snapshot.ID)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockFailsWithoutTransactionSession(t *testing.T) {
	engine, _ := newRepositoryMockEngine(t)
	_, err := NewProjectRepository(engine).LockByID(context.Background(), nil, 1, 1)
	assert.ErrorIs(t, err, base.ErrInvalidArgument)
}

func TestRepositoryPreservesContextCancellation(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectQuery(regexp.QuoteMeta(configByIDSQL)).
		WithArgs(int64(1), int64(2)).
		WillReturnError(context.Canceled)

	_, err := NewConfigRepository(engine).FindByID(context.Background(), 1, 2)
	assert.True(t, errors.Is(err, context.Canceled))
}

func newRepositoryMockEngine(t *testing.T) (*xorm.Engine, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	engine, err := xorm.NewEngineWithDB("mysql", repositoryMockDSN, core.FromDB(db))
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	return engine, mock
}
