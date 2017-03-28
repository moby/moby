package plugin

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type enableOpts struct {
	timeout int
	name    string
}

func newEnableCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts enableOpts

	cmd := &cobra.Command{
		Use:   "enable [OPTIONS] PLUGIN",
		Short: "Enable a plugin",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			return runEnable(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&opts.timeout, "timeout", 0, "HTTP client timeout (in seconds)")
	return cmd
}

func runEnable(dockerCli *command.DockerCli, opts *enableOpts) error {
	name := opts.name
	if opts.timeout < 0 {
		return errors.Errorf("negative timeout %d is invalid", opts.timeout)
	}

	if err := dockerCli.Client().PluginEnable(context.Background(), name, types.PluginEnableOptions{Timeout: opts.timeout}); err != nil {
		return err
	}
	fmt.Fprintln(dockerCli.Out(), name)
	return nil
}
