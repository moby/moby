package client

import (
	"fmt"
	"net/url"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

func (cli *DockerCli) CmdRm(args ...string) error {
	cmd := cli.Subcmd("rm", "CONTAINER [CONTAINER...]", "Remove one or more containers", true)
	v := cmd.Bool([]string{"v", "-volumes"}, false, "Remove the volumes associated with the container")
	link := cmd.Bool([]string{"l", "#link", "-link"}, false, "Remove the specified link")
	force := cmd.Bool([]string{"f", "-force"}, false, "Force the removal of a running container (uses SIGKILL)")
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	val := url.Values{}
	if *v {
		val.Set("v", "1")
	}
	if *link {
		val.Set("link", "1")
	}

	if *force {
		val.Set("force", "1")
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("DELETE", "/containers/"+name+"?"+val.Encode(), nil, nil))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}
