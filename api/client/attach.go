package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
)

// CmdAttach attaches to a running container.
//
// Usage: docker attach [OPTIONS] CONTAINER
func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := Cli.Subcmd("attach", []string{"CONTAINER"}, Cli.DockerCommands["attach"].Description, true)
	noStdin := cmd.Bool([]string{"-no-stdin"}, false, "Do not attach STDIN")
	proxy := cmd.Bool([]string{"-sig-proxy"}, true, "Proxy all received signals to the process")

	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	serverResp, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil, nil)
	if err != nil {
		return err
	}

	defer serverResp.body.Close()

	var c types.ContainerJSON
	if err := json.NewDecoder(serverResp.body).Decode(&c); err != nil {
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

	var in io.ReadCloser

	v := url.Values{}
	v.Set("stream", "1")
	if !*noStdin && c.Config.OpenStdin {
		v.Set("stdin", "1")
		in = cli.in
	}

	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if *proxy && !c.Config.Tty {
		sigc := cli.forwardAllSignals(cmd.Arg(0))
		defer signal.StopCatch(sigc)
	}

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), c.Config.Tty, in, cli.out, cli.err, nil, nil); err != nil {
		return err
	}

	_, status, err := getExitCode(cli, cmd.Arg(0))
	if err != nil {
		return err
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}
