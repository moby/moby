package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
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
	"github.com/docker/docker/cli/cobraadaptor"
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
	cobraadaptor.SetupRootCommand(cmd)

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Version, "version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.ConfigDir, "config", cliconfig.ConfigDir(), "Location of client config files")
	opts.Common.InstallFlags(flags)

	cmd.SetOutput(dockerCli.Out())
	cmd.AddCommand(
		newDaemonCommand(),
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
	plugin.NewPluginCommand(cmd, dockerCli)

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
