package kirby

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yvvlee/kirby/server/internal/conf"
	serverruntime "github.com/yvvlee/kirby/server/internal/server"
)

func newServeCommand() *cobra.Command {
	var configPath string
	var webRoot string

	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the Kirby server",
		RunE: func(command *cobra.Command, _ []string) error {
			return runServer(command.Context(), configPath, webRoot)
		},
	}
	command.Flags().StringVar(&configPath, "config", "", "path to the YAML configuration file")
	command.Flags().StringVar(&webRoot, "web-root", "", "path to the built web application")

	return command
}

func runServer(ctx context.Context, configPath, webRoot string) error {
	cfg, err := conf.Load(configPath, os.LookupEnv)
	if err != nil {
		return err
	}
	webHandler, err := serverruntime.NewWebHandler(webRoot)
	if err != nil {
		return fmt.Errorf("load web application: %w", err)
	}
	app, cleanup, err := wireServer(ctx, cfg, webHandler)
	if err != nil {
		return fmt.Errorf("initialize server: %w", err)
	}
	defer cleanup()
	if err := app.Run(); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}
