package kirby

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/spf13/cobra"

	"github.com/yvvlee/kirby/server/internal/health"
	"github.com/yvvlee/kirby/server/internal/version"
)

const defaultHTTPAddress = ":8000"

func newServeCommand() *cobra.Command {
	var httpAddress string

	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the Kirby server",
		RunE: func(command *cobra.Command, _ []string) error {
			return runServer(command.Context(), httpAddress)
		},
	}
	command.Flags().StringVar(&httpAddress, "http-address", defaultHTTPAddress, "HTTP listen address")

	return command
}

func runServer(ctx context.Context, httpAddress string) error {
	probe := health.NewProbe()
	server := kratoshttp.NewServer(
		kratoshttp.Address(httpAddress),
		kratoshttp.Timeout(10*time.Second),
	)
	health.Register(server, probe)

	app := kratos.New(
		kratos.Name("kirby"),
		kratos.Version(version.Version),
		kratos.Context(ctx),
		kratos.Server(server),
		kratos.BeforeStart(func(context.Context) error {
			probe.SetReady(true)
			return nil
		}),
		kratos.AfterStop(func(context.Context) error {
			probe.SetReady(false)
			return nil
		}),
	)
	if err := app.Run(); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}
