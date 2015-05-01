package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/docker/api/types"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdLogs fetches the logs of a given container.
//
// docker logs [OPTIONS] CONTAINER
func (cli *DockerCli) CmdLogs(args ...string) error {
	var (
		cmd    = cli.Subcmd("logs", "CONTAINER", "Fetch the logs of a container", true)
		follow = cmd.Bool([]string{"f", "-follow"}, false, "Follow log output")
		times  = cmd.Bool([]string{"t", "-timestamps"}, false, "Show timestamps")
		tail   = cmd.String([]string{"-tail"}, "all", "Number of lines to show from the end of the logs")
	)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	name := cmd.Arg(0)

	stream, _, err := cli.call("GET", "/containers/"+name+"/json", nil, nil)
	if err != nil {
		return err
	}

	var c types.ContainerJSON
	if err := json.NewDecoder(stream).Decode(&c); err != nil {
		return err
	}

	if c.HostConfig.LogConfig.Type != "json-file" {
		return fmt.Errorf("\"logs\" command is supported only for \"json-file\" logging driver")
	}

	v := url.Values{}
	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if *times {
		v.Set("timestamps", "1")
	}

	if *follow {
		v.Set("follow", "1")
	}
	v.Set("tail", *tail)

	sopts := &streamOpts{
		rawTerminal: c.Config.Tty,
		out:         cli.out,
		err:         cli.err,
	}

	return cli.stream("GET", "/containers/"+name+"/logs?"+v.Encode(), sopts)
}
