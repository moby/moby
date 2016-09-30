package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type unpauseOptions struct {
	containers []string
}

// NewUnpauseCommand creates a new cobra.Command for `docker unpause`
func NewUnpauseCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts unpauseOptions

	cmd := &cobra.Command{
		Use:   "unpause CONTAINER [CONTAINER...]",
		Short: "Unpause all processes within one or more containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runUnpause(dockerCli, &opts)
		},
	}
	return cmd
}

func runUnpause(dockerCli *command.DockerCli, opts *unpauseOptions) error {
	ctx := context.Background()

	var errs []string
	errChan := parallelOperation(ctx, opts.containers, dockerCli.Client().ContainerUnpause)
	for _, container := range opts.containers {
		if err := <-errChan; err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", container)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
