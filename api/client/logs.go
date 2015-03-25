package client

import (
	"fmt"
	"net/url"

	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
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

	utils.ParseFlags(cmd, args, true)

	name := cmd.Arg(0)

	stream, _, err := cli.call("GET", "/containers/"+name+"/json", nil, false)
	if err != nil {
		return err
	}

	env := engine.Env{}
	if err := env.Decode(stream); err != nil {
		return err
	}

	if env.GetSubEnv("HostConfig").GetSubEnv("LogConfig").Get("Type") != "json-file" {
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

	return cli.streamHelper("GET", "/containers/"+name+"/logs?"+v.Encode(), env.GetSubEnv("Config").GetBool("Tty"), nil, cli.out, cli.err, nil)
}
