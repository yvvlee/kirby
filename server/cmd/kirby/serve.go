package kirby

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yvvlee/kirby/server/internal/i18n"
	serverruntime "github.com/yvvlee/kirby/server/internal/server"
)

func newServeCommand() *cobra.Command {
	locale := i18n.FromLANG(os.Getenv("LANG"))
	var configPath string
	var webRoot string

	command := &cobra.Command{
		Use:     "serve",
		Short:   cliText(locale, "Start the Kirby server"),
		Long:    cliText(locale, "Start Kirby and serve the browser application."),
		Example: "  kirby serve --config ./config.yaml --web-root ./web/dist",
		RunE: func(command *cobra.Command, _ []string) error {
			return runServer(command.Context(), configPath, webRoot)
		},
	}
	command.Flags().StringVar(&configPath, "config", "", cliText(locale, "path to the YAML configuration file"))
	command.Flags().StringVar(&webRoot, "web-root", "", cliText(locale, "path to the built web application"))

	return command
}

func runServer(ctx context.Context, configPath, webRoot string) error {
	webHandler, err := serverruntime.NewWebHandler(webRoot)
	if err != nil {
		return fmt.Errorf("load web application: %w", err)
	}
	app, cleanup, err := wireServer(ctx, configPath, webHandler)
	if err != nil {
		return fmt.Errorf("initialize server: %w", err)
	}
	defer cleanup()
	if err := app.Run(); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}
