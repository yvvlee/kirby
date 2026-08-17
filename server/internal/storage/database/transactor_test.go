package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

func TestEngineTransactorUsesSharedTransactionBoundary(t *testing.T) {
	engine, mock := newMockEngine(t)
	mock.ExpectBegin()
	mock.ExpectCommit()
	transactor, err := NewTransactor(engine)
	require.NoError(t, err)
	require.NoError(t, transactor.WithTx(context.Background(), func(*xorm.Session) error { return nil }))
	require.NoError(t, mock.ExpectationsWereMet())
}
