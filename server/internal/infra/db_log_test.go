package infra

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/conf"
)

func TestDatabaseLogsRetainFailuresWithoutCredentialArguments(t *testing.T) {
	database, mock, err := sqlmock.NewWithDSN(t.Name())
	require.NoError(t, err)
	defer database.Close()
	var output bytes.Buffer
	logger, err := NewLoggerWithWriter(&output, conf.LogConfig{Level: "DEBUG"})
	require.NoError(t, err)
	defer logger.Close()
	cfg := conf.DatabaseConfig{
		Driver: conf.DatabaseDriverMySQL,
		DSN:    conf.NewSecret(t.Name()),
	}
	engine, err := connectEngine(context.Background(), cfg, "sqlmock", NewDBLogger(logger))
	require.NoError(t, err)
	defer engine.Close()
	failure := errors.New("connection unavailable")
	query := "UPDATE users SET password_hash = ? WHERE id = ?"
	credential := "credential-value-must-not-be-logged"
	mock.ExpectExec(regexp.QuoteMeta(query)).WithArgs(credential, 1).WillReturnError(failure)

	_, err = engine.Exec(context.Background(), query, credential, 1)

	require.ErrorIs(t, err, failure)
	require.Contains(t, output.String(), failure.Error())
	require.NotContains(t, output.String(), credential)
	require.NoError(t, mock.ExpectationsWereMet())
}
