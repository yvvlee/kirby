//go:build wireinject

package wire

import (
	"context"
	"log/slog"

	"github.com/google/wire"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/provider"
)

func Initialize(context.Context, *config.Config, *slog.Logger) (*provider.Application, error) {
	wire.Build(provider.NewApplication)
	return nil, nil
}
