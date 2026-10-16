package permission

import (
	"github.com/google/wire"
)

// ProviderSet is permission providers.
var ProviderSet = wire.NewSet(
	NewResolver,
)
