package client

import (
	"fmt"

	flag "github.com/docker/docker/pkg/mflag"
)

// CmdPause pauses all processes within one or more containers.
//
// Usage: docker pause CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdPause(args ...string) error {
	cmd := cli.Subcmd("pause", "CONTAINER [CONTAINER...]", "Pause all processes within a container", true)
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, false)

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/pause", name), nil, nil)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to pause container named %s", name)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}
