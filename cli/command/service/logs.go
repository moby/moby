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
	"github.com/pkg/errors"
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

	target string
}

// TODO(dperny) the whole CLI for this is kind of a mess IMHOIRL and it needs
// to be refactored agressively. There may be changes to the implementation of
// details, which will be need to be reflected in this code. The refactoring
// should be put off until we make those changes, tho, because I think the
// decisions made WRT details will impact the design of the CLI.
func newLogsCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts logsOptions

	cmd := &cobra.Command{
		Use:   "logs [OPTIONS] SERVICE",
		Short: "Fetch the logs of a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.target = args[0]
			return runLogs(dockerCli, &opts)
		},
		Tags: map[string]string{"experimental": ""},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "Do not map IDs to Names in output")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&opts.noTaskIDs, "no-task-ids", false, "Do not include task IDs in output")
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
		Details:    true,
	}

	cli := dockerCli.Client()

	var (
		maxLength    = 1
		responseBody io.ReadCloser
		tty          bool
	)

	service, _, err := cli.ServiceInspectWithRaw(ctx, opts.target)
	if err != nil {
		// if it's any error other than service not found, it's Real
		if !client.IsErrServiceNotFound(err) {
			return err
		}
		task, _, err := cli.TaskInspectWithRaw(ctx, opts.target)
		tty = task.Spec.ContainerSpec.TTY
		// TODO(dperny) hot fix until we get a nice details system squared away,
		// ignores details (including task context) if we have a TTY log
		if tty {
			options.Details = false
		}

		responseBody, err = cli.TaskLogs(ctx, opts.target, options)
		if err != nil {
			if client.IsErrTaskNotFound(err) {
				// if the task ALSO isn't found, rewrite the error to be clear
				// that we looked for services AND tasks
				err = fmt.Errorf("No such task or service")
			}
			return err
		}
		maxLength = getMaxLength(task.Slot)
		responseBody, err = cli.TaskLogs(ctx, opts.target, options)
	} else {
		tty = service.Spec.TaskTemplate.ContainerSpec.TTY
		// TODO(dperny) hot fix until we get a nice details system squared away,
		// ignores details (including task context) if we have a TTY log
		if tty {
			options.Details = false
		}

		responseBody, err = cli.ServiceLogs(ctx, opts.target, options)
		if err != nil {
			return err
		}
		if service.Spec.Mode.Replicated != nil && service.Spec.Mode.Replicated.Replicas != nil {
			// if replicas are initialized, figure out if we need to pad them
			replicas := *service.Spec.Mode.Replicated.Replicas
			maxLength = getMaxLength(int(replicas))
		}
	}
	defer responseBody.Close()

	if tty {
		_, err = io.Copy(dockerCli.Out(), responseBody)
		return err
	}

	taskFormatter := newTaskFormatter(cli, opts, maxLength)

	stdout := &logWriter{ctx: ctx, opts: opts, f: taskFormatter, w: dockerCli.Out()}
	stderr := &logWriter{ctx: ctx, opts: opts, f: taskFormatter, w: dockerCli.Err()}

	// TODO(aluzzardi): Do an io.Copy for services with TTY enabled.
	_, err = stdcopy.StdCopy(stdout, stderr, responseBody)
	return err
}

// getMaxLength gets the maximum length of the number in base 10
func getMaxLength(i int) int {
	return len(strconv.FormatInt(int64(i), 10))
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

	padding := strings.Repeat(" ", f.padding-getMaxLength(task.Slot))
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
		return 0, errors.Errorf("invalid context in log message: %v", string(buf))
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
			return logContext{}, errors.Errorf("invalid context: %s", input)
		}
		context[parts[0]] = parts[1]
	}

	nodeID, ok := context["com.docker.swarm.node.id"]
	if !ok {
		return logContext{}, errors.Errorf("missing node id in context: %s", input)
	}

	serviceID, ok := context["com.docker.swarm.service.id"]
	if !ok {
		return logContext{}, errors.Errorf("missing service id in context: %s", input)
	}

	taskID, ok := context["com.docker.swarm.task.id"]
	if !ok {
		return logContext{}, errors.Errorf("missing task id in context: %s", input)
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
