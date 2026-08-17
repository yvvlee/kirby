package model

import (
	"context"
	"database/sql/driver"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/core"
)

type tableNamer interface {
	TableName() string
}

func TestAllModelsContainsFixedTableInventory(t *testing.T) {
	t.Parallel()

	want := []string{
		"audit_logs",
		"config_enums",
		"configs",
		"environments",
		"import_records",
		"permissions",
		"project_api_keys",
		"projects",
		"refresh_tokens",
		"role_permissions",
		"roles",
		"snapshots",
		"structures",
		"user_environment_roles",
		"users",
	}

	got := make([]string, 0, len(AllModels))
	seen := make(map[string]struct{}, len(AllModels))
	for _, record := range AllModels {
		namer, ok := record.(tableNamer)
		if !ok {
			t.Fatalf("model %T does not implement TableName", record)
		}
		name := namer.TableName()
		if _, duplicate := seen[name]; duplicate {
			t.Fatalf("duplicate model table %q", name)
		}
		seen[name] = struct{}{}
		got = append(got, name)
	}

	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("table inventory mismatch\nwant: %v\n got: %v", want, got)
	}
}

func TestCredentialModelsOnlyExposeHashes(t *testing.T) {
	t.Parallel()

	assertBinaryHashField(t, reflect.TypeOf(RefreshToken{}), "TokenHash")
	assertBinaryHashField(t, reflect.TypeOf(ProjectAPIKey{}), "SecretHash")
	assertBinaryHashField(t, reflect.TypeOf(ImportRecord{}), "RequestHash")
	secretSuffix, ok := reflect.TypeOf(ProjectAPIKey{}).FieldByName("SecretSuffix")
	if !ok || secretSuffix.Type.Kind() != reflect.String || !strings.Contains(secretSuffix.Tag.Get("xorm"), "CHAR(4)") {
		t.Fatal("ProjectAPIKey.SecretSuffix must store only the four-character display suffix")
	}

	for _, record := range []any{RefreshToken{}, ProjectAPIKey{}} {
		recordType := reflect.TypeOf(record)
		for index := 0; index < recordType.NumField(); index++ {
			name := recordType.Field(index).Name
			if name == "Token" || name == "Secret" || name == "APIKey" {
				t.Fatalf("%s must not contain plaintext credential field %s", recordType.Name(), name)
			}
		}
	}
}

func assertBinaryHashField(t *testing.T, recordType reflect.Type, fieldName string) {
	t.Helper()

	field, ok := recordType.FieldByName(fieldName)
	if !ok {
		t.Fatalf("%s.%s is missing", recordType.Name(), fieldName)
	}
	if field.Type != reflect.TypeOf([]byte(nil)) {
		t.Fatalf("%s.%s must be []byte, got %s", recordType.Name(), fieldName, field.Type)
	}
	if !strings.Contains(field.Tag.Get("xorm"), "BINARY(32)") {
		t.Fatalf("%s.%s must use a 32-byte database hash", recordType.Name(), fieldName)
	}
}

func TestFixedStatusValues(t *testing.T) {
	t.Parallel()

	if SnapshotStatusUnreleased != 1 || SnapshotStatusReleased != 3 {
		t.Fatalf("snapshot status values changed: unreleased=%d released=%d", SnapshotStatusUnreleased, SnapshotStatusReleased)
	}

	wantImports := []ImportStatus{ImportStatusPending, ImportStatusSucceeded, ImportStatusFailed}
	gotImports := []ImportStatus{"pending", "succeeded", "failed"}
	if !reflect.DeepEqual(gotImports, wantImports) {
		t.Fatalf("import status values changed: %v", wantImports)
	}
}

func TestSchemaKeepsCriticalDatabaseGuarantees(t *testing.T) {
	t.Parallel()

	schemaBytes, err := os.ReadFile("../../../deploy/schema.sql")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema := string(schemaBytes)

	required := []string{
		"UNIQUE KEY `ux_projects_environment_key` (`environment_id`, `key`)",
		"UNIQUE KEY `ux_import_records_idempotency` (`user_id`, `target_environment_id`, `idempotency_key`)",
		"`runtime_version` BIGINT NOT NULL DEFAULT 0",
		"`is_system_admin` BOOLEAN NOT NULL DEFAULT FALSE",
		"`token_hash` BINARY(32) NOT NULL",
		"`secret_hash` BINARY(32) NOT NULL",
		"`secret_suffix` CHAR(4) CHARACTER SET ascii COLLATE ascii_bin NOT NULL",
		"CONSTRAINT `ck_snapshots_status` CHECK (`status` IN (1, 3))",
		"System permissions (18-20) are deliberately not assigned to an environment",
	}
	for _, fragment := range required {
		if !strings.Contains(schema, fragment) {
			t.Errorf("schema is missing %q", fragment)
		}
	}

	if strings.Contains(strings.ToUpper(schema), "CREATE TABLE IF NOT EXISTS") {
		t.Error("schema must reject accidental repeated execution")
	}
}

func TestValidateSchemaFailsWithMissingTables(t *testing.T) {
	t.Parallel()

	engine, mock := newSchemaMockEngine(t)
	query := schemaInspectionQueryPattern()
	mock.ExpectQuery(query).
		WithArgs(schemaQueryArguments()...).
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("users"))

	err := ValidateSchema(context.Background(), engine)
	require.ErrorContains(t, err, "database schema is incomplete")
	require.ErrorContains(t, err, "environments")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSchemaAcceptsCompleteSchema(t *testing.T) {
	t.Parallel()

	engine, mock := newSchemaMockEngine(t)
	rows := sqlmock.NewRows([]string{"table_name"})
	for _, table := range RequiredTables() {
		rows.AddRow(table)
	}
	mock.ExpectQuery(schemaInspectionQueryPattern()).
		WithArgs(schemaQueryArguments()...).
		WillReturnRows(rows)

	require.NoError(t, ValidateSchema(context.Background(), engine))
	require.NoError(t, mock.ExpectationsWereMet())
}

func newSchemaMockEngine(t *testing.T) (*xorm.Engine, sqlmock.Sqlmock) {
	t.Helper()

	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	engine, err := xorm.NewEngineWithDB("mysql", "user:password@/kirby", core.FromDB(database))
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	return engine, mock
}

func schemaInspectionQueryPattern() string {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(RequiredTables())), ",")
	query := "SELECT table_name FROM information_schema.tables " +
		"WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' AND table_name IN (" + placeholders + ")"
	return regexp.QuoteMeta(query)
}

func schemaQueryArguments() []driver.Value {
	tables := RequiredTables()
	arguments := make([]driver.Value, len(tables))
	for index, table := range tables {
		arguments[index] = table
	}
	return arguments
}
