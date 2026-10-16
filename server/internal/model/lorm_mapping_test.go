package model

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLormMetadataMatchesExistingSchemas(t *testing.T) {
	for _, schemaFile := range []string{"sql/schema_mysql.sql", "sql/schema_pgsql.sql"} {
		t.Run(schemaFile, func(t *testing.T) {
			assertSchemaModelColumns(t, schemaFile)
		})
	}
}

func assertSchemaModelColumns(t *testing.T, schemaFile string) {
	t.Helper()
	schema, err := schemaSQL.ReadFile(schemaFile)
	require.NoError(t, err)
	for _, record := range AllModels {
		columns := schemaTableColumns(t, string(schema), record.TableName())
		require.ElementsMatch(t, columns, record.LormModelDescriptor().AllFields(), record.TableName())
	}
}

func schemaTableColumns(t *testing.T, schema, table string) []string {
	t.Helper()
	// Both embedded schemas quote identifiers. Restrict the scan to one CREATE
	// TABLE body so index columns cannot conceal a misspelled model field.
	normalized := strings.ReplaceAll(schema, "`", "\"")
	tablePattern := regexp.MustCompile(`(?s)CREATE TABLE "` + regexp.QuoteMeta(table) + `" \((.*?)\n\)`)
	body := tablePattern.FindStringSubmatch(normalized)
	require.Len(t, body, 2, table)
	columnPattern := regexp.MustCompile(`(?m)^\s+"([a-z_]+)"\s`)
	matches := columnPattern.FindAllStringSubmatch(body[1], -1)
	columns := make([]string, 0, len(matches))
	for _, match := range matches {
		columns = append(columns, match[1])
	}
	return columns
}
