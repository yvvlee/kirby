package conf

import (
	"github.com/google/wire"
)

// ProviderSet provides the configuration source and its cleanup.
var ProviderSet = wire.NewSet(
	NewConfig,
)
