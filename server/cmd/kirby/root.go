package kirby

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/yvvlee/kirby/server/internal/version"
)

// Execute runs the Kirby command-line interface.
func Execute() error {
	return newRootCommand(os.Stdout, os.Stderr).Execute()
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:           "kirby",
		Short:         "Kirby configuration platform server",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.String(),
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetVersionTemplate(fmt.Sprintf("kirby %s\n", version.String()))
	command.AddCommand(newServeCommand())
	command.AddCommand(newCreateAdminCommand())

	return command
}
