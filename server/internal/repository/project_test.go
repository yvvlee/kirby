package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestProjectListScopesCountAndRowsToEnvironment prevents cross-environment
// project metadata from appearing after an environment switch.
// Parameters: t controls the regression test.
// Returns: none.
// Throws: none; failed SQL expectations fail the test.
func TestProjectListScopesCountAndRowsToEnvironment(t *testing.T) {
	engine, mock := newSmokeEngine(t)
	mock.ExpectQuery("(?s)SELECT COUNT\\(1\\).*WHERE .*`e`.`id` = \\? AND").
		WithArgs(int64(23), "%Target%", "%Target%").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectQuery("(?s)SELECT .*WHERE .*`e`.`id` = \\? AND.*ORDER BY `p`.`id` DESC").
		WithArgs(int64(23), "%Target%", "%Target%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "key", "name"}).AddRow(7, "Target", "Target Project"))

	page, err := NewProjectRepository(engine).List(context.Background(), 23, "Target", PageRequest{})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.EqualValues(t, 7, page.Items[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGlobalProjectListRemainsUnscoped keeps system administration independent
// from the environment-specific project list.
// Parameters: t controls the regression test.
// Returns: none.
// Throws: none; failed SQL expectations fail the test.
func TestGlobalProjectListRemainsUnscoped(t *testing.T) {
	engine, mock := newSmokeEngine(t)
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM projects AS p WHERE").
		WithArgs("%%", "%%").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))

	page, err := NewProjectRepository(engine).List(context.Background(), 0, "", PageRequest{})
	require.NoError(t, err)
	require.Empty(t, page.Items)
	require.NoError(t, mock.ExpectationsWereMet())
}
