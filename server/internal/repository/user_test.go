package repository

import (
	"context"
	"errors"
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

func TestLastAdministratorGuardRollsBackDatabaseMutation(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .* FROM `+"`users`"+` WHERE .*id.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "display_name", "password_hash", "enabled", "is_system_admin", "version", "created_at", "updated_at", "deleted_at",
		}).AddRow(7, "admin", "Admin", "hash", true, true, 3, time.Now(), time.Now(), nil))
	mock.ExpectQuery(`(?s)SELECT .* FROM `+"`users`"+` WHERE .*enabled.*is_system_admin.*FOR UPDATE`).
		WithArgs(true, true, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "display_name", "password_hash", "enabled", "is_system_admin", "version", "created_at", "updated_at", "deleted_at",
		}).AddRow(7, "admin", "Admin", "hash", true, true, 3, time.Now(), time.Now(), nil))
	mock.ExpectRollback()

	repository, err := NewUserRepository(engine)
	require.NoError(t, err)
	_, err = repository.UpdateSecurityState(context.Background(), 7, false, true, 3, 7)
	require.ErrorIs(t, err, ErrLastSystemAdmin)
	require.NoError(t, mock.ExpectationsWereMet())
}
