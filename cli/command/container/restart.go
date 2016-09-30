package container

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type restartOptions struct {
	nSeconds int

	containers []string
}

// NewRestartCommand creates a new cobra.Command for `docker restart`
func NewRestartCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts restartOptions

	cmd := &cobra.Command{
		Use:   "restart [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "Restart one or more containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runRestart(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVarP(&opts.nSeconds, "time", "t", 10, "Seconds to wait for stop before killing the container")
	return cmd
}

func runRestart(dockerCli *command.DockerCli, opts *restartOptions) error {
	ctx := context.Background()
	var errs []string
	for _, name := range opts.containers {
		timeout := time.Duration(opts.nSeconds) * time.Second
		if err := dockerCli.Client().ContainerRestart(ctx, name, &timeout); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
