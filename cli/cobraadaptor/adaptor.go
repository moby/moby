package cobraadaptor

import (
	"github.com/docker/docker/api/client"
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
	var rootCmd = &cobra.Command{
		Use: "docker",
	}
	rootCmd.SetUsageTemplate(usageTemplate)

	stdin, stdout, stderr := term.StdStreams()
	dockerCli := client.NewDockerCli(stdin, stdout, stderr, clientFlags)

	rootCmd.AddCommand(
		volume.NewVolumeCommand(dockerCli),
	)
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
		cmds = append(cmds, cli.Command{Name: cmd.Use, Description: cmd.Short})
	}
	return cmds
}

func (c CobraAdaptor) run(cmd string, args []string) error {
	c.dockerCli.Initialize()
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

var usageTemplate = `Usage:  {{if .Runnable}}{{if .HasFlags}}{{appendIfNotPresent .UseLine "[OPTIONS]"}}{{else}}{{.UseLine}}{{end}}{{end}}{{if .HasSubCommands}}{{ .CommandPath}} COMMAND {{end}}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}
{{end}}{{if .HasExample}}

Examples:
{{ .Example }}{{end}}{{ if .HasLocalFlags}}

Options:
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{ if .HasAvailableSubCommands}}

Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasSubCommands }}

Run '{{.CommandPath}} COMMAND --help' for more information on a command.{{end}}
`
