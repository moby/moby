package client

import (
	"fmt"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// CmdUnpause unpauses all processes within a container, for one or more containers.
//
// Usage: docker unpause CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdUnpause(args ...string) error {
	cmd := cli.Subcmd("unpause", "CONTAINER [CONTAINER...]", "Unpause all processes within a container", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, false)

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/unpause", name), nil, nil)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to unpause container named %s", name)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}
