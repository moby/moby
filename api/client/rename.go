package client

import "fmt"

func (cli *DockerCli) CmdRename(args ...string) error {
	cmd := cli.Subcmd("rename", "OLD_NAME NEW_NAME", "Rename a container", true)
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}
	oldName := cmd.Arg(0)
	newName := cmd.Arg(1)

	if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/rename?name=%s", oldName, newName), nil, false)); err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		return fmt.Errorf("Error: failed to rename container named %s", oldName)
	}
	return nil
}
