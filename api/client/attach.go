package client

import (
	"fmt"
	"io"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/utils"
)

// CmdAttach attaches to a running container.
//
// Usage: docker attach [OPTIONS] CONTAINER
func (cli *DockerCli) CmdAttach(args ...string) error {
	var (
		cmd     = cli.Subcmd("attach", "CONTAINER", "Attach to a running container", true)
		noStdin = cmd.Bool([]string{"#nostdin", "-no-stdin"}, false, "Do not attach STDIN")
		proxy   = cmd.Bool([]string{"#sig-proxy", "-sig-proxy"}, true, "Proxy all received signals to the process")
	)
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)
	name := cmd.Arg(0)

	stream, _, err := cli.call("GET", "/containers/"+name+"/json", nil, nil)
	if err != nil {
		return err
	}

	env := engine.Env{}
	if err := env.Decode(stream); err != nil {
		return err
	}

	if !env.GetSubEnv("State").GetBool("Running") {
		return fmt.Errorf("You cannot attach to a stopped container, start it first")
	}

	var (
		config = env.GetSubEnv("Config")
		tty    = config.GetBool("Tty")
	)

	if err := cli.CheckTtyInput(!*noStdin, tty); err != nil {
		return err
	}

	if tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(cmd.Arg(0), false); err != nil {
			log.Debugf("Error monitoring TTY size: %s", err)
		}
	}

	var in io.ReadCloser

	v := url.Values{}
	v.Set("stream", "1")
	if !*noStdin && config.GetBool("OpenStdin") {
		v.Set("stdin", "1")
		in = cli.in
	}

	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if *proxy && !tty {
		sigc := cli.forwardAllSignals(cmd.Arg(0))
		defer signal.StopCatch(sigc)
	}

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), tty, in, cli.out, cli.err, nil, nil); err != nil {
		return err
	}

	_, status, err := getExitCode(cli, cmd.Arg(0))
	if err != nil {
		return err
	}
	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}

	return nil
}
