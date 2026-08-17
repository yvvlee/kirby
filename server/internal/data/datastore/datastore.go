// Package datastore assembles the shared persistence dependencies.
package datastore

import (
	"context"
	"errors"
	"fmt"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

// Store contains long-lived persistence clients shared by application services.
type Store struct {
	Database *xorm.Engine
	Cache    cache.Store
}

// Open connects to all configured dependencies without changing database schema.
func Open(ctx context.Context, cfg *config.Config) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate datastore config: %w", err)
	}
	engine, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		return nil, err
	}
	cacheStore, err := cache.Open(ctx, cfg.Cache)
	if err != nil {
		_ = engine.Close()
		return nil, err
	}
	return &Store{Database: engine, Cache: cacheStore}, nil
}

// Close releases cache and database connections.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var errs []error
	if s.Cache != nil {
		if err := s.Cache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close cache: %w", err))
		}
	}
	if s.Database != nil {
		if err := s.Database.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close database: %w", err))
		}
	}
	return errors.Join(errs...)
}
