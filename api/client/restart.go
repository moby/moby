package client

import (
	"fmt"
	"net/url"
	"strconv"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRestart restarts one or more containers.
//
// Usage: docker restart [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdRestart(args ...string) error {
	cmd := Cli.Subcmd("restart", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["restart"].Description, true)
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "Seconds to wait for stop before killing the container")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	v := url.Values{}
	v.Set("t", strconv.Itoa(*nSeconds))

	var errNames []string
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/restart?"+v.Encode(), nil, nil))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			errNames = append(errNames, name)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	if len(errNames) > 0 {
		return fmt.Errorf("Error: failed to restart containers: %v", errNames)
	}
	return nil
}
