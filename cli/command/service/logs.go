package service

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
)

type logsOptions struct {
	noResolve  bool
	follow     bool
	since      string
	timestamps bool
	details    bool
	tail       string

	service string
}

func newLogsCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts logsOptions

	cmd := &cobra.Command{
		Use:   "logs [OPTIONS] SERVICE",
		Short: "Fetch the logs of a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.service = args[0]
			return runLogs(dockerCli, &opts)
		},
		Tags: map[string]string{"experimental": ""},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "Do not map IDs to Names")
	flags.BoolVarP(&opts.follow, "follow", "f", false, "Follow log output")
	flags.StringVar(&opts.since, "since", "", "Show logs since timestamp")
	flags.BoolVarP(&opts.timestamps, "timestamps", "t", false, "Show timestamps")
	flags.BoolVar(&opts.details, "details", false, "Show extra details provided to logs")
	flags.StringVar(&opts.tail, "tail", "all", "Number of lines to show from the end of the logs")
	return cmd
}

func runLogs(dockerCli *command.DockerCli, opts *logsOptions) error {
	ctx := context.Background()

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      opts.since,
		Timestamps: opts.timestamps,
		Follow:     opts.follow,
		Tail:       opts.tail,
		Details:    opts.details,
	}

	client := dockerCli.Client()
	responseBody, err := client.ServiceLogs(ctx, opts.service, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	resolver := idresolver.New(client, opts.noResolve)

	stdout := &logWriter{ctx: ctx, opts: opts, r: resolver, w: dockerCli.Out()}
	stderr := &logWriter{ctx: ctx, opts: opts, r: resolver, w: dockerCli.Err()}

	// TODO(aluzzardi): Do an io.Copy for services with TTY enabled.
	_, err = stdcopy.StdCopy(stdout, stderr, responseBody)
	return err
}

type logWriter struct {
	ctx  context.Context
	opts *logsOptions
	r    *idresolver.IDResolver
	w    io.Writer
}

func (lw *logWriter) Write(buf []byte) (int, error) {
	contextIndex := 0
	numParts := 2
	if lw.opts.timestamps {
		contextIndex++
		numParts++
	}

	parts := bytes.SplitN(buf, []byte(" "), numParts)
	if len(parts) != numParts {
		return 0, fmt.Errorf("invalid context in log message: %v", string(buf))
	}

	taskName, nodeName, err := lw.parseContext(string(parts[contextIndex]))
	if err != nil {
		return 0, err
	}

	output := []byte{}
	for i, part := range parts {
		// First part doesn't get space separation.
		if i > 0 {
			output = append(output, []byte(" ")...)
		}

		if i == contextIndex {
			// TODO(aluzzardi): Consider constant padding.
			output = append(output, []byte(fmt.Sprintf("%s@%s    |", taskName, nodeName))...)
		} else {
			output = append(output, part...)
		}
	}
	_, err = lw.w.Write(output)
	if err != nil {
		return 0, err
	}

	return len(buf), nil
}

func (lw *logWriter) parseContext(input string) (string, string, error) {
	context := make(map[string]string)

	components := strings.Split(input, ",")
	for _, component := range components {
		parts := strings.SplitN(component, "=", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid context: %s", input)
		}
		context[parts[0]] = parts[1]
	}

	taskID, ok := context["com.docker.swarm.task.id"]
	if !ok {
		return "", "", fmt.Errorf("missing task id in context: %s", input)
	}
	taskName, err := lw.r.Resolve(lw.ctx, swarm.Task{}, taskID)
	if err != nil {
		return "", "", err
	}

	nodeID, ok := context["com.docker.swarm.node.id"]
	if !ok {
		return "", "", fmt.Errorf("missing node id in context: %s", input)
	}
	nodeName, err := lw.r.Resolve(lw.ctx, swarm.Node{}, nodeID)
	if err != nil {
		return "", "", err
	}

	return taskName, nodeName, nil
}
