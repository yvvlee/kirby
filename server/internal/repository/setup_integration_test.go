package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

func TestInitialAdministratorLifecycle(t *testing.T) {
	tests := []struct {
		driver string
		dsnEnv string
	}{
		{
			driver: "mysql",
			dsnEnv: "KIRBY_SETUP_TEST_MYSQL_DSN",
		},
		{
			driver: "pgx",
			dsnEnv: "KIRBY_SETUP_TEST_PGSQL_DSN",
		},
	}
	for _, test := range tests {
		t.Run(test.driver, func(t *testing.T) {
			runInitialAdministratorLifecycle(t, test.driver, test.dsnEnv)
		})
	}
}

func runInitialAdministratorLifecycle(t *testing.T, driver, dsnEnv string) {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skip("set " + dsnEnv + " to a disposable empty database")
	}
	ctx := context.Background()
	first := openSetupEngine(t, driver, dsn)
	second := openSetupEngine(t, driver, dsn)
	require.NoError(t, initializeSetupDatabase(ctx, driver, dsn))
	assertSetupState(t, ctx, first, false, 0)
	verifySetupRollback(t, ctx, first)
	verifyConcurrentSetup(t, ctx, first, second)
	require.NoError(t, first.Close())
	restarted := openSetupEngine(t, driver, dsn)
	assertSetupState(t, ctx, restarted, true, 1)
	_, err := restarted.CoreDB.Exec(ctx, "DELETE FROM users")
	require.NoError(t, err)
	assertSetupState(t, ctx, restarted, true, 0)
	err = restarted.CoreDB.TX(ctx, func(txctx context.Context) error {
		return NewSetupRepository(restarted).CreateInitialAdmin(txctx, initialTestUser("replacement"))
	})
	require.ErrorIs(t, err, ErrAlreadyInitialized)
}

func openSetupEngine(t *testing.T, driver, dsn string) *infra.Storage {
	t.Helper()
	engine, err := lorm.NewEngine(driver, dsn, lorm.WithLogger(nil))
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	return &infra.Storage{CoreDB: engine}
}

func assertSetupState(t *testing.T, ctx context.Context, engine *infra.Storage, initialized bool, users int64) {
	t.Helper()
	actual, err := NewSetupRepository(engine).IsInitialized(ctx)
	require.NoError(t, err)
	require.Equal(t, initialized, actual)
	count, err := engine.CoreDB.Query[*model.User]().Count(ctx)
	require.NoError(t, err)
	require.EqualValues(t, users, count)
}

func verifySetupRollback(t *testing.T, ctx context.Context, engine *infra.Storage) {
	t.Helper()
	sentinel := errors.New("operation failed after writing administrator")
	err := engine.CoreDB.TX(ctx, func(txctx context.Context) error {
		if err := NewSetupRepository(engine).CreateInitialAdmin(txctx, initialTestUser("rolled-back")); err != nil {
			return err
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	assertSetupState(t, ctx, engine, false, 0)
	err = engine.CoreDB.TX(ctx, func(txctx context.Context) error {
		if err := NewSetupRepository(engine).MarkInitialized(txctx); err != nil {
			return err
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	assertSetupState(t, ctx, engine, false, 0)
}

func verifyConcurrentSetup(t *testing.T, ctx context.Context, engines ...*infra.Storage) {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, len(engines))
	var group sync.WaitGroup
	for _, engine := range engines {
		group.Go(func() {
			<-start
			results <- engine.CoreDB.TX(ctx, func(txctx context.Context) error {
				return NewSetupRepository(engine).CreateInitialAdmin(txctx, initialTestUser("first-admin"))
			})
		})
	}
	close(start)
	group.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if errors.Is(err, ErrAlreadyInitialized) {
			conflicts++
			continue
		}
		require.NoError(t, err)
		successes++
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	assertSetupState(t, ctx, engines[0], true, 1)
}

func initialTestUser(username string) *model.User {
	return &model.User{
		Username:      username,
		DisplayName:   username,
		PasswordHash:  "integration-test-hash",
		Enabled:       true,
		IsSystemAdmin: true,
	}
}

func initializeSetupDatabase(ctx context.Context, driver, dsn string) error {
	database, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	defer database.Close()
	return model.InitializeDatabase(ctx, database, driver == "pgx")
}
