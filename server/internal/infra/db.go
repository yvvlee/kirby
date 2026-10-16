package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/model"
)

// OpenEngine initializes the schema before making the shared lorm engine available.
//
// Parameters: ctx controls startup; cfg selects the database; logger records database failures.
// Returns: the connected engine, or a connection/schema initialization error.
// Throws: none.
func OpenEngine(ctx context.Context, cfg conf.DatabaseConfig, logger *DBLogger) (*lorm.Engine, error) {
	driver, err := databaseDriver(cfg.Driver)
	if err != nil {
		return nil, err
	}
	if err := initializeDatabase(ctx, driver, cfg.DSN.Value()); err != nil {
		return nil, err
	}
	return connectEngine(ctx, cfg, driver, logger)
}

func databaseDriver(driver conf.DatabaseDriver) (string, error) {
	switch driver {
	case conf.DatabaseDriverMySQL:
		return "mysql", nil
	case conf.DatabaseDriverPGSQL:
		return "pgx", nil
	default:
		return "", fmt.Errorf("unsupported database driver %q", driver)
	}
}

func initializeDatabase(ctx context.Context, driver, dsn string) (err error) {
	// Schema locks belong to one physical connection. Keep this short-lived pool
	// separate because lorm intentionally does not expose its application pool.
	database, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("open schema initialization database: %w", err)
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	database.SetMaxOpenConns(1)
	if err := model.InitializeDatabase(ctx, database, driver == "pgx"); err != nil {
		return fmt.Errorf("initialize database schema: %w", err)
	}
	return nil
}

func connectEngine(ctx context.Context, cfg conf.DatabaseConfig, driver string, logger lorm.Logger) (*lorm.Engine, error) {
	engine, err := lorm.NewEngineContext(ctx, driver, cfg.DSN.Value(),
		lorm.WithLogger(logger),
		// Installation secrets and credential hashes are SQL parameters.
		lorm.WithLogSQLArgs(false),
		lorm.WithMaxOpenConns(cfg.MaxOpenConns),
		lorm.WithMaxIdleConns(cfg.MaxIdleConns),
		lorm.WithConnMaxLifetime(cfg.ConnMaxLifetime.Duration),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", cfg.Driver, err)
	}
	return engine, nil
}
