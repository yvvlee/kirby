package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const setupStatusQuery = "SELECT .* FROM `application_secrets` WHERE .*`id`.*LIMIT 1"
const setupLockedQuery = setupStatusQuery + " FOR UPDATE"

func TestSetupStatusFailsWhenPersistentMarkerIsMissingOrUnreadable(t *testing.T) {
	t.Run("missing marker", func(t *testing.T) {
		engine, mock := newSmokeEngine(t)
		mock.ExpectQuery(setupStatusQuery).WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "admin_initialized"}))
		_, err := NewSetupRepository(engine).IsInitialized(context.Background())
		require.ErrorContains(t, err, "initialization state is missing")
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("database read failed", func(t *testing.T) {
		engine, mock := newSmokeEngine(t)
		failure := errors.New("database unavailable")
		mock.ExpectQuery(setupStatusQuery).WithArgs(1).WillReturnError(failure)
		_, err := NewSetupRepository(engine).IsInitialized(context.Background())
		require.ErrorIs(t, err, failure)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSetupInsertFailureRollsBackWithoutCompletingMarker(t *testing.T) {
	engine, mock := newSmokeEngine(t)
	failure := errors.New("user insert failed")
	mock.ExpectBegin()
	mock.ExpectQuery(setupLockedQuery).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "admin_initialized"}).AddRow(1, false))
	mock.ExpectExec("INSERT INTO `users`").WillReturnError(failure)
	mock.ExpectRollback()
	err := engine.CoreDB.TX(context.Background(), func(ctx context.Context) error {
		return NewSetupRepository(engine).CreateInitialAdmin(ctx, initialTestUser("owner"))
	})
	require.ErrorIs(t, err, failure)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupMarkerFailureRollsBackAdministratorInsert(t *testing.T) {
	engine, mock := newSmokeEngine(t)
	failure := errors.New("setup marker write failed")
	mock.ExpectBegin()
	mock.ExpectQuery(setupLockedQuery).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "admin_initialized"}).AddRow(1, false))
	mock.ExpectExec("INSERT INTO `users`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("UPDATE `application_secrets` SET `admin_initialized` = \\? WHERE.*`id`.*").
		WithArgs(true, 1).WillReturnError(failure)
	mock.ExpectRollback()
	err := engine.CoreDB.TX(context.Background(), func(ctx context.Context) error {
		return NewSetupRepository(engine).CreateInitialAdmin(ctx, initialTestUser("owner"))
	})
	require.ErrorIs(t, err, failure)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCompletedSetupRefusesAdministratorInsert(t *testing.T) {
	engine, mock := newSmokeEngine(t)
	mock.ExpectBegin()
	mock.ExpectQuery(setupLockedQuery).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "admin_initialized"}).AddRow(1, true))
	mock.ExpectRollback()
	err := engine.CoreDB.TX(context.Background(), func(ctx context.Context) error {
		return NewSetupRepository(engine).CreateInitialAdmin(ctx, initialTestUser("owner"))
	})
	require.ErrorIs(t, err, ErrAlreadyInitialized)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCLISetupMarkerFailureRollsBackBeforeUserProvisioning(t *testing.T) {
	engine, mock := newSmokeEngine(t)
	failure := errors.New("marker unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery(setupLockedQuery).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "admin_initialized"}).AddRow(1, false))
	mock.ExpectExec("UPDATE `application_secrets` SET `admin_initialized` = \\? WHERE.*`id`.*").
		WithArgs(true, 1).WillReturnError(failure)
	mock.ExpectRollback()
	err := engine.CoreDB.TX(context.Background(), func(ctx context.Context) error {
		return NewSetupRepository(engine).MarkInitialized(ctx)
	})
	require.ErrorIs(t, err, failure)
	require.NoError(t, mock.ExpectationsWereMet())
}
