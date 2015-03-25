package client

import (
	"fmt"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

func (cli *DockerCli) CmdUnpause(args ...string) error {
	cmd := cli.Subcmd("unpause", "CONTAINER [CONTAINER...]", "Unpause all processes within a container", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, false)

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/unpause", name), nil, false)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to unpause container named %s", name)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}
