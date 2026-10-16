package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestNullableJSONRoundTripAgainstDatabase(t *testing.T) {
	databases := []struct {
		driver string
		dsnEnv string
	}{
		{
			driver: "mysql",
			dsnEnv: "KIRBY_NULLABLE_JSON_MYSQL_DSN",
		},
		{
			driver: "pgx",
			dsnEnv: "KIRBY_NULLABLE_JSON_PGSQL_DSN",
		},
	}
	for _, database := range databases {
		t.Run(database.driver, func(t *testing.T) {
			testNullableJSONDatabase(t, database.driver, database.dsnEnv)
		})
	}
}

func testNullableJSONDatabase(t *testing.T, driver, dsnEnv string) {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skip("set " + dsnEnv + " to a database with Kirby tables; all writes are rolled back")
	}
	engine, err := lorm.NewEngine(driver, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	database := engine
	rollback := errors.New("roll back nullable JSON fixture")
	err = database.TX(context.Background(), func(ctx context.Context) error {
		for _, rawJSON := range []any{`{"target_snapshot_id":42}`, nil} {
			verifyNullableJSONDatabaseRows(t, ctx, database, rawJSON)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

func verifyNullableJSONDatabaseRows(t *testing.T, ctx context.Context, database *lorm.Engine, rawJSON any) {
	t.Helper()
	var value *string
	if rawJSON != nil {
		text := rawJSON.(string)
		value = &text
	}
	digest := sha256.Sum256([]byte("nullable-json-fixture"))
	record := &model.ImportRecord{
		UserID:              1,
		SourceEnvironmentID: 1,
		TargetEnvironmentID: 2,
		SourceSnapshotID:    1,
		TargetProjectID:     1,
		IdempotencyKey:      uuid.NewString(),
		RequestHash:         digest[:],
		Status:              model.ImportStatusSucceeded,
		ResultJSON:          value,
	}
	_, err := database.Insert[*model.ImportRecord]().AddModel(record).Exec(ctx)
	require.NoError(t, err)
	loaded, found, err := database.Query[*model.ImportRecord]().ID(record.ID).Get(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assertRawJSONValue(t, loaded.ResultJSON, rawJSON)

	audit := &model.AuditLog{
		Action:       "nullable-json.test",
		ResourceType: "integration-test",
		ResourceID:   "1",
		Result:       model.AuditResultSucceeded,
		RequestID:    uuid.NewString(),
		DetailsJSON:  value,
	}
	_, err = database.Insert[*model.AuditLog]().AddModel(audit).Exec(ctx)
	require.NoError(t, err)
	loadedAudit, found, err := database.Query[*model.AuditLog]().ID(audit.ID).Get(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assertRawJSONValue(t, loadedAudit.DetailsJSON, rawJSON)
}
