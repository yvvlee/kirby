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

// ProviderSet provides shared infrastructure dependencies. Each provider owns
// and releases exactly the resources it creates.
var ProviderSet = wire.NewSet(
	NewLogger,
	NewDBLogger,
	NewCoreEngine,
	NewCache,
	NewStorage,
	ProvideApplicationSecrets,
	ProvideObjectStorage,
)

// NewCoreEngine 初始化共享数据库引擎，并返回只负责自身的 cleanup 函数。
//
// Parameters: ctx controls startup; cfg contains validated settings; logger records database events.
// Returns: the shared lorm engine, its cleanup function, or an initialization error.
// Throws: none.
func NewCoreEngine(ctx context.Context, cfg *conf.Config, logger *DBLogger) (*lorm.Engine, func(), error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate config: %w", err)
	}
	engine, err := OpenEngine(ctx, cfg.Database, logger)
	if err != nil {
		return nil, nil, err
	}
	return engine, func() {
		if err := engine.Close(); err != nil {
			logger.Errorw("msg", "close database engine", "error", err)
		}
	}, nil
}

// NewCache 初始化配置指定的缓存，并返回只负责自身的 cleanup 函数。
//
// Parameters: ctx controls startup; cfg selects the cache backend; logger records close failures.
// Returns: the cache store, its cleanup function, or an initialization error.
// Throws: none.
func NewCache(ctx context.Context, cfg *conf.Config, logger *Logger) (cache.Store, func(), error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}
	store, err := cache.Open(ctx, cfg.Cache)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {
		if err := store.Close(); err != nil {
			logger.Errorw("msg", "close cache", "error", err)
		}
	}, nil
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
