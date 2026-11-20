package kirby

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/yvvlee/kirby/server/internal/i18n"
	"github.com/yvvlee/kirby/server/internal/version"
)

// Execute runs the Kirby command-line interface.
// Parameters: none; command arguments come from the process.
// Returns: the command failure so main can select a nonzero exit status.
// Throws: argument, configuration, or selected command errors.
func Execute() error {
	return newRootCommand(os.Stdout, os.Stderr).Execute()
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	locale := i18n.FromLANG(os.Getenv("LANG"))
	command := &cobra.Command{
		Use:           "kirby",
		Short:         cliText(locale, "Kirby configuration platform server"),
		Long:          cliText(locale, "Run the Kirby server and its management commands."),
		Example:       rootExamples(locale),
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.String(),
	}
	if locale == i18n.LocaleChinese {
		command.SetUsageTemplate(chineseUsageTemplate)
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetVersionTemplate(fmt.Sprintf("kirby %s\n", version.String()))
	command.AddCommand(newServeCommand())
	command.AddCommand(newCreateAdminCommand())
	command.AddCommand(newHealthcheckCommand())
	command.InitDefaultHelpCmd()
	localizeDefaultHelpCommand(command, locale)
	command.InitDefaultCompletionCmd()
	localizeCompletionCommand(command, locale)
	localizeHelpFlags(command, locale)

	return command
}

func localizeDefaultHelpCommand(root *cobra.Command, locale i18n.Locale) {
	for _, child := range root.Commands() {
		if child.Name() != "help" {
			continue
		}
		child.Short = cliText(locale, "Help about any command")
		child.Long = cliText(locale, "Help provides details for any command. Use kirby help [command] to see its full help.")
		return
	}
}

func localizeCompletionCommand(root *cobra.Command, locale i18n.Locale) {
	for _, command := range root.Commands() {
		if command.Name() != "completion" {
			continue
		}
		command.Short = cliText(locale, "Generate shell completion scripts for Kirby")
		command.Long = cliText(locale, "Generate a shell completion script for Kirby. Select bash, zsh, fish, or powershell and follow your shell's standard completion setup.")
		for _, shellCommand := range command.Commands() {
			localizeCompletionShell(shellCommand, locale)
		}
		return
	}
}

func localizeCompletionShell(command *cobra.Command, locale i18n.Locale) {
	shell := command.Name()
	command.Short = fmt.Sprintf(cliText(locale, "Generate the %s shell completion script for Kirby"), shell)
	command.Long = fmt.Sprintf(cliText(locale, "Generate a completion script for Kirby's %s shell. Install or source it using your shell's standard completion setup."), shell)
	if flag := command.Flags().Lookup("no-descriptions"); flag != nil {
		flag.Usage = cliText(locale, "Omit command and flag descriptions from completion output.")
	}
}

func localizeHelpFlags(command *cobra.Command, locale i18n.Locale) {
	command.InitDefaultHelpFlag()
	command.InitDefaultVersionFlag()
	if help := command.Flags().Lookup("help"); help != nil {
		if locale == i18n.LocaleChinese {
			help.Usage = fmt.Sprintf("查看 %s 命令的帮助", command.CommandPath())
		} else {
			help.Usage = fmt.Sprintf("help for %s", command.CommandPath())
		}
	}
	if version := command.Flags().Lookup("version"); version != nil && locale == i18n.LocaleChinese {
		version.Usage = "显示版本信息"
	}
	for _, child := range command.Commands() {
		localizeHelpFlags(child, locale)
	}
}

func cliText(locale i18n.Locale, message string) string {
	return i18n.Translate(locale, message)
}

func rootExamples(locale i18n.Locale) string {
	return fmt.Sprintf("  kirby serve --config ./config.yaml\n    %s\n\n  kirby healthcheck\n    %s\n\n  kirby create-admin --config ./config.yaml --username admin --email admin@example.com\n    %s",
		cliText(locale, "Use --config to select the YAML configuration file."),
		cliText(locale, "Check both listeners with kirby healthcheck."),
		cliText(locale, "Create an administrator with kirby create-admin."))
}

const chineseUsageTemplate = `用法：{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

别名：
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

示例：
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

可用命令：{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

其他命令：{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

标志：
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局标志：
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

其他帮助主题：{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

使用 "{{.CommandPath}} [command] --help" 查看命令的详细帮助。{{end}}
`
