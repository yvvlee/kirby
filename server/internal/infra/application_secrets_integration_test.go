package infra

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/auth"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/model"
)

func TestApplicationSecretsPersistAcrossConcurrentStartup(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsnEnv string
	}{
		{
			name:   "mysql",
			driver: "mysql",
			dsnEnv: "KIRBY_TEST_MYSQL_DSN",
		},
		{
			name:   "postgres",
			driver: "pgx",
			dsnEnv: "KIRBY_TEST_PGSQL_DSN",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := os.Getenv(test.dsnEnv)
			if dsn == "" {
				t.Skip("set " + test.dsnEnv + " to a disposable empty database")
			}
			verifyPersistedInstallation(t, test.driver, dsn)
		})
	}
}

func verifyPersistedInstallation(t *testing.T, driver, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	first := openInstallationEngine(t, driver, dsn)
	second := openInstallationEngine(t, driver, dsn)
	initializeConcurrentInstances(t, ctx, driver == "pgx", first, second)
	one := loadInstallationSecrets(t, ctx, driver, dsn)
	two := loadInstallationSecrets(t, ctx, driver, dsn)
	require.Equal(t, one, two)
	require.NotEqual(t, one.JWTSigningKey, one.APIKeyPepper)
	require.NoError(t, first.Close())
	require.NoError(t, second.Close())
	restarted := openInstallationEngine(t, driver, dsn)
	require.NoError(t, model.InitializeDatabase(ctx, restarted, driver == "pgx"))
	loaded := loadInstallationSecrets(t, ctx, driver, dsn)
	require.Equal(t, one, loaded)
	verifyInstallationCredentials(t, one, loaded)
}

func openInstallationEngine(t *testing.T, driver, dsn string) *sql.DB {
	t.Helper()
	engine, err := sql.Open(driver, dsn)
	require.NoError(t, err)
	// The startup lock, DDL, seed transaction, and inspection must all share one
	// connection. A pool of one also catches accidental reads outside that session.
	engine.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = engine.Close() })
	require.NoError(t, engine.Ping())
	return engine
}

func initializeConcurrentInstances(t *testing.T, ctx context.Context, postgres bool, engines ...*sql.DB) {
	t.Helper()
	errors := make(chan error, len(engines))
	var group sync.WaitGroup
	for _, engine := range engines {
		group.Go(func() { errors <- model.InitializeDatabase(ctx, engine, postgres) })
	}
	group.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
}

func loadInstallationSecrets(t *testing.T, ctx context.Context, driver, dsn string) *model.ApplicationSecrets {
	t.Helper()
	engine, err := lorm.NewEngineContext(ctx, driver, dsn, lorm.WithLogger(nil))
	require.NoError(t, err)
	defer engine.Close()
	secrets, err := ProvideApplicationSecrets(ctx, &Storage{
		CoreDB: engine,
	})
	require.NoError(t, err)
	return secrets
}

func verifyInstallationCredentials(t *testing.T, first, second *model.ApplicationSecrets) {
	t.Helper()
	cfg := &conf.Config{
		JWT: conf.JWTConfig{
			Issuer:    "kirby-integration",
			AccessTTL: conf.Duration{Duration: 15 * time.Minute},
		},
	}
	issuer, err := auth.ProvideJWTManager(cfg, first)
	require.NoError(t, err)
	verifier, err := auth.ProvideJWTManager(cfg, second)
	require.NoError(t, err)
	token, _, err := issuer.Issue(1, "shared-session")
	require.NoError(t, err)
	_, err = verifier.Verify(token)
	require.NoError(t, err)
	one, err := auth.ProvideCredentialManager(first)
	require.NoError(t, err)
	two, err := auth.ProvideCredentialManager(second)
	require.NoError(t, err)
	credential, err := one.Generate()
	require.NoError(t, err)
	require.True(t, two.Verify(credential.Full, credential.PublicID, credential.Hash))
}
