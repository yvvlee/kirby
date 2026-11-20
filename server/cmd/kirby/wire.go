//go:build wireinject

package kirby

import (
	"context"
	"net/http"

	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"

	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/server"
	"github.com/yvvlee/kirby/server/internal/service"
)

func wireServer(ctx context.Context, configPath string, webHandler http.Handler) (*kratos.App, func(), error) {
	panic(wire.Build(
		conf.ProviderSet,
		repository.ProviderSet,
		logic.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
	))
}
