package client

import (
	"fmt"
	"strings"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRename renames a container.
//
// Usage: docker rename OLD_NAME NEW_NAME
func (cli *DockerCli) CmdRename(args ...string) error {
	cmd := Cli.Subcmd("rename", []string{"OLD_NAME NEW_NAME"}, Cli.DockerCommands["rename"].Description, true)
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	oldName := strings.TrimSpace(cmd.Arg(0))
	newName := strings.TrimSpace(cmd.Arg(1))

	if oldName == "" || newName == "" {
		return fmt.Errorf("Error: Neither old nor new names may be empty")
	}

	if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/rename?name=%s", oldName, newName), nil, nil)); err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		return fmt.Errorf("Error: failed to rename container named %s", oldName)
	}
	return nil
}
