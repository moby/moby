package container

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

var validDrivers = map[string]bool{
	"json-file": true,
	"journald":  true,
}

type logsOptions struct {
	follow     bool
	since      string
	timestamps bool
	details    bool
	tail       string

	container string
}

// NewLogsCommand creats a new cobra.Command for `docker logs`
func NewLogsCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts logsOptions

	cmd := &cobra.Command{
		Use:   "logs [OPTIONS] CONTAINER",
		Short: "Fetch the logs of a container",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runLogs(dockerCli, &opts)
		},
	}
	cmd.SetFlagErrorFunc(flagErrorFunc)

	flags := cmd.Flags()
	flags.BoolVarP(&opts.follow, "follow", "f", false, "Follow log output")
	flags.StringVar(&opts.since, "since", "", "Show logs since timestamp")
	flags.BoolVarP(&opts.timestamps, "timestamps", "t", false, "Show timestamps")
	flags.BoolVar(&opts.details, "details", false, "Show extra details provided to logs")
	flags.StringVar(&opts.tail, "tail", "all", "Number of lines to show from the end of the logs")
	return cmd
}

func runLogs(dockerCli *client.DockerCli, opts *logsOptions) error {
	ctx := context.Background()

	c, err := dockerCli.Client().ContainerInspect(ctx, opts.container)
	if err != nil {
		return err
	}

	if !validDrivers[c.HostConfig.LogConfig.Type] {
		return fmt.Errorf("\"logs\" command is supported only for \"json-file\" and \"journald\" logging drivers (got: %s)", c.HostConfig.LogConfig.Type)
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      opts.since,
		Timestamps: opts.timestamps,
		Follow:     opts.follow,
		Tail:       opts.tail,
		Details:    opts.details,
	}
	responseBody, err := dockerCli.Client().ContainerLogs(ctx, opts.container, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if c.Config.Tty {
		_, err = io.Copy(dockerCli.Out(), responseBody)
	} else {
		_, err = stdcopy.StdCopy(dockerCli.Out(), dockerCli.Err(), responseBody)
	}
	return err
}
