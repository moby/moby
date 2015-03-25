package client

import (
	"fmt"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// 'docker wait': block until a container stops
func (cli *DockerCli) CmdWait(args ...string) error {
	cmd := cli.Subcmd("wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.", true)
	cmd.Require(flag.Min, 1)

	utils.ParseFlags(cmd, args, true)

	var encounteredError error
	for _, name := range cmd.Args() {
		status, err := waitForExit(cli, name)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to wait one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%d\n", status)
		}
	}
	return encounteredError
}
