package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/command"
	"github.com/docker/docker/cli"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newDockerCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := cliflags.NewClientOptions()
	cmd := &cobra.Command{
		Use:              "docker [OPTIONS] COMMAND [arg...]",
		Short:            "A self-sufficient runtime for containers.",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		Args:             cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Version {
				showVersion()
				return nil
			}
			fmt.Fprintf(dockerCli.Err(), "\n"+cmd.UsageString())
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			dockerPreRun(cmd.Flags(), opts)
			return dockerCli.Initialize(opts)
		},
	}
	cli.SetupRootCommand(cmd)

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Version, "version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.ConfigDir, "config", cliconfig.ConfigDir(), "Location of client config files")
	opts.Common.InstallFlags(flags)

	cmd.SetOutput(dockerCli.Out())
	cmd.AddCommand(newDaemonCommand())
	command.AddCommands(cmd, dockerCli)

	return cmd
}

func main() {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	dockerCli := client.NewDockerCli(stdin, stdout, stderr)
	cmd := newDockerCommand(dockerCli)

	if err := cmd.Execute(); err != nil {
		if sterr, ok := err.(cli.StatusError); ok {
			if sterr.Status != "" {
				fmt.Fprintln(stderr, sterr.Status)
			}
			// StatusError should only be used for errors, and all errors should
			// have a non-zero exit status, so never exit with 0
			if sterr.StatusCode == 0 {
				os.Exit(1)
			}
			os.Exit(sterr.StatusCode)
		}
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Docker version %s, build %s, experimental\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}

func dockerPreRun(flags *pflag.FlagSet, opts *cliflags.ClientOptions) {
	opts.Common.SetDefaultOptions(flags)
	cliflags.SetDaemonLogLevel(opts.Common.LogLevel)

	if opts.ConfigDir != "" {
		cliconfig.SetConfigDir(opts.ConfigDir)
	}

	if opts.Common.Debug {
		utils.EnableDebug()
	}
}
