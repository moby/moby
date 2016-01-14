package client

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types"
)

// CmdAttach attaches to a running container.
//
// Usage: docker attach [OPTIONS] CONTAINER
func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := Cli.Subcmd("attach", []string{"CONTAINER"}, Cli.DockerCommands["attach"].Description, true)
	noStdin := cmd.Bool([]string{"-no-stdin"}, false, "Do not attach STDIN")
	proxy := cmd.Bool([]string{"-sig-proxy"}, true, "Proxy all received signals to the process")
	detachKeys := cmd.String([]string{"-detach-keys"}, "", "Override the key sequence for detaching a container")

	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	c, err := cli.client.ContainerInspect(cmd.Arg(0))
	if err != nil {
		return err
	}

	if !c.State.Running {
		return fmt.Errorf("You cannot attach to a stopped container, start it first")
	}

	if c.State.Paused {
		return fmt.Errorf("You cannot attach to a paused container, unpause it first")
	}

	if err := cli.CheckTtyInput(!*noStdin, c.Config.Tty); err != nil {
		return err
	}

	if c.Config.Tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(cmd.Arg(0), false); err != nil {
			logrus.Debugf("Error monitoring TTY size: %s", err)
		}
	}

	if *detachKeys != "" {
		cli.configFile.DetachKeys = *detachKeys
	}

	options := types.ContainerAttachOptions{
		ContainerID: cmd.Arg(0),
		Stream:      true,
		Stdin:       !*noStdin && c.Config.OpenStdin,
		Stdout:      true,
		Stderr:      true,
		DetachKeys:  cli.configFile.DetachKeys,
	}

	var in io.ReadCloser
	if options.Stdin {
		in = cli.in
	}

	if *proxy && !c.Config.Tty {
		sigc := cli.forwardAllSignals(options.ContainerID)
		defer signal.StopCatch(sigc)
	}

	resp, err := cli.client.ContainerAttach(options)
	if err != nil {
		return err
	}
	defer resp.Close()

	if err := cli.holdHijackedConnection(c.Config.Tty, in, cli.out, cli.err, resp); err != nil {
		return err
	}

	_, status, err := getExitCode(cli, options.ContainerID)
	if err != nil {
		return err
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}
