package client

// Command returns a cli command handler if one exists
func (cli *DockerCli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"exec":    cli.CmdExec,
		"inspect": cli.CmdInspect,
	}[name]
}
