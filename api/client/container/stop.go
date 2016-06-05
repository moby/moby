package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type stopOptions struct {
	time int

	containers []string
}

// NewStopCommand creats a new cobra.Command for `docker stop`
func NewStopCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "Stop one or more running containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runStop(dockerCli, &opts)
		},
	}
	cmd.SetFlagErrorFunc(flagErrorFunc)

	flags := cmd.Flags()
	flags.IntVarP(&opts.time, "time", "t", 10, "Seconds to wait for stop before killing it")
	return cmd
}

func runStop(dockerCli *client.DockerCli, opts *stopOptions) error {
	ctx := context.Background()

	var errs []string
	for _, container := range opts.containers {
		if err := dockerCli.Client().ContainerStop(ctx, container, opts.time); err != nil {
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
