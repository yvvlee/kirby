package repository

import (
	"context"
	"errors"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestLastEnabledSystemAdministratorCannotBeRemoved(t *testing.T) {
	admins := []model.User{{
		Meta:          model.Meta{ID: 7},
		Enabled:       true,
		IsSystemAdmin: true,
	}}
	if err := ensureSystemAdminSurvives(admins, 7); !errors.Is(err, ErrLastSystemAdmin) {
		t.Fatalf("last administrator removal error = %v", err)
	}

	admins = append(admins, model.User{
		Meta:          model.Meta{ID: 8},
		Enabled:       true,
		IsSystemAdmin: true,
	})
	if err := ensureSystemAdminSurvives(admins, 7); err != nil {
		t.Fatalf("removing one of two administrators failed: %v", err)
	}
}

func TestDisabledAdministratorsDoNotSatisfyProtection(t *testing.T) {
	admins := []model.User{
		{
			Meta:          model.Meta{ID: 1},
			Enabled:       true,
			IsSystemAdmin: true,
		},
		{
			Meta:          model.Meta{ID: 2},
			Enabled:       false,
			IsSystemAdmin: true,
		},
	}
	if err := ensureSystemAdminSurvives(admins, 1); !errors.Is(err, ErrLastSystemAdmin) {
		t.Fatalf("unusable administrators bypassed protection: %v", err)
	}
}

func TestUnknownLoginIdentifierReturnsCredentialNotFound(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectQuery("SELECT .* FROM `users`").WithArgs("unknown@example.test", "unknown@example.test").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err := NewUserRepository(storage).FindByIdentifier(context.Background(), "unknown@example.test")
	require.ErrorIs(t, err, ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
