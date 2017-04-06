package container

import (
	"fmt"
	"strings"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type killOptions struct {
	signal string

	containers []string
}

// NewKillCommand creates a new cobra.Command for `docker kill`
func NewKillCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts killOptions

	cmd := &cobra.Command{
		Use:   "kill [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "Kill one or more running containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runKill(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.signal, "signal", "s", "KILL", "Signal to send to the container")
	return cmd
}

func runKill(dockerCli *command.DockerCli, opts *killOptions) error {
	var errs []string
	ctx := context.Background()
	errChan := parallelOperation(ctx, opts.containers, func(ctx context.Context, container string) error {
		return dockerCli.Client().ContainerKill(ctx, container, opts.signal)
	})
	for _, name := range opts.containers {
		if err := <-errChan; err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintln(dockerCli.Out(), name)
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
