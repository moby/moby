package command

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/log"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/dockerversion"
	"github.com/spf13/cobra"
)

var honorXDG bool

func newDaemonCommand(stderr io.Writer) (*cobra.Command, error) {
	// FIXME(thaJeztah): config.New also looks up default binary-path, but this code is also executed when running "--version".
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}
	opts := newDaemonOptions(cfg)

	cmd := &cobra.Command{
		Use:           "dockerd [OPTIONS]",
		Short:         "A self-sufficient runtime for containers.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.flags = cmd.Flags()

			cli, err := newDaemonCLI(opts)
			if err != nil {
				return err
			}
			if opts.Validate {
				// If config wasn't OK we wouldn't have made it this far.
				_, _ = fmt.Fprintln(stderr, "configuration OK")
				return nil
			}

			return runDaemon(cmd.Context(), cli)
		},
		DisableFlagsInUseLine: true,
		Version:               fmt.Sprintf("%s, build %s", dockerversion.Version, dockerversion.GitCommit),
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd:   false,
			HiddenDefaultCmd:    true,
			DisableDescriptions: false,
		},
	}

	// Cobra's [Command.InitDefaultCompletionCmd] has a special-case for
	// binaries/commands that don't have subcommands, and does not set up
	// the default completion command in that case.
	//
	// Unfortunately, the definition of the default completion commands
	// is not exported, and we don't want to replicate them. As a workaround,
	// we're adding a hidden dummy-command to trick Cobra into applying
	// the default.
	//
	// TODO(thaJeztah): consider contributing to Cobra to either allow explicitly enabling, or to export the default completion commands
	//
	// [Command.InitDefaultCompletionCmd]: https://github.com/spf13/cobra/blob/v1.8.1/completions.go#L685-L698
	cmd.AddCommand(&cobra.Command{
		Use:    "__dummy_command",
		Hidden: true,
	})

	SetupRootCommand(cmd)

	flags := cmd.Flags()
	flags.BoolP("version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.configFile, "config-file", opts.configFile, "Daemon configuration file")
	opts.installFlags(flags)
	installConfigFlags(opts.daemonConfig, flags)
	installServiceFlags(flags)

	return cmd, nil
}

func init() {
	if dockerversion.ProductName != "" {
		apicaps.ExportedProduct = dockerversion.ProductName
	}
	// When running with RootlessKit, $XDG_RUNTIME_DIR, $XDG_DATA_HOME, and $XDG_CONFIG_HOME needs to be
	// honored as the default dirs, because we are unlikely to have permissions to access the system-wide
	// directories.
	//
	// Note that even running with --rootless, when not running with RootlessKit, honorXDG needs to be kept false,
	// because the system-wide directories in the current mount namespace are expected to be accessible.
	// ("rootful" dockerd in rootless dockerd, #38702)
	honorXDG = rootless.RunningWithRootlessKit()
}

// Runner is used to run the daemon command
type Runner interface {
	Run(context.Context) error
}

type daemonRunner struct {
	*cobra.Command
}

func (d daemonRunner) Run(ctx context.Context) error {
	configureGRPCLog(ctx)

	return d.ExecuteContext(ctx)
}

// NewDaemonRunner creates a new daemon runner with the given
// stdout and stderr writers.
func NewDaemonRunner(stdout, stderr io.Writer) (Runner, error) {
	err := log.SetFormat(log.TextFormat)
	if err != nil {
		return nil, err
	}

	initLogging(stdout, stderr)

	cmd, err := newDaemonCommand(stderr)
	if err != nil {
		return nil, err
	}
	cmd.SetOut(stdout)

	return daemonRunner{cmd}, nil
}
