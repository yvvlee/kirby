package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestEnvironmentMemberCannotReceiveRoleWithSystemPermission(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT e\.\*.*FROM environments AS e.*WHERE e\.id = \?.*FOR UPDATE`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "enabled", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(10, "prod", "Production", true, now, now, 0, nil))
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*WHERE u\.id = \?.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "enabled", "is_system_admin", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(7, "alice", "Alice", true, false, now, now, 0, nil))
	mock.ExpectQuery(`(?s)SELECT r\.\* FROM roles AS r WHERE r\.id IN \(\?\).*FOR UPDATE`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "builtin", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(4, "unsafe", "Unsafe", false, now, now, 0, nil))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) AS total.*p\.` + "`key`" + ` LIKE 'system:%'`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectRollback()

	repository := NewUserEnvironmentRoleRepository(engine, NewAuditLogRepository(engine))
	actorID := int64(9)
	err := repository.ReplaceRoles(context.Background(), 10, 7, []int64{4}, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "environment.member.roles.update", ResourceType: "user",
		Result: model.AuditResultSucceeded, RequestID: "request-7",
	})
	if !errors.Is(err, ErrSystemRolePermission) {
		t.Fatalf("system permission role error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRolePermissionUpdateRejectsSystemPermission(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT r\.\*.*FROM roles AS r.*WHERE r\.id = \?.*FOR UPDATE`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "builtin", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(4, "custom", "Custom", false, now, now, 0, nil))
	mock.ExpectQuery(`(?s)SELECT p\.\* FROM permissions AS p WHERE p\.id IN \(\?\).*FOR UPDATE`).
		WithArgs(int64(18)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "description", "created_at", "updated_at"}).
			AddRow(18, "system:user:manage", "Manage users", "", now, now))
	mock.ExpectRollback()

	repository := NewRoleRepository(engine, NewAuditLogRepository(engine))
	actorID := int64(9)
	_, err := repository.UpdatePermissions(context.Background(), 4, []int64{18}, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "role.permissions.update", ResourceType: "role",
		Result: model.AuditResultSucceeded, RequestID: "request-8",
	})
	if !errors.Is(err, ErrSystemRolePermission) {
		t.Fatalf("system permission update error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAssignedRoleCannotBeDeleted(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT r\.\*.*FROM roles AS r.*WHERE r\.id = \?.*FOR UPDATE`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "builtin", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(4, "custom", "Custom", false, now, now, 0, nil))
	mock.ExpectQuery(`(?s)SELECT uer\.user_id, uer\.environment_id.*WHERE uer\.role_id = \?.*FOR UPDATE`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "environment_id"}).AddRow(7, 10))
	mock.ExpectRollback()

	repository := NewRoleRepository(engine, NewAuditLogRepository(engine))
	actorID := int64(9)
	err := repository.Delete(context.Background(), 4, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "role.delete", ResourceType: "role",
		Result: model.AuditResultSucceeded, RequestID: "request-9",
	})
	if !errors.Is(err, ErrRoleInUse) {
		t.Fatalf("assigned role deletion error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
