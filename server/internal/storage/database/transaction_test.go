package database

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/core"
)

const mockMySQLDSN = "user:password@tcp(localhost:3306)/kirby"

func TestWithTxRollsBackBusinessError(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	businessErr := errors.New("business rule rejected")

	err := WithTx(context.Background(), engine, func(*xorm.Session) error {
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
		_ = WithTx(context.Background(), engine, func(*xorm.Session) error {
			panic("boom")
		})
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxCommitsSuccess(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	require.NoError(t, WithTx(context.Background(), engine, func(*xorm.Session) error {
		return nil
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func newMockEngine(t *testing.T) (*xorm.Engine, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	engine, err := xorm.NewEngineWithDB("mysql", mockMySQLDSN, core.FromDB(db))
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	return engine, mock
}
