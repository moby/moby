package service

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/stringid"
	"github.com/spf13/cobra"
)

type logsOptions struct {
	noResolve  bool
	noTrunc    bool
	noTaskIDs  bool
	follow     bool
	since      string
	timestamps bool
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
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&opts.noTaskIDs, "no-task-ids", false, "Do not include task IDs")
	flags.BoolVarP(&opts.follow, "follow", "f", false, "Follow log output")
	flags.StringVar(&opts.since, "since", "", "Show logs since timestamp (e.g. 2013-01-02T13:23:37) or relative (e.g. 42m for 42 minutes)")
	flags.BoolVarP(&opts.timestamps, "timestamps", "t", false, "Show timestamps")
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
	}

	client := dockerCli.Client()

	service, _, err := client.ServiceInspectWithRaw(ctx, opts.service)
	if err != nil {
		return err
	}

	responseBody, err := client.ServiceLogs(ctx, opts.service, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	var replicas uint64
	padding := 1
	if service.Spec.Mode.Replicated != nil && service.Spec.Mode.Replicated.Replicas != nil {
		// if replicas are initialized, figure out if we need to pad them
		replicas = *service.Spec.Mode.Replicated.Replicas
		padding = len(strconv.FormatUint(replicas, 10))
	}

	taskFormatter := newTaskFormatter(client, opts, padding)

	stdout := &logWriter{ctx: ctx, opts: opts, f: taskFormatter, w: dockerCli.Out()}
	stderr := &logWriter{ctx: ctx, opts: opts, f: taskFormatter, w: dockerCli.Err()}

	// TODO(aluzzardi): Do an io.Copy for services with TTY enabled.
	_, err = stdcopy.StdCopy(stdout, stderr, responseBody)
	return err
}

type taskFormatter struct {
	client  client.APIClient
	opts    *logsOptions
	padding int

	r     *idresolver.IDResolver
	cache map[logContext]string
}

func newTaskFormatter(client client.APIClient, opts *logsOptions, padding int) *taskFormatter {
	return &taskFormatter{
		client:  client,
		opts:    opts,
		padding: padding,
		r:       idresolver.New(client, opts.noResolve),
		cache:   make(map[logContext]string),
	}
}

func (f *taskFormatter) format(ctx context.Context, logCtx logContext) (string, error) {
	if cached, ok := f.cache[logCtx]; ok {
		return cached, nil
	}

	nodeName, err := f.r.Resolve(ctx, swarm.Node{}, logCtx.nodeID)
	if err != nil {
		return "", err
	}

	serviceName, err := f.r.Resolve(ctx, swarm.Service{}, logCtx.serviceID)
	if err != nil {
		return "", err
	}

	task, _, err := f.client.TaskInspectWithRaw(ctx, logCtx.taskID)
	if err != nil {
		return "", err
	}

	taskName := fmt.Sprintf("%s.%d", serviceName, task.Slot)
	if !f.opts.noTaskIDs {
		if f.opts.noTrunc {
			taskName += fmt.Sprintf(".%s", task.ID)
		} else {
			taskName += fmt.Sprintf(".%s", stringid.TruncateID(task.ID))
		}
	}
	padding := strings.Repeat(" ", f.padding-len(strconv.FormatInt(int64(task.Slot), 10)))
	formatted := fmt.Sprintf("%s@%s%s", taskName, nodeName, padding)
	f.cache[logCtx] = formatted
	return formatted, nil
}

type logWriter struct {
	ctx  context.Context
	opts *logsOptions
	f    *taskFormatter
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

	logCtx, err := lw.parseContext(string(parts[contextIndex]))
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
			formatted, err := lw.f.format(lw.ctx, logCtx)
			if err != nil {
				return 0, err
			}
			output = append(output, []byte(fmt.Sprintf("%s    |", formatted))...)
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

func (lw *logWriter) parseContext(input string) (logContext, error) {
	context := make(map[string]string)

	components := strings.Split(input, ",")
	for _, component := range components {
		parts := strings.SplitN(component, "=", 2)
		if len(parts) != 2 {
			return logContext{}, fmt.Errorf("invalid context: %s", input)
		}
		context[parts[0]] = parts[1]
	}

	nodeID, ok := context["com.docker.swarm.node.id"]
	if !ok {
		return logContext{}, fmt.Errorf("missing node id in context: %s", input)
	}

	serviceID, ok := context["com.docker.swarm.service.id"]
	if !ok {
		return logContext{}, fmt.Errorf("missing service id in context: %s", input)
	}

	taskID, ok := context["com.docker.swarm.task.id"]
	if !ok {
		return logContext{}, fmt.Errorf("missing task id in context: %s", input)
	}

	return logContext{
		nodeID:    nodeID,
		serviceID: serviceID,
		taskID:    taskID,
	}, nil
}

type logContext struct {
	nodeID    string
	serviceID string
	taskID    string
}
