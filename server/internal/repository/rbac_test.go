package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestPermissionIdentityReturnsEnvironmentGeneration(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectQuery(`(?s)SELECT u\.is_system_admin,.*COALESCE\(e\.version, 0\) AS environment_version.*FROM users AS u`).
		WithArgs(int64(10), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"is_system_admin", "environment_id", "environment_version", "environment_enabled", "environment_member",
		}).AddRow(false, 10, 42, true, true))

	identity, err := NewPermissionRepository(engine).Identity(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if identity.EnvironmentVersion != 42 || !identity.EnvironmentMember || identity.EnvironmentID != 10 {
		t.Fatalf("permission identity = %#v", identity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMemberRoleRevocationAdvancesEnvironmentGenerationInTransaction(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT e\.\*.*FROM environments AS e.*WHERE e\.id = \?.*FOR UPDATE`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "enabled", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(10, "prod", "Production", true, now, now, 4, nil))
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*WHERE u\.id = \?.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "enabled", "is_system_admin", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(7, "alice", "Alice", true, false, now, now, 0, nil))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM user_environment_roles WHERE user_id = ? AND environment_id = ?")).
		WithArgs(int64(7), int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE environments.*SET updated_by = \?, updated_at = UTC_TIMESTAMP\(6\), version = version \+ 1.*WHERE id = \?`).
		WithArgs(int64(9), int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO audit_logs.*SELECT \?, e\.id, \?, \?, \?, \?, \?, \?.*WHERE e\.id = \?`).
		WithArgs(int64(9), "environment.member.roles.update", "user", "7", model.AuditResultSucceeded, "request-revoke", nil, int64(10)).
		WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectCommit()

	repository := NewUserEnvironmentRoleRepository(engine, NewAuditLogRepository(engine))
	actorID := int64(9)
	err := repository.ReplaceRoles(context.Background(), 10, 7, nil, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "environment.member.roles.update", ResourceType: "user",
		Result: model.AuditResultSucceeded, RequestID: "request-revoke",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRolePermissionShrinkAdvancesEveryUsedEnvironmentGenerationInTransaction(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT r\.\*.*FROM roles AS r.*WHERE r\.id = \?.*FOR UPDATE`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "builtin", "created_at", "updated_at", "version", "deleted_at"}).
			AddRow(4, "editor", "Editor", false, now, now, 3, nil))
	mock.ExpectQuery(`(?s)SELECT p\.\* FROM permissions AS p WHERE p\.id IN \(\?\).*FOR UPDATE`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name", "description", "created_at", "updated_at"}).
			AddRow(1, "project:read", "Read projects", "", now, now))
	mock.ExpectQuery(`(?s)SELECT DISTINCT uer\.user_id, uer\.environment_id.*INNER JOIN environments AS e.*WHERE uer\.role_id = \?.*FOR UPDATE`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "environment_id"}).
			AddRow(7, 20).
			AddRow(8, 10).
			AddRow(9, 10))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM role_permissions WHERE role_id = ?")).
		WithArgs(int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO role_permissions (role_id, permission_id, created_by) VALUES (?, ?, ?)")).
		WithArgs(int64(4), int64(1), int64(11)).
		WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectExec(`(?s)UPDATE roles.*SET updated_by = \?, updated_at = UTC_TIMESTAMP\(6\), version = version \+ 1.*WHERE id = \?`).
		WithArgs(int64(11), int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, environmentID := range []int64{10, 20} {
		mock.ExpectExec(`(?s)UPDATE environments.*SET updated_by = \?, updated_at = UTC_TIMESTAMP\(6\), version = version \+ 1.*WHERE id = \?`).
			WithArgs(int64(11), environmentID).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec(`(?s)INSERT INTO audit_logs.*VALUES \(\?, NULL, \?, \?, \?, \?, \?, \?\)`).
		WithArgs(int64(11), "role.permissions.update", "role", "4", model.AuditResultSucceeded, "request-shrink", nil).
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectCommit()

	repository := NewRoleRepository(engine, NewAuditLogRepository(engine))
	actorID := int64(11)
	assignments, err := repository.UpdatePermissions(context.Background(), 4, []int64{1}, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "role.permissions.update", ResourceType: "role",
		Result: model.AuditResultSucceeded, RequestID: "request-shrink",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 3 {
		t.Fatalf("role assignments = %#v", assignments)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

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
