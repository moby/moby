package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/commands"
	cliconfig "github.com/docker/docker/cli/config"
	"github.com/docker/docker/cli/debug"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/term"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"strings"
)

func newDockerCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := cliflags.NewClientOptions()
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:              "docker [OPTIONS] COMMAND [ARG...]",
		Short:            "A self-sufficient runtime for containers",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		Args:             noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Version {
				showVersion()
				return nil
			}
			return dockerCli.ShowHelp(cmd, args)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// daemon command is special, we redirect directly to another binary
			if cmd.Name() == "daemon" {
				return nil
			}
			// flags must be the top-level command flags, not cmd.Flags()
			opts.Common.SetDefaultOptions(flags)
			dockerPreRun(opts)
			if err := dockerCli.Initialize(opts); err != nil {
				return err
			}
			return isSupported(cmd, dockerCli.Client().ClientVersion(), dockerCli.HasExperimental())
		},
	}
	cli.SetupRootCommand(cmd)

	cmd.SetHelpFunc(func(ccmd *cobra.Command, args []string) {
		if dockerCli.Client() == nil { // when using --help, PersistenPreRun is not called, so initialization is needed.
			// flags must be the top-level command flags, not cmd.Flags()
			opts.Common.SetDefaultOptions(flags)
			dockerPreRun(opts)
			dockerCli.Initialize(opts)
		}

		if err := isSupported(ccmd, dockerCli.Client().ClientVersion(), dockerCli.HasExperimental()); err != nil {
			ccmd.Println(err)
			return
		}

		hideUnsupportedFeatures(ccmd, dockerCli.Client().ClientVersion(), dockerCli.HasExperimental())

		if err := ccmd.Help(); err != nil {
			ccmd.Println(err)
		}
	})

	flags = cmd.Flags()
	flags.BoolVarP(&opts.Version, "version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.ConfigDir, "config", cliconfig.Dir(), "Location of client config files")
	opts.Common.InstallFlags(flags)

	cmd.SetOutput(dockerCli.Out())
	cmd.AddCommand(newDaemonCommand())
	commands.AddCommands(cmd, dockerCli)

	return cmd
}

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf(
		"docker: '%s' is not a docker command.\nSee 'docker --help'", args[0])
}

func main() {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	dockerCli := command.NewDockerCli(stdin, stdout, stderr)
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
	fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
}

func dockerPreRun(opts *cliflags.ClientOptions) {
	cliflags.SetLogLevel(opts.Common.LogLevel)

	if opts.ConfigDir != "" {
		cliconfig.SetDir(opts.ConfigDir)
	}

	if opts.Common.Debug {
		debug.Enable()
	}
}

func hideUnsupportedFeatures(cmd *cobra.Command, clientVersion string, hasExperimental bool) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// hide experimental flags
		if !hasExperimental {
			if _, ok := f.Annotations["experimental"]; ok {
				f.Hidden = true
			}
		}

		// hide flags not supported by the server
		if !isFlagSupported(f, clientVersion) {
			f.Hidden = true
		}

	})

	for _, subcmd := range cmd.Commands() {
		// hide experimental subcommands
		if !hasExperimental {
			if _, ok := subcmd.Tags["experimental"]; ok {
				subcmd.Hidden = true
			}
		}

		// hide subcommands not supported by the server
		if subcmdVersion, ok := subcmd.Tags["version"]; ok && versions.LessThan(clientVersion, subcmdVersion) {
			subcmd.Hidden = true
		}
	}
}

func isSupported(cmd *cobra.Command, clientVersion string, hasExperimental bool) error {
	if !hasExperimental {
		if _, ok := cmd.Tags["experimental"]; ok {
			return errors.New("only supported on a Docker daemon with experimental features enabled")
		}
	}

	if cmdVersion, ok := cmd.Tags["version"]; ok && versions.LessThan(clientVersion, cmdVersion) {
		return fmt.Errorf("requires API version %s, but the Docker daemon API version is %s", cmdVersion, clientVersion)
	}

	errs := []string{}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			if !isFlagSupported(f, clientVersion) {
				errs = append(errs, fmt.Sprintf("\"--%s\" requires API version %s, but the Docker daemon API version is %s", f.Name, getFlagVersion(f), clientVersion))
				return
			}
			if _, ok := f.Annotations["experimental"]; ok && !hasExperimental {
				errs = append(errs, fmt.Sprintf("\"--%s\" is only supported on a Docker daemon with experimental features enabled", f.Name))
			}
		}
	})
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func getFlagVersion(f *pflag.Flag) string {
	if flagVersion, ok := f.Annotations["version"]; ok && len(flagVersion) == 1 {
		return flagVersion[0]
	}
	return ""
}

func isFlagSupported(f *pflag.Flag, clientVersion string) bool {
	if v := getFlagVersion(f); v != "" {
		return versions.GreaterThanOrEqualTo(clientVersion, v)
	}
	return true
}
