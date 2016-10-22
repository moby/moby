package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type daemonOptions struct {
	version      bool
	configFile   string
	daemonConfig *daemon.Config
	common       *cliflags.CommonOptions
	flags        *pflag.FlagSet
}

func newDaemonCommand() *cobra.Command {
	opts := daemonOptions{
		daemonConfig: daemon.NewConfig(),
		common:       cliflags.NewCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:           "dockerd [OPTIONS]",
		Short:         "A self-sufficient runtime for containers.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.flags = cmd.Flags()
			return runDaemon(opts)
		},
	}
	cli.SetupRootCommand(cmd)

	flags := cmd.Flags()
	flags.BoolVarP(&opts.version, "version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.configFile, flagDaemonConfigFile, defaultDaemonConfigFile, "Daemon configuration file")
	opts.common.InstallFlags(flags)
	opts.daemonConfig.InstallFlags(flags)
	installServiceFlags(flags)

	return cmd
}

func runDaemon(opts daemonOptions) error {
	if opts.version {
		showVersion()
		return nil
	}

	daemonCli := NewDaemonCli()

	// On Windows, if there's no explicit pidfile set, set to under the daemon root
	if runtime.GOOS == "windows" && opts.daemonConfig.Pidfile == "" {
		opts.daemonConfig.Pidfile = filepath.Join(opts.daemonConfig.Root, "docker.pid")
	}

	// On Windows, this may be launching as a service or with an option to
	// register the service.
	stop, err := initService(daemonCli)
	if err != nil {
		logrus.Fatal(err)
	}

	if stop {
		return nil
	}

	err = daemonCli.start(opts)
	notifyShutdown(err)
	return err
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Docker version %s, build %s, experimental\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}

func main() {
	if reexec.Init() {
		return
	}

	// Set terminal emulation based on platform as required.
	_, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	cmd := newDaemonCommand()
	cmd.SetOutput(stdout)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		os.Exit(1)
	}
}
