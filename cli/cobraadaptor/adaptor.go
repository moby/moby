package cobraadaptor

import (
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/container"
	"github.com/docker/docker/api/client/image"
	"github.com/docker/docker/api/client/network"
	"github.com/docker/docker/api/client/node"
	"github.com/docker/docker/api/client/plugin"
	"github.com/docker/docker/api/client/registry"
	"github.com/docker/docker/api/client/service"
	"github.com/docker/docker/api/client/stack"
	"github.com/docker/docker/api/client/swarm"
	"github.com/docker/docker/api/client/system"
	"github.com/docker/docker/api/client/volume"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

// SetupRootCommand sets default usage, help, and error handling for the
// root command.
// TODO: move to cmd/docker/docker?
// TODO: split into common setup and client setup
func SetupRootCommand(rootCmd *cobra.Command, dockerCli *client.DockerCli) {
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetFlagErrorFunc(cli.FlagErrorFunc)
	rootCmd.SetOutput(dockerCli.Out())
	rootCmd.AddCommand(
		node.NewNodeCommand(dockerCli),
		service.NewServiceCommand(dockerCli),
		stack.NewStackCommand(dockerCli),
		stack.NewTopLevelDeployCommand(dockerCli),
		swarm.NewSwarmCommand(dockerCli),
		container.NewAttachCommand(dockerCli),
		container.NewCommitCommand(dockerCli),
		container.NewCopyCommand(dockerCli),
		container.NewCreateCommand(dockerCli),
		container.NewDiffCommand(dockerCli),
		container.NewExecCommand(dockerCli),
		container.NewExportCommand(dockerCli),
		container.NewKillCommand(dockerCli),
		container.NewLogsCommand(dockerCli),
		container.NewPauseCommand(dockerCli),
		container.NewPortCommand(dockerCli),
		container.NewPsCommand(dockerCli),
		container.NewRenameCommand(dockerCli),
		container.NewRestartCommand(dockerCli),
		container.NewRmCommand(dockerCli),
		container.NewRunCommand(dockerCli),
		container.NewStartCommand(dockerCli),
		container.NewStatsCommand(dockerCli),
		container.NewStopCommand(dockerCli),
		container.NewTopCommand(dockerCli),
		container.NewUnpauseCommand(dockerCli),
		container.NewUpdateCommand(dockerCli),
		container.NewWaitCommand(dockerCli),
		image.NewBuildCommand(dockerCli),
		image.NewHistoryCommand(dockerCli),
		image.NewImagesCommand(dockerCli),
		image.NewLoadCommand(dockerCli),
		image.NewRemoveCommand(dockerCli),
		image.NewSaveCommand(dockerCli),
		image.NewPullCommand(dockerCli),
		image.NewPushCommand(dockerCli),
		image.NewSearchCommand(dockerCli),
		image.NewImportCommand(dockerCli),
		image.NewTagCommand(dockerCli),
		network.NewNetworkCommand(dockerCli),
		system.NewEventsCommand(dockerCli),
		system.NewInspectCommand(dockerCli),
		registry.NewLoginCommand(dockerCli),
		registry.NewLogoutCommand(dockerCli),
		system.NewVersionCommand(dockerCli),
		volume.NewVolumeCommand(dockerCli),
		system.NewInfoCommand(dockerCli),
	)
	plugin.NewPluginCommand(rootCmd, dockerCli)

	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")
	rootCmd.PersistentFlags().MarkShorthandDeprecated("help", "please use --help")
}

// GetRootCommand returns the root command. Required to generate the man pages
// and reference docs from a script outside this package.
func (c CobraAdaptor) GetRootCommand() *cobra.Command {
	return c.rootCmd
}

var usageTemplate = `Usage:	{{if not .HasSubCommands}}{{.UseLine}}{{end}}{{if .HasSubCommands}}{{ .CommandPath}} COMMAND{{end}}

{{ .Short | trim }}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{ .Example }}{{end}}{{if .HasFlags}}

Options:
{{.Flags.FlagUsages | trimRightSpace}}{{end}}{{ if .HasAvailableSubCommands}}

Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasSubCommands }}

Run '{{.CommandPath}} COMMAND --help' for more information on a command.{{end}}
`

var helpTemplate = `
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
