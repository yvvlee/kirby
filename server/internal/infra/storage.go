// Package infra provides shared database, cache, object storage, and logging dependencies.
package infra

import (
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra/cache"
)

// Storage contains long-lived persistence clients shared by application services.
// It aggregates references only: each client is created and released by its own provider.
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
