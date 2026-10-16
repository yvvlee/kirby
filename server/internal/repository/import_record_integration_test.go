package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

func TestImportClaimReplayKeepsDatabaseTransactionUsable(t *testing.T) {
	databases := []struct {
		driver string
		dsnEnv string
	}{
		{
			driver: "mysql",
			dsnEnv: "KIRBY_IMPORT_REPLAY_MYSQL_DSN",
		},
		{
			driver: "pgx",
			dsnEnv: "KIRBY_IMPORT_REPLAY_PGSQL_DSN",
		},
	}
	for _, database := range databases {
		t.Run(database.driver, func(t *testing.T) {
			testDatabaseImportClaimReplay(t, database.driver, database.dsnEnv)
		})
	}
}

func testDatabaseImportClaimReplay(t *testing.T, driver, dsnEnv string) {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skip("set " + dsnEnv + " to a database with Kirby tables; all writes are rolled back")
	}
	engine, err := lorm.NewEngine(driver, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	database := engine
	repo := NewImportRecordRepository(&infra.Storage{CoreDB: database})
	rollback := errors.New("roll back import replay fixture")
	err = database.TX(context.Background(), func(ctx context.Context) error {
		verifyImportClaimReplay(t, ctx, repo, database)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

func verifyImportClaimReplay(t *testing.T, ctx context.Context, repo *ImportRecordRepositoryImpl, database *lorm.Engine) {
	t.Helper()
	request := claimRecordForSmoke()
	request.IdempotencyKey = uuid.NewString()
	claimed, created, err := repo.Claim(ctx, request)
	require.NoError(t, err)
	require.True(t, created)
	require.Positive(t, claimed.ID)
	require.Equal(t, int64(1), claimed.Version)
	require.NoError(t, repo.Complete(ctx, claimed.ID, 42, `{"target_snapshot_id":42}`))

	retry := claimRecordForSmoke()
	retry.IdempotencyKey = request.IdempotencyKey
	retry.ErrorMessage = "different transaction claim"
	replayed, created, err := repo.Claim(ctx, retry)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, claimed.ID, replayed.ID)
	require.Equal(t, model.ImportStatusSucceeded, replayed.Status)
	require.NotNil(t, replayed.TargetSnapshotID)
	require.Equal(t, int64(42), *replayed.TargetSnapshotID)
	assertRawJSONValue(t, replayed.ResultJSON, `{"target_snapshot_id":42}`)
	count, err := database.Query[*model.ImportRecord]().
		Where("idempotency_key = ?", request.IdempotencyKey).Count(ctx)
	require.NoError(t, err, "the replay must not abort its enclosing transaction")
	require.EqualValues(t, 1, count)
}
