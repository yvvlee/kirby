package infra

import (
	"context"
	"fmt"

	"github.com/google/wire"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/infra/object"
)

// ProviderSet provides shared infrastructure dependencies.
var ProviderSet = wire.NewSet(
	NewLogger,
	NewDBLogger,
	NewCoreStorage,
	ProvideApplicationSecrets,
	ProvideCoreDB,
	ProvideCache,
	ProvideObjectStorage,
)

// NewCoreStorage makes initialized persistence available to dependent providers.
//
// Parameters: ctx controls startup; cfg selects dependencies; logger records failures.
// Returns: storage, its cleanup function, or an initialization error.
// Throws: none.
func NewCoreStorage(ctx context.Context, cfg *conf.Config, logger *DBLogger) (*Storage, func(), error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate config: %w", err)
	}
	store, err := Open(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {
		if err := store.Close(); err != nil {
			logger.Errorw("msg", "close storage", "error", err)
		}
	}, nil
}

// ProvideCoreDB shares the engine used by repositories and transaction runners.
//
// Parameters: store contains the initialized database engine.
// Returns: the shared lorm engine.
// Throws: none.
func ProvideCoreDB(store *Storage) *lorm.Engine {
	return store.CoreDB
}

// ProvideCache shares the configured cache with application services.
//
// Parameters: store contains the initialized cache.
// Returns: the shared cache client.
// Throws: none.
func ProvideCache(store *Storage) cache.Store {
	return store.Cache
}

// ProvideObjectStorage opens the configured object store for uploaded assets.
//
// Parameters: ctx controls startup; cfg selects the backend; logger records close errors.
// Returns: the object store, cleanup function, or initialization error.
// Throws: none.
func ProvideObjectStorage(ctx context.Context, cfg *conf.Config, logger *Logger) (object.ObjectStorage, func(), error) {
	store, err := object.Open(ctx, cfg.Mode, cfg.ObjectStorage)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {
		if err := store.Close(); err != nil {
			logger.Errorw("msg", "close object storage", "error", err)
		}
	}, nil
}
