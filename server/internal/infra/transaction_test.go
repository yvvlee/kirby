package infra

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"
)

func TestWithTxRollsBackBusinessError(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	businessErr := errors.New("business rule rejected")

	err := engine.TX(context.Background(), func(context.Context) error {
		return businessErr
	})

	assert.ErrorIs(t, err, businessErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxRollsBackPanic(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	assert.PanicsWithValue(t, "boom", func() {
		_ = engine.TX(context.Background(), func(context.Context) error {
			panic("boom")
		})
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxCommitsSuccess(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	require.NoError(t, engine.TX(context.Background(), func(context.Context) error {
		return nil
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func newMockEngine(t *testing.T) (*lorm.Engine, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.NewWithDSN(t.Name())
	require.NoError(t, err)
	engine, err := lorm.NewEngine("sqlmock", t.Name(),
		lorm.WithLogger(nil),
		lorm.WithDialectConfig(lorm.DefaultDialectConfig("mysql")),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = engine.Close()
		_ = db.Close()
	})
	return engine, mock
}

func TestWithTxPropagatesSessionToNestedWrites(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET enabled = ? WHERE id = ?")).
		WithArgs(false, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := engine.TX(context.Background(), func(ctx context.Context) error {
		return engine.TX(ctx, func(txctx context.Context) error {
			_, err := engine.Exec(txctx, "UPDATE users SET enabled = ? WHERE id = ?", false, int64(7))
			return err
		})
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxDoesNotRunCallbackWhenBeginFails(t *testing.T) {
	engine, mock := newMockEngine(t)
	beginErr := errors.New("connection unavailable")
	mock.ExpectBegin().WillReturnError(beginErr)
	called := false

	err := engine.TX(context.Background(), func(context.Context) error {
		called = true
		return nil
	})

	require.ErrorIs(t, err, beginErr)
	require.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxReturnsBusinessAndRollbackErrors(t *testing.T) {
	engine, mock := newMockEngine(t)
	businessErr := errors.New("business rule rejected")
	rollbackErr := errors.New("rollback connection lost")
	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(rollbackErr)

	err := engine.TX(context.Background(), func(context.Context) error {
		return businessErr
	})

	require.ErrorIs(t, err, businessErr)
	require.ErrorIs(t, err, rollbackErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
