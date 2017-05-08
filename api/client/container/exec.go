package container

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type execOptions struct {
	detachKeys  string
	interactive bool
	tty         bool
	detach      bool
	user        string
	privileged  bool
}

// NewExecCommand creats a new cobra.Command for `docker exec`
func NewExecCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts execOptions

	cmd := &cobra.Command{
		Use:   "exec [OPTIONS] CONTAINER COMMAND [ARG...]",
		Short: "Run a command in a running container",
		Args:  cli.RequiresMinArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			container := args[0]
			execCmd := args[1:]
			return runExec(dockerCli, &opts, container, execCmd)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.StringVarP(&opts.detachKeys, "detach-keys", "", "", "Override the key sequence for detaching a container")
	flags.BoolVarP(&opts.interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&opts.tty, "tty", "t", false, "Allocate a pseudo-TTY")
	flags.BoolVarP(&opts.detach, "detach", "d", false, "Detached mode: run command in the background")
	flags.StringVarP(&opts.user, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	flags.BoolVarP(&opts.privileged, "privileged", "", false, "Give extended privileges to the command")

	return cmd
}

func runExec(dockerCli *client.DockerCli, opts *execOptions, container string, execCmd []string) error {
	execConfig, err := parseExec(opts, container, execCmd)
	// just in case the ParseExec does not exit
	if container == "" || err != nil {
		return cli.StatusError{StatusCode: 1}
	}

	if opts.detachKeys != "" {
		dockerCli.ConfigFile().DetachKeys = opts.detachKeys
	}

	// Send client escape keys
	execConfig.DetachKeys = dockerCli.ConfigFile().DetachKeys

	ctx := context.Background()

	response, err := dockerCli.Client().ContainerExecCreate(ctx, container, *execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(dockerCli.Out(), "exec ID empty")
		return nil
	}

	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := dockerCli.CheckTtyInput(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
		}

		if err := dockerCli.Client().ContainerExecStart(ctx, execID, execStartCheck); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(dockerCli.Out(), "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = dockerCli.In()
	}
	if execConfig.AttachStdout {
		out = dockerCli.Out()
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = dockerCli.Out()
		} else {
			stderr = dockerCli.Err()
		}
	}

	resp, err := dockerCli.Client().ContainerExecAttach(ctx, execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	errCh = promise.Go(func() error {
		return dockerCli.HoldHijackedConnection(ctx, execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && dockerCli.IsTerminalIn() {
		if err := dockerCli.MonitorTtySize(ctx, execID, true); err != nil {
			fmt.Fprintf(dockerCli.Err(), "Error monitoring TTY size: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = dockerCli.GetExecExitCode(ctx, execID); err != nil {
		return err
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}

	return nil
}

// parseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
func parseExec(opts *execOptions, container string, execCmd []string) (*types.ExecConfig, error) {
	execConfig := &types.ExecConfig{
		User:       opts.user,
		Privileged: opts.privileged,
		Tty:        opts.tty,
		Cmd:        execCmd,
		Detach:     opts.detach,
		// container is not used here
	}

	// If -d is not set, attach to everything by default
	if !opts.detach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if opts.interactive {
			execConfig.AttachStdin = true
		}
	}

	return execConfig, nil
}
