package client

import (
	"fmt"
	"net/url"
	"strconv"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// CmdRestart restarts one or more running containers.
//
// Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdRestart(args ...string) error {
	cmd := cli.Subcmd("restart", "CONTAINER [CONTAINER...]", "Restart a running container", true)
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "Seconds to wait for stop before killing the container")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	v := url.Values{}
	v.Set("t", strconv.Itoa(*nSeconds))

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/restart?"+v.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to restart one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}
