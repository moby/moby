package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type pauseOptions struct {
	containers []string
}

// NewPauseCommand creats a new cobra.Command for `docker pause`
func NewPauseCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts pauseOptions

	cmd := &cobra.Command{
		Use:   "pause CONTAINER [CONTAINER...]",
		Short: "Pause all processes within one or more containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runPause(dockerCli, &opts)
		},
	}
	cmd.SetFlagErrorFunc(flagErrorFunc)

	return cmd
}

func runPause(dockerCli *client.DockerCli, opts *pauseOptions) error {
	ctx := context.Background()

	var errs []string
	for _, container := range opts.containers {
		if err := dockerCli.Client().ContainerPause(ctx, container); err != nil {
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
