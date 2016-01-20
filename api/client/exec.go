package client

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/engine-api/types"
)

// CmdExec runs a command in a running container.
//
// Usage: docker exec [OPTIONS] CONTAINER COMMAND [ARG...]
func (cli *DockerCli) CmdExec(args ...string) error {
	cmd := Cli.Subcmd("exec", []string{"CONTAINER COMMAND [ARG...]"}, Cli.DockerCommands["exec"].Description, true)
	detachKeys := cmd.String([]string{"-detach-keys"}, "", "Override the key sequence for detaching a container")

	execConfig, err := ParseExec(cmd, args)
	// just in case the ParseExec does not exit
	if execConfig.Container == "" || err != nil {
		return Cli.StatusError{StatusCode: 1}
	}

	if *detachKeys != "" {
		cli.configFile.DetachKeys = *detachKeys
	}

	// Send client escape keys
	execConfig.DetachKeys = cli.configFile.DetachKeys

	response, err := cli.client.ContainerExecCreate(*execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(cli.out, "exec ID empty")
		return nil
	}

	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := cli.CheckTtyInput(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
		}

		if err := cli.client.ContainerExecStart(execID, execStartCheck); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(cli.out, "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = cli.in
	}
	if execConfig.AttachStdout {
		out = cli.out
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = cli.out
		} else {
			stderr = cli.err
		}
	}

	resp, err := cli.client.ContainerExecAttach(execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	if in != nil && execConfig.Tty {
		if err := cli.setRawTerminal(); err != nil {
			return err
		}
		defer cli.restoreTerminal(in)
	}
	errCh = promise.Go(func() error {
		return cli.holdHijackedConnection(execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && cli.isTerminalIn {
		if err := cli.monitorTtySize(execID, true); err != nil {
			fmt.Fprintf(cli.err, "Error monitoring TTY size: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = getExecExitCode(cli, execID); err != nil {
		return err
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}

// ParseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
// If the minimal number of specified args is not right or if specified args are
// not valid, it will return an error.
func ParseExec(cmd *flag.FlagSet, args []string) (*types.ExecConfig, error) {
	var (
		flStdin      = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty        = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run command in the background")
		flUser       = cmd.String([]string{"u", "-user"}, "", "Username or UID (format: <name|uid>[:<group|gid>])")
		flPrivileged = cmd.Bool([]string{"-privileged"}, false, "Give extended privileges to the command")
		execCmd      []string
		container    string
	)
	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return nil, err
	}
	container = cmd.Arg(0)
	parsedArgs := cmd.Args()
	execCmd = parsedArgs[1:]

	execConfig := &types.ExecConfig{
		User:       *flUser,
		Privileged: *flPrivileged,
		Tty:        *flTty,
		Cmd:        execCmd,
		Container:  container,
		Detach:     *flDetach,
	}

	// If -d is not set, attach to everything by default
	if !*flDetach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if *flStdin {
			execConfig.AttachStdin = true
		}
	}

	return execConfig, nil
}
