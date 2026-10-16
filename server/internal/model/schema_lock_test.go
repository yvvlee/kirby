package model

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestInitializeDatabaseKeepsSchemaInspectionInsideAdvisoryLock(t *testing.T) {
	for _, postgres := range []bool{false, true} {
		t.Run(schemaDialectName(postgres), func(t *testing.T) {
			assertExistingSchemaInitialization(t, postgres)
		})
	}
}

func assertExistingSchemaInitialization(t *testing.T, postgres bool) {
	t.Helper()
	database, mock := newSchemaMockDatabase(t)
	database.SetMaxOpenConns(1)
	expectSchemaLock(mock, postgres)
	schemaExpr := "DATABASE()"
	unlock := mysqlSchemaUnlock
	if postgres {
		schemaExpr = "current_schema()"
		unlock = pgSchemaUnlock
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = " + schemaExpr + " AND table_name = 'users'")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	rows := sqlmock.NewRows([]string{"table_name"})
	for _, table := range RequiredTables() {
		rows.AddRow(table)
	}
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = " + schemaExpr + " AND table_type = 'BASE TABLE' AND table_name IN (" + schemaPlaceholders(postgres) + ")"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(schemaQueryArguments()...).WillReturnRows(rows)
	mock.ExpectExec("ROLLBACK").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(unlock)).WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, InitializeDatabase(context.Background(), database, postgres))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInitializeDatabaseRejectsFailedSchemaInspection(t *testing.T) {
	database, mock := newSchemaMockDatabase(t)
	queryErr := errors.New("schema inspection failed")
	expectSchemaLock(mock, false)
	mock.ExpectQuery("SELECT COUNT").WillReturnError(queryErr)
	mock.ExpectExec("ROLLBACK").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(mysqlSchemaUnlock)).WillReturnResult(sqlmock.NewResult(0, 0))

	require.ErrorIs(t, InitializeDatabase(context.Background(), database, false), queryErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectSchemaLock(mock sqlmock.Sqlmock, postgres bool) {
	if postgres {
		mock.ExpectExec(regexp.QuoteMeta(pgSchemaLock)).WillReturnResult(sqlmock.NewResult(0, 0))
		return
	}
	mock.ExpectQuery(regexp.QuoteMeta(mysqlSchemaLock)).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(1))
}

func schemaDialectName(postgres bool) string {
	if postgres {
		return "postgres"
	}
	return "mysql"
}
