package client

// Command returns a cli command handler if one exists
func (cli *DockerCli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"commit":  cli.CmdCommit,
		"cp":      cli.CmdCp,
		"exec":    cli.CmdExec,
		"info":    cli.CmdInfo,
		"inspect": cli.CmdInspect,
		"ps":      cli.CmdPs,
		"pull":    cli.CmdPull,
		"push":    cli.CmdPush,
		"update":  cli.CmdUpdate,
	}[name]
}
