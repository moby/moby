package client

// Command returns a cli command handler if one exists
func (cli *DockerCli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"attach":  cli.CmdAttach,
		"build":   cli.CmdBuild,
		"commit":  cli.CmdCommit,
		"cp":      cli.CmdCp,
		"events":  cli.CmdEvents,
		"exec":    cli.CmdExec,
		"history": cli.CmdHistory,
		"images":  cli.CmdImages,
		"import":  cli.CmdImport,
		"info":    cli.CmdInfo,
		"inspect": cli.CmdInspect,
		"kill":    cli.CmdKill,
		"load":    cli.CmdLoad,
		"login":   cli.CmdLogin,
		"logout":  cli.CmdLogout,
		"pause":   cli.CmdPause,
		"ps":      cli.CmdPs,
		"pull":    cli.CmdPull,
		"push":    cli.CmdPush,
		"rename":  cli.CmdRename,
		"restart": cli.CmdRestart,
		"rm":      cli.CmdRm,
		"save":    cli.CmdSave,
		"stats":   cli.CmdStats,
		"tag":     cli.CmdTag,
		"top":     cli.CmdTop,
		"update":  cli.CmdUpdate,
		"version": cli.CmdVersion,
	}[name]
}
