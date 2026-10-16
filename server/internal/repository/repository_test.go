package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/yvvlee/lorm/builder"
)

func TestClassifyKeyWriteErrorMapsDuplicateKeys(t *testing.T) {
	assert.NoError(t, classifyKeyWriteError("config", nil))
	assert.ErrorIs(t, classifyKeyWriteError("config", &mysql.MySQLError{Number: 1062}), ErrKeyConflict)
	assert.ErrorIs(t, classifyKeyWriteError("config", &pgconn.PgError{Code: "23505"}), ErrKeyConflict)

	other := errors.New("ordinary failure")
	wrapped := classifyKeyWriteError("config", other)
	assert.Error(t, wrapped)
	assert.NotErrorIs(t, wrapped, ErrKeyConflict)
}

func TestDuplicateKeyDetection(t *testing.T) {
	assert.True(t, isDuplicateKeyError(&mysql.MySQLError{
		Number:  1062,
		Message: "duplicate",
	}))
	assert.False(t, isDuplicateKeyError(&mysql.MySQLError{
		Number:  1146,
		Message: "table missing",
	}))
	assert.False(t, isDuplicateKeyError(errors.New("unrelated")))
}

func TestOptionalCondsSkipNilPointers(t *testing.T) {
	value := int64(7)
	query, args, err := (builder.And{eqIf("p.id", &value), eqIf("c.key", (*string)(nil))}).ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "(p.id = ?)", query)
	assert.Equal(t, []any{value}, args)
	query, _, err = (builder.And{neqIf("s.id", &value)}).ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "(s.id <> ?)", query)
}

func TestWrapDatabaseErrorPreservesContextCancellation(t *testing.T) {
	assert.ErrorIs(t, WrapDatabaseError(context.Canceled), context.Canceled)
	assert.ErrorIs(t, WrapDatabaseError(context.DeadlineExceeded), context.DeadlineExceeded)
}
