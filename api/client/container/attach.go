package container

import (
	"fmt"
	"io"
	"net/http/httputil"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type attachOptions struct {
	noStdin    bool
	proxy      bool
	detachKeys string

	container string
}

// NewAttachCommand creats a new cobra.Command for `docker attach`
func NewAttachCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts attachOptions

	cmd := &cobra.Command{
		Use:   "attach [OPTIONS] CONTAINER",
		Short: "Attach to a running container",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runAttach(dockerCli, &opts)
		},
	}
	cmd.SetFlagErrorFunc(flagErrorFunc)

	flags := cmd.Flags()
	flags.BoolVar(&opts.noStdin, "no-stdin", false, "Do not attach STDIN")
	flags.BoolVar(&opts.proxy, "sig-proxy", true, "Proxy all received signals to the process")
	flags.StringVar(&opts.detachKeys, "detach-keys", "", "Override the key sequence for detaching a container")
	return cmd
}

func runAttach(dockerCli *client.DockerCli, opts *attachOptions) error {
	ctx := context.Background()

	c, err := dockerCli.Client().ContainerInspect(ctx, opts.container)
	if err != nil {
		return err
	}

	if !c.State.Running {
		return fmt.Errorf("You cannot attach to a stopped container, start it first")
	}

	if c.State.Paused {
		return fmt.Errorf("You cannot attach to a paused container, unpause it first")
	}

	if err := dockerCli.CheckTtyInput(!opts.noStdin, c.Config.Tty); err != nil {
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
		sigc := dockerCli.ForwardAllSignals(ctx, opts.container)
		defer signal.StopCatch(sigc)
	}

	resp, errAttach := dockerCli.Client().ContainerAttach(ctx, opts.container, options)
	if errAttach != nil && errAttach != httputil.ErrPersistEOF {
		// ContainerAttach returns an ErrPersistEOF (connection closed)
		// means server met an error and put it in Hijacked connection
		// keep the error and read detailed error message from hijacked connection later
		return errAttach
	}
	defer resp.Close()

	if c.Config.Tty && dockerCli.IsTerminalOut() {
		height, width := dockerCli.GetTtySize()
		// To handle the case where a user repeatedly attaches/detaches without resizing their
		// terminal, the only way to get the shell prompt to display for attaches 2+ is to artificially
		// resize it, then go back to normal. Without this, every attach after the first will
		// require the user to manually resize or hit enter.
		dockerCli.ResizeTtyTo(ctx, opts.container, height+1, width+1, false)

		// After the above resizing occurs, the call to MonitorTtySize below will handle resetting back
		// to the actual size.
		if err := dockerCli.MonitorTtySize(ctx, opts.container, false); err != nil {
			logrus.Debugf("Error monitoring TTY size: %s", err)
		}
	}
	if err := dockerCli.HoldHijackedConnection(ctx, c.Config.Tty, in, dockerCli.Out(), dockerCli.Err(), resp); err != nil {
		return err
	}

	if errAttach != nil {
		return errAttach
	}

	_, status, err := dockerCli.GetExitCode(ctx, opts.container)
	if err != nil {
		return err
	}
	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}

	return nil
}
