package cobraadaptor

import (
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/container"
	"github.com/docker/docker/api/client/image"
	"github.com/docker/docker/api/client/network"
	"github.com/docker/docker/api/client/system"
	"github.com/docker/docker/api/client/volume"
	"github.com/docker/docker/cli"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/pkg/term"
	"github.com/spf13/cobra"
)

// CobraAdaptor is an adaptor for supporting spf13/cobra commands in the
// docker/cli framework
type CobraAdaptor struct {
	rootCmd   *cobra.Command
	dockerCli *client.DockerCli
}

// NewCobraAdaptor returns a new handler
func NewCobraAdaptor(clientFlags *cliflags.ClientFlags) CobraAdaptor {
	stdin, stdout, stderr := term.StdStreams()
	dockerCli := client.NewDockerCli(stdin, stdout, stderr, clientFlags)

	var rootCmd = &cobra.Command{
		Use:           "docker",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetFlagErrorFunc(cli.FlagErrorFunc)
	rootCmd.SetOutput(stdout)
	rootCmd.AddCommand(
		container.NewAttachCommand(dockerCli),
		container.NewCreateCommand(dockerCli),
		container.NewDiffCommand(dockerCli),
		container.NewExportCommand(dockerCli),
		container.NewKillCommand(dockerCli),
		container.NewLogsCommand(dockerCli),
		container.NewPauseCommand(dockerCli),
		container.NewPortCommand(dockerCli),
		container.NewRenameCommand(dockerCli),
		container.NewRestartCommand(dockerCli),
		container.NewRmCommand(dockerCli),
		container.NewRunCommand(dockerCli),
		container.NewStartCommand(dockerCli),
		container.NewStopCommand(dockerCli),
		container.NewTopCommand(dockerCli),
		container.NewUnpauseCommand(dockerCli),
		container.NewWaitCommand(dockerCli),
		image.NewHistoryCommand(dockerCli),
		image.NewImagesCommand(dockerCli),
		image.NewRemoveCommand(dockerCli),
		image.NewSearchCommand(dockerCli),
		image.NewImportCommand(dockerCli),
		image.NewTagCommand(dockerCli),
		network.NewNetworkCommand(dockerCli),
		system.NewVersionCommand(dockerCli),
		volume.NewVolumeCommand(dockerCli),
	)

	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")
	rootCmd.PersistentFlags().MarkShorthandDeprecated("help", "please use --help")

	return CobraAdaptor{
		rootCmd:   rootCmd,
		dockerCli: dockerCli,
	}
}

// Usage returns the list of commands and their short usage string for
// all top level cobra commands.
func (c CobraAdaptor) Usage() []cli.Command {
	cmds := []cli.Command{}
	for _, cmd := range c.rootCmd.Commands() {
		cmds = append(cmds, cli.Command{Name: cmd.Name(), Description: cmd.Short})
	}
	return cmds
}

func (c CobraAdaptor) run(cmd string, args []string) error {
	if err := c.dockerCli.Initialize(); err != nil {
		return err
	}
	// Prepend the command name to support normal cobra command delegation
	c.rootCmd.SetArgs(append([]string{cmd}, args...))
	return c.rootCmd.Execute()
}

// Command returns a cli command handler if one exists
func (c CobraAdaptor) Command(name string) func(...string) error {
	for _, cmd := range c.rootCmd.Commands() {
		if cmd.Name() == name {
			return func(args ...string) error {
				return c.run(name, args)
			}
		}
	}
	return nil
}

var usageTemplate = `Usage:	{{if not .HasSubCommands}}{{if .HasLocalFlags}}{{appendIfNotPresent .UseLine "[OPTIONS]"}}{{else}}{{.UseLine}}{{end}}{{end}}{{if .HasSubCommands}}{{ .CommandPath}} COMMAND{{end}}

{{with or .Long .Short }}{{. | trim}}{{end}}{{if gt .Aliases 0}}

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
