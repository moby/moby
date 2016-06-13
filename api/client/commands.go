package client

// Command returns a cli command handler if one exists
func (cli *DockerCli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"cp":      cli.CmdCp,
		"exec":    cli.CmdExec,
		"info":    cli.CmdInfo,
		"inspect": cli.CmdInspect,
		"ps":      cli.CmdPs,
		"update":  cli.CmdUpdate,
	}[name]
}
