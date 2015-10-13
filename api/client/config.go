package client

import (
	"fmt"
	"net/url"

	Cli "github.com/docker/docker/cli"
)

// CmdConfig config the docker daemon without restarting docker daemon.
//
// Usage: docker config
func (cli *DockerCli) CmdConfig(args ...string) error {
	cmd := Cli.Subcmd("config", []string{"KEY=VALUE"}, Cli.DockerCommands["config"].Description, true)

	flAdd := cmd.Bool([]string{"a", "-add"}, false, "Add config")
	flRemove := cmd.Bool([]string{"r", "-remove"}, false, "Remove  config")
	flModify := cmd.Bool([]string{"m", "-modify"}, false, "Modify  config")

	cmd.ParseFlags(args, true)
	if *flAdd && *flRemove && *flModify {
		return fmt.Errorf("Conflict: only a method is allowed")
	}
	if !*flAdd && !*flRemove && !*flModify {
		return fmt.Errorf("You should specify a method")
	}

	var method string
	if *flAdd {
		method = "Add"
	}
	if *flRemove {
		method = "Remove"
	}
	if *flModify {
		method = "Modify"
	}

	v := url.Values{}
	v.Set("method", method)
	v.Set("config", cmd.Arg(0))

	if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/config?%s", v.Encode()), nil, nil)); err != nil {
		return fmt.Errorf("Error: failed to '%s' config '%s': %v.", method, cmd.Arg(0), err)
	}
	return nil
}
