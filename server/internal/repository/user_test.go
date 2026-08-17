package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestLastEnabledSystemAdministratorCannotBeRemoved(t *testing.T) {
	admins := []model.User{{Meta: model.Meta{ID: 7}, Enabled: true, IsSystemAdmin: true}}
	if err := ensureSystemAdminSurvives(admins, 7); !errors.Is(err, ErrLastSystemAdmin) {
		t.Fatalf("last administrator removal error = %v", err)
	}

	admins = append(admins, model.User{Meta: model.Meta{ID: 8}, Enabled: true, IsSystemAdmin: true})
	if err := ensureSystemAdminSurvives(admins, 7); err != nil {
		t.Fatalf("removing one of two administrators failed: %v", err)
	}
}

func TestDisabledOrDeletedAdministratorsDoNotSatisfyProtection(t *testing.T) {
	admins := []model.User{
		{Meta: model.Meta{ID: 1}, Enabled: true, IsSystemAdmin: true},
		{Meta: model.Meta{ID: 2}, Enabled: false, IsSystemAdmin: true},
		{Meta: model.Meta{ID: 3, DeletedAt: time.Now()}, Enabled: true, IsSystemAdmin: true},
	}
	if err := ensureSystemAdminSurvives(admins, 1); !errors.Is(err, ErrLastSystemAdmin) {
		t.Fatalf("unusable administrators bypassed protection: %v", err)
	}
}

func TestManagedLastAdministratorGuardAndAuditShareTransaction(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	userColumns := []string{
		"id", "username", "display_name", "password_hash", "enabled", "is_system_admin",
		"created_by", "updated_by", "created_at", "updated_at", "version", "deleted_at",
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*WHERE u\.id = \?.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(7, "admin", "Admin", "hash", true, true, 1, 1, now, now, 3, nil))
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*enabled = TRUE.*is_system_admin = TRUE.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(7, "admin", "Admin", "hash", true, true, 1, 1, now, now, 3, nil))
	mock.ExpectRollback()

	repository, err := NewUserRepository(engine)
	require.NoError(t, err)
	actorID := int64(9)
	_, err = repository.UpdateManagedStatus(context.Background(), 7, false, 3, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "user.status.update", ResourceType: "user",
		Result: model.AuditResultSucceeded, RequestID: "request-5",
	})
	require.ErrorIs(t, err, ErrLastSystemAdmin)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManagedAdministratorDemotionCommitsAuditInSameTransaction(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	userColumns := []string{
		"id", "username", "display_name", "password_hash", "enabled", "is_system_admin",
		"created_by", "updated_by", "created_at", "updated_at", "version", "deleted_at",
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*WHERE u\.id = \?.*FOR UPDATE`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(7, "admin", "Admin", "hash", true, true, 1, 1, now, now, 3, nil))
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*enabled = TRUE.*is_system_admin = TRUE.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows(userColumns).
			AddRow(7, "admin", "Admin", "hash", true, true, 1, 1, now, now, 3, nil).
			AddRow(8, "backup", "Backup", "hash", true, true, 1, 1, now, now, 2, nil))
	mock.ExpectExec(`(?s)UPDATE users.*SET display_name = \?, is_system_admin = \?.*WHERE id = \? AND version = \?`).
		WithArgs("Admin", false, int64(9), int64(7), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO audit_logs.*VALUES \(\?, NULL, \?, \?, \?, \?, \?, \?\)`).
		WithArgs(int64(9), "user.update", "user", "7", model.AuditResultSucceeded, "request-6", nil).
		WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectCommit()

	repository, err := NewUserRepository(engine)
	require.NoError(t, err)
	actorID := int64(9)
	updated, err := repository.UpdateManagedUser(context.Background(), 7, "Admin", false, 3, actorID, &model.AuditLog{
		ActorUserID: &actorID, Action: "user.update", ResourceType: "user",
		Result: model.AuditResultSucceeded, RequestID: "request-6",
	})
	require.NoError(t, err)
	require.False(t, updated.IsSystemAdmin)
	require.Equal(t, int64(4), updated.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManagedUserListUsesDeterministicOrderAndFixedLimit(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "display_name", "password_hash", "enabled", "is_system_admin",
		"created_at", "updated_at", "version", "deleted_at",
	}).
		AddRow(2, "alice", "Alice", "hash", true, false, now, now, 0, nil).
		AddRow(7, "admin", "Admin", "hash", true, true, now, now, 0, nil)
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*ORDER BY u\.id ASC.*LIMIT \?`).
		WithArgs(managedUserListLimit + 1).
		WillReturnRows(rows)

	repository, err := NewUserRepository(engine)
	require.NoError(t, err)
	users, err := repository.ListManagedUsers(context.Background())
	require.NoError(t, err)
	require.Len(t, users, 2)
	require.Equal(t, int64(2), users[0].ID)
	require.Equal(t, int64(7), users[1].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManagedUserListFailsWhenFixedLimitIsExceeded(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	rows := sqlmock.NewRows([]string{"id"})
	for id := 1; id <= managedUserListLimit+1; id++ {
		rows.AddRow(id)
	}
	mock.ExpectQuery(`(?s)SELECT u\.\*.*FROM users AS u.*ORDER BY u\.id ASC.*LIMIT \?`).
		WithArgs(managedUserListLimit + 1).
		WillReturnRows(rows)

	repository, err := NewUserRepository(engine)
	require.NoError(t, err)
	_, err = repository.ListManagedUsers(context.Background())
	require.ErrorIs(t, err, ErrUserListLimit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAdminRevokesExistingSessionsForResetRestoreAndPromotion(t *testing.T) {
	now := time.Now().UTC()
	userColumns := []string{
		"id", "username", "display_name", "password_hash", "enabled", "is_system_admin",
		"created_by", "updated_by", "created_at", "updated_at", "version", "deleted_at",
	}
	tests := []struct {
		name        string
		enabled     bool
		systemAdmin bool
		deletedAt   any
	}{
		{name: "reset existing administrator", enabled: true, systemAdmin: true},
		{name: "restore deleted administrator", enabled: false, systemAdmin: true, deletedAt: now.Add(-time.Hour)},
		{name: "promote ordinary user", enabled: true, systemAdmin: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine, mock := newRepositoryMockEngine(t)
			mock.ExpectBegin()
			mock.ExpectQuery(`(?s)SELECT .* FROM ` + "`users`" + ` WHERE .*username.*FOR UPDATE`).
				WithArgs("admin").
				WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
					7, "admin", "Old name", "old-hash", test.enabled, test.systemAdmin,
					1, 1, now, now, 3, test.deletedAt,
				))
			mock.ExpectExec(regexp.QuoteMeta(
				"UPDATE users SET display_name = ?, password_hash = ?, enabled = TRUE, "+
					"is_system_admin = TRUE, deleted_at = NULL, updated_at = ?, version = version + 1 WHERE id = ?",
			)).
				WithArgs("Administrator", "new-hash", sqlmock.AnyArg(), int64(7)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(regexp.QuoteMeta(
				"UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(6)), updated_at = UTC_TIMESTAMP(6) WHERE user_id = ?",
			)).
				WithArgs(int64(7)).
				WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectQuery(`(?s)SELECT .* FROM ` + "`users`" + ` WHERE ` + "`id`" + `=\? LIMIT 1`).
				WithArgs(int64(7)).
				WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
					7, "admin", "Administrator", "new-hash", true, true,
					1, 1, now, now, 4, nil,
				))
			mock.ExpectCommit()

			repository, err := NewUserRepository(engine)
			require.NoError(t, err)
			user, created, err := repository.CreateOrPromoteSystemAdmin(
				context.Background(), "admin", "Administrator", "new-hash",
			)
			require.NoError(t, err)
			require.False(t, created)
			require.True(t, user.Enabled)
			require.True(t, user.IsSystemAdmin)
			require.Equal(t, "new-hash", user.PasswordHash)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreateAdminRollsBackPasswordWhenSessionRevocationFails(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Now().UTC()
	userColumns := []string{
		"id", "username", "display_name", "password_hash", "enabled", "is_system_admin",
		"created_by", "updated_by", "created_at", "updated_at", "version", "deleted_at",
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .* FROM ` + "`users`" + ` WHERE .*username.*FOR UPDATE`).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
			7, "admin", "Old name", "old-hash", true, true, 1, 1, now, now, 3, nil,
		))
	mock.ExpectExec(`(?s)UPDATE users SET display_name = \?, password_hash = \?.*WHERE id = \?`).
		WithArgs("Administrator", "new-hash", sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE refresh_tokens SET revoked_at = COALESCE.*WHERE user_id = \?`).
		WithArgs(int64(7)).
		WillReturnError(errors.New("database unavailable"))
	mock.ExpectRollback()

	repository, err := NewUserRepository(engine)
	require.NoError(t, err)
	user, created, err := repository.CreateOrPromoteSystemAdmin(
		context.Background(), "admin", "Administrator", "new-hash",
	)
	require.Error(t, err)
	require.Nil(t, user)
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}
