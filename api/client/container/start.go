package container

import (
	"fmt"
	"io"
	"net/http/httputil"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type startOptions struct {
	attach     bool
	openStdin  bool
	detachKeys string

	containers []string
}

// NewStartCommand creats a new cobra.Command for `docker start`
func NewStartCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "Start one or more stopped containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runStart(dockerCli, &opts)
		},
	}
	cmd.SetFlagErrorFunc(flagErrorFunc)

	flags := cmd.Flags()
	flags.BoolVarP(&opts.attach, "attach", "a", false, "Attach STDOUT/STDERR and forward signals")
	flags.BoolVarP(&opts.openStdin, "interactive", "i", false, "Attach container's STDIN")
	flags.StringVar(&opts.detachKeys, "detach-keys", "", "Override the key sequence for detaching a container")
	return cmd
}

func runStart(dockerCli *client.DockerCli, opts *startOptions) error {
	ctx, cancelFun := context.WithCancel(context.Background())

	if opts.attach || opts.openStdin {
		// We're going to attach to a container.
		// 1. Ensure we only have one container.
		if len(opts.containers) > 1 {
			return fmt.Errorf("You cannot start and attach multiple containers at once.")
		}

		// 2. Attach to the container.
		container := opts.containers[0]
		c, err := dockerCli.Client().ContainerInspect(ctx, container)
		if err != nil {
			return err
		}

		if !c.Config.Tty {
			sigc := dockerCli.ForwardAllSignals(ctx, container)
			defer signal.StopCatch(sigc)
		}

		if opts.detachKeys != "" {
			dockerCli.ConfigFile().DetachKeys = opts.detachKeys
		}

		options := types.ContainerAttachOptions{
			Stream:     true,
			Stdin:      opts.openStdin && c.Config.OpenStdin,
			Stdout:     true,
			Stderr:     true,
			DetachKeys: dockerCli.ConfigFile().DetachKeys,
		}

		var in io.ReadCloser

		if options.Stdin {
			in = dockerCli.In()
		}

		resp, errAttach := dockerCli.Client().ContainerAttach(ctx, container, options)
		if errAttach != nil && errAttach != httputil.ErrPersistEOF {
			// ContainerAttach return an ErrPersistEOF (connection closed)
			// means server met an error and put it in Hijacked connection
			// keep the error and read detailed error message from hijacked connection
			return errAttach
		}
		defer resp.Close()
		cErr := promise.Go(func() error {
			errHijack := dockerCli.HoldHijackedConnection(ctx, c.Config.Tty, in, dockerCli.Out(), dockerCli.Err(), resp)
			if errHijack == nil {
				return errAttach
			}
			return errHijack
		})

		// 3. Start the container.
		if err := dockerCli.Client().ContainerStart(ctx, container, types.ContainerStartOptions{}); err != nil {
			cancelFun()
			<-cErr
			return err
		}

		// 4. Wait for attachment to break.
		if c.Config.Tty && dockerCli.IsTerminalOut() {
			if err := dockerCli.MonitorTtySize(ctx, container, false); err != nil {
				fmt.Fprintf(dockerCli.Err(), "Error monitoring TTY size: %s\n", err)
			}
		}
		if attchErr := <-cErr; attchErr != nil {
			return attchErr
		}
		_, status, err := dockerCli.GetExitCode(ctx, container)
		if err != nil {
			return err
		}
		if status != 0 {
			return cli.StatusError{StatusCode: status}
		}
	} else {
		// We're not going to attach to anything.
		// Start as many containers as we want.
		return startContainersWithoutAttachments(dockerCli, ctx, opts.containers)
	}

	return nil
}

func startContainersWithoutAttachments(dockerCli *client.DockerCli, ctx context.Context, containers []string) error {
	var failedContainers []string
	for _, container := range containers {
		if err := dockerCli.Client().ContainerStart(ctx, container, types.ContainerStartOptions{}); err != nil {
			fmt.Fprintf(dockerCli.Err(), "%s\n", err)
			failedContainers = append(failedContainers, container)
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", container)
		}
	}

	if len(failedContainers) > 0 {
		return fmt.Errorf("Error: failed to start containers: %v", strings.Join(failedContainers, ", "))
	}
	return nil
}
