package client

// Command returns a cli command handler if one exists
func (cli *DockerCli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"commit":  cli.CmdCommit,
		"cp":      cli.CmdCp,
		"exec":    cli.CmdExec,
		"info":    cli.CmdInfo,
		"inspect": cli.CmdInspect,
		"login":   cli.CmdLogin,
		"logout":  cli.CmdLogout,
		"ps":      cli.CmdPs,
		"push":    cli.CmdPush,
		"update":  cli.CmdUpdate,
	}[name]
}
