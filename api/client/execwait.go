package client

import (
	"fmt"
)

// 'docker execwait': block until an exec stops
func (cli *DockerCli) CmdExecwait(args ...string) error {
	cmd := cli.Subcmd("execwait", "execID [execID...]",
		"Block until an exec stops, then print its exit code.", true)
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	var encounteredError error
	for _, name := range cmd.Args() {
		status, err := waitForExecExit(cli, name)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to wait " +
				"one or more execs")
		} else {
			fmt.Fprintf(cli.out, "%d\n", status)
		}
	}
	return encounteredError
}
