package repository

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

func newSmokeEngine(t *testing.T) (*infra.Storage, sqlmock.Sqlmock) {
	t.Helper()
	return newDialectSmokeEngine(t, "mysql")
}

func newDialectSmokeEngine(t *testing.T, dialect string) (*infra.Storage, sqlmock.Sqlmock) {
	t.Helper()
	dsn := uuid.NewString()
	db, mock, err := sqlmock.NewWithDSN(dsn)
	require.NoError(t, err)
	engine, err := lorm.NewEngine("sqlmock", dsn, lorm.WithDialectConfig(lorm.DefaultDialectConfig(dialect)), lorm.WithLogger(nil))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = engine.Close()
		_ = db.Close()
	})
	return &infra.Storage{
		CoreDB: engine,
	}, mock
}

func smokeModelRows(t testing.TB, item interface {
	lorm.Model
	lorm.ModelFieldValueAccessor
}) *sqlmock.Rows {
	t.Helper()
	fields := item.LormModelDescriptor().Fields
	columns := make([]string, len(fields))
	values := make([]driver.Value, len(fields))
	for index, field := range fields {
		columns[index] = field.DBField
		value, err := driver.DefaultParameterConverter.ConvertValue(item.LormFieldValue(field.DBField))
		require.NoError(t, err)
		values[index] = value
	}
	return sqlmock.NewRows(columns).AddRow(values...)
}

const smokeDeleted = "`deleted_at` IS NULL"

// TestSmokeGeneratedSQL校验关键链式查询生成的SQL形状：环境作用域JOIN、软删除过滤与受保护更新。
func TestSmokeGeneratedSQL(t *testing.T) {
	ctx := context.Background()
	t.Run("config find scoped joins", func(t *testing.T) {
		storage, mock := newSmokeEngine(t)
		mock.ExpectQuery("SELECT c\\.\\* FROM configs AS c INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL WHERE `c`.`deleted_at` IS NULL AND `c`.`id` = \\? AND `e`.`id` = \\? LIMIT 1").
			WithArgs(int64(42), int64(5)).
			WillReturnRows(smokeModelRows(t, &model.Config{Meta: model.Meta{ID: 42}}))
		_, err := NewConfigRepository(storage).FindByID(ctx, 5, 42)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("snapshot list counts with joins", func(t *testing.T) {
		storage, mock := newSmokeEngine(t)
		using := true
		mock.ExpectQuery("SELECT COUNT\\(1\\) FROM snapshots AS s INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL WHERE `s`.`deleted_at` IS NULL AND \\(`c`.`id` = \\? AND `e`.`id` = \\? AND `p`.`id` = \\? AND `s`.`is_using` = \\?\\)").
			WithArgs(int64(9), int64(5), int64(3), true).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
		_, err := NewSnapshotRepository(storage).List(ctx, 5, SnapshotFilter{
			ProjectID: 3,
			ConfigID:  9,
			IsUsing:   &using,
		}, PageRequest{})
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("set current unsets others then sets target", func(t *testing.T) {
		storage, mock := newSmokeEngine(t)
		mock.ExpectQuery("SELECT c\\.\\* FROM configs AS c .* WHERE `c`.`deleted_at` IS NULL AND `c`.`id` = \\? AND `e`.`id` = \\? LIMIT 1").
			WithArgs(int64(9), int64(5)).WillReturnRows(smokeModelRows(t, &model.Config{
			Meta:      model.Meta{ID: 9},
			ProjectID: 3,
		}))
		mock.ExpectQuery("SELECT .* FROM `snapshots` WHERE `deleted_at` IS NULL AND `config_id` = \\? AND `id` = \\? AND `project_id` = \\? LIMIT 1").
			WithArgs(int64(9), int64(11), int64(3)).WillReturnRows(smokeModelRows(t, &model.Snapshot{Meta: model.Meta{ID: 11}}))
		mock.ExpectExec("UPDATE `snapshots` SET .*version.* WHERE `deleted_at` IS NULL AND `config_id` = \\? AND `project_id` = \\? AND `id` <> \\? AND `is_using` = \\?").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE `snapshots` SET .*version.* WHERE `deleted_at` IS NULL AND `id` = \\? AND `config_id` = \\? AND `project_id` = \\?").
			WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, NewSnapshotRepository(storage).SetCurrent(ctx, 5, 9, 11, 7))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("role update permissions flow", testRolePermissionsSQL)
	t.Run("permission identity reads", testPermissionIdentitySQL)
	t.Run("import claim falls back to locked read on duplicate key", func(t *testing.T) {
		storage, mock := newSmokeEngine(t)
		record := claimRecordForSmoke()
		mock.ExpectExec("INSERT INTO `import_records`").WillReturnError(duplicateErrorForSmoke())
		record.ID = 501
		mock.ExpectQuery("SELECT .* FROM `import_records` WHERE `idempotency_key` = \\? AND `target_environment_id` = \\? AND `user_id` = \\? LIMIT 1 FOR UPDATE").
			WithArgs(record.IdempotencyKey, record.TargetEnvironmentID, record.UserID).
			WillReturnRows(smokeModelRows(t, record))
		claimed, created, err := NewImportRecordRepository(storage).Claim(ctx, claimRecordForSmoke())
		require.NoError(t, err)
		require.False(t, created)
		require.Equal(t, int64(501), claimed.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func testRolePermissionsSQL(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectQuery("SELECT .* FROM `roles` WHERE .* LIMIT 1 FOR UPDATE").
		WillReturnRows(smokeModelRows(t, &model.Role{Meta: model.Meta{ID: 3}}))
	mock.ExpectQuery("SELECT .* FROM `permissions` WHERE .* FOR UPDATE").
		WillReturnRows(smokeModelRows(t, &model.Permission{ID: 1, Key: "project:read"}))
	mock.ExpectQuery("SELECT .* FROM `user_environment_roles` WHERE .* FOR UPDATE").
		WillReturnRows(smokeModelRows(t, &model.UserEnvironmentRole{UserID: 7, EnvironmentID: 5}))
	mock.ExpectExec("DELETE FROM `role_permissions` WHERE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `role_permissions`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `roles` SET .* WHERE .*`deleted_at` IS NULL").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `environments` SET .* WHERE .*`deleted_at` IS NULL").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `audit_logs`").WillReturnResult(sqlmock.NewResult(17, 1))
	assignments, err := NewRoleRepository(storage).UpdatePermissions(context.Background(), 3, []int64{1}, 9, &model.AuditLog{
		Action: "role.permissions.update",
	})
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func testPermissionIdentitySQL(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectQuery("SELECT .* FROM `users` WHERE .* LIMIT 1").
		WillReturnRows(smokeModelRows(t, &model.User{Meta: model.Meta{ID: 7}}))
	mock.ExpectQuery("SELECT .* FROM `environments` WHERE .* LIMIT 1").
		WillReturnRows(smokeModelRows(t, &model.Environment{Meta: model.Meta{ID: 5, Version: 4}, Enabled: true}))
	mock.ExpectQuery("SELECT .* FROM `user_environment_roles` WHERE .* LIMIT 1").
		WillReturnRows(smokeModelRows(t, &model.UserEnvironmentRole{ID: 1}))
	identity, err := NewPermissionRepository(storage).Identity(context.Background(), 7, 5)
	require.NoError(t, err)
	require.True(t, identity.EnvironmentMember)
	require.Equal(t, int64(5), identity.EnvironmentID)
	require.Equal(t, int64(4), identity.EnvironmentVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func duplicateErrorForSmoke() error {
	return &mysql.MySQLError{
		Number:  1062,
		Message: "duplicate",
	}
}

func claimRecordForSmoke() *model.ImportRecord {
	hash := make([]byte, 32)
	return &model.ImportRecord{
		WorkflowMeta: model.WorkflowMeta{
			CreatedBy: 9,
			UpdatedBy: 9,
		},
		UserID:              9,
		SourceEnvironmentID: 1,
		TargetEnvironmentID: 2,
		SourceSnapshotID:    12,
		TargetProjectID:     20,
		IdempotencyKey:      "request-00000001",
		RequestHash:         hash,
		Status:              model.ImportStatusPending,
		ErrorMessage:        "claim-token",
	}
}
