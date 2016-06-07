package client

// Command returns a cli command handler if one exists
func (cli *DockerCli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"build":   cli.CmdBuild,
		"commit":  cli.CmdCommit,
		"cp":      cli.CmdCp,
		"events":  cli.CmdEvents,
		"exec":    cli.CmdExec,
		"images":  cli.CmdImages,
		"import":  cli.CmdImport,
		"info":    cli.CmdInfo,
		"inspect": cli.CmdInspect,
		"kill":    cli.CmdKill,
		"load":    cli.CmdLoad,
		"login":   cli.CmdLogin,
		"logout":  cli.CmdLogout,
		"ps":      cli.CmdPs,
		"pull":    cli.CmdPull,
		"push":    cli.CmdPush,
		"restart": cli.CmdRestart,
		"rm":      cli.CmdRm,
		"save":    cli.CmdSave,
		"stats":   cli.CmdStats,
		"tag":     cli.CmdTag,
		"update":  cli.CmdUpdate,
		"version": cli.CmdVersion,
	}[name]
}
