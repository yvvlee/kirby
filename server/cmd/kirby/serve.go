package kirby

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/observability"
	serverruntime "github.com/yvvlee/kirby/server/internal/server"
	appwire "github.com/yvvlee/kirby/server/internal/wire"
)

func newServeCommand() *cobra.Command {
	var configPath string

	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the Kirby server",
		RunE: func(command *cobra.Command, _ []string) error {
			return runServer(command.Context(), configPath)
		},
	}
	command.Flags().StringVar(&configPath, "config", "", "path to the YAML configuration file")

	return command
}

func runServer(ctx context.Context, configPath string) (err error) {
	cfg, err := config.Load(configPath, os.LookupEnv)
	if err != nil {
		return err
	}
	logger, err := observability.NewLogger(os.Stderr, cfg.Log)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	dependencies, err := appwire.Initialize(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("initialize server: %w", err)
	}
	defer func() { err = errors.Join(err, dependencies.Close()) }()
	app, err := serverruntime.NewApplication(ctx, dependencies)
	if err != nil {
		return fmt.Errorf("assemble server: %w", err)
	}
	if err := app.Run(); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}
