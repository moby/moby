package container

import (
	"io"
	"net/http/httputil"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type attachOptions struct {
	noStdin    bool
	proxy      bool
	detachKeys string

	container string
}

func containerInspect(cli client.APIClient, ctx context.Context, args string) (*types.ContainerJSON, error) {
	c, err := cli.ContainerInspect(ctx, args)
	if err != nil {
		return nil, err
	}
	if !c.State.Running {
		return nil, errors.New("You cannot attach to a stopped container, start it first")
	}
	if c.State.Paused {
		return nil, errors.New("You cannot attach to a paused container, unpause it first")
	}
	return &c, nil
}

// NewAttachCommand creates a new cobra.Command for `docker attach`
func NewAttachCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts attachOptions

	cmd := &cobra.Command{
		Use:   "attach [OPTIONS] CONTAINER",
		Short: "Attach local standard input, output, and error streams to a running container",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runAttach(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noStdin, "no-stdin", false, "Do not attach STDIN")
	flags.BoolVar(&opts.proxy, "sig-proxy", true, "Proxy all received signals to the process")
	flags.StringVar(&opts.detachKeys, "detach-keys", "", "Override the key sequence for detaching a container")
	return cmd
}

func runAttach(dockerCli *command.DockerCli, opts *attachOptions) error {
	ctx := context.Background()
	client := dockerCli.Client()

	c, err := containerInspect(client, ctx, opts.container)
	if err != nil {
		return err
	}

	if !c.State.Running {
		return errors.New("You cannot attach to a stopped container, start it first")
	}

	if c.State.Paused {
		return errors.New("You cannot attach to a paused container, unpause it first")
	}

	if err := dockerCli.In().CheckTty(!opts.noStdin, c.Config.Tty); err != nil {
		return err
	}

	if opts.detachKeys != "" {
		dockerCli.ConfigFile().DetachKeys = opts.detachKeys
	}

	options := types.ContainerAttachOptions{
		Stream:     true,
		Stdin:      !opts.noStdin && c.Config.OpenStdin,
		Stdout:     true,
		Stderr:     true,
		DetachKeys: dockerCli.ConfigFile().DetachKeys,
	}

	var in io.ReadCloser
	if options.Stdin {
		in = dockerCli.In()
	}

	if opts.proxy && !c.Config.Tty {
		sigc := ForwardAllSignals(ctx, dockerCli, opts.container)
		defer signal.StopCatch(sigc)
	}

	resp, errAttach := client.ContainerAttach(ctx, opts.container, options)
	if errAttach != nil && errAttach != httputil.ErrPersistEOF {
		// ContainerAttach returns an ErrPersistEOF (connection closed)
		// means server met an error and put it in Hijacked connection
		// keep the error and read detailed error message from hijacked connection later
		return errAttach
	}
	defer resp.Close()

	// If use docker attach command to attach to a stop container, it will return
	// "You cannot attach to a stopped container" error, it's ok, but when
	// attach to a running container, it(docker attach) use inspect to check
	// the container's state, if it pass the state check on the client side,
	// and then the container is stopped, docker attach command still attach to
	// the container and not exit.
	//
	// Recheck the container's state to avoid attach block.
	_, err = containerInspect(client, ctx, opts.container)
	if err != nil {
		return err
	}

	if c.Config.Tty && dockerCli.Out().IsTerminal() {
		height, width := dockerCli.Out().GetTtySize()
		// To handle the case where a user repeatedly attaches/detaches without resizing their
		// terminal, the only way to get the shell prompt to display for attaches 2+ is to artificially
		// resize it, then go back to normal. Without this, every attach after the first will
		// require the user to manually resize or hit enter.
		resizeTtyTo(ctx, client, opts.container, height+1, width+1, false)

		// After the above resizing occurs, the call to MonitorTtySize below will handle resetting back
		// to the actual size.
		if err := MonitorTtySize(ctx, dockerCli, opts.container, false); err != nil {
			logrus.Debugf("Error monitoring TTY size: %s", err)
		}
	}
	if err := holdHijackedConnection(ctx, dockerCli, c.Config.Tty, in, dockerCli.Out(), dockerCli.Err(), resp); err != nil {
		return err
	}

	if errAttach != nil {
		return errAttach
	}

	_, status, err := getExitCode(ctx, dockerCli, opts.container)
	if err != nil {
		return err
	}
	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}

	return nil
}
