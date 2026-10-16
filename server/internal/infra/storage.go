// Package infra provides shared database, cache, object storage, and logging dependencies.
package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
)

// Storage contains long-lived persistence clients shared by application services.
type Storage struct {
	CoreDB *lorm.Engine
	Cache  cache.Store
}

// NewStorage constructs the shared persistence container.
//
// Parameters: engine owns the database pool; cacheStore owns the configured cache.
// Returns: the shared storage container.
// Throws: none.
func NewStorage(engine *lorm.Engine, cacheStore cache.Store) *Storage {
	return &Storage{
		CoreDB: engine,
		Cache:  cacheStore,
	}
}

// Open initializes the database and connects all configured persistence clients.
//
// Parameters: ctx controls startup; cfg contains validated settings; logger records database events.
// Returns: shared persistence clients, or the first initialization error.
// Throws: none.
func Open(ctx context.Context, cfg *conf.Config, logger *DBLogger) (*Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate datastore config: %w", err)
	}
	databaseConfig := cfg.Database
	if databaseConfig.Driver == "" {
		databaseConfig = cfg.MySQL
		if databaseConfig.Driver == "" {
			databaseConfig.Driver = conf.DatabaseDriverMySQL
		}
	}
	engine, err := OpenEngine(ctx, databaseConfig, logger)
	if err != nil {
		return nil, err
	}
	cacheStore, err := cache.Open(ctx, cfg.Cache)
	if err != nil {
		_ = engine.Close()
		return nil, err
	}
	return NewStorage(engine, cacheStore), nil
}

// Close releases cache and database connections.
//
// Parameters: none.
// Returns: joined close errors from the owned clients.
// Throws: none.
func (s *Storage) Close() error {
	var errs []error
	if s.Cache != nil {
		if err := s.Cache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close cache: %w", err))
		}
	}
	if s.CoreDB != nil {
		if err := s.CoreDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close database: %w", err))
		}
	}
	return errors.Join(errs...)
}
