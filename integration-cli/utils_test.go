package main

import (
	"github.com/docker/docker/pkg/testutil/cmd"
	"os/exec"
)

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if daemonPlatform == "windows" {
		return "c:", `\`
	}
	return "", "/"
}

// TODO: update code to call cmd.RunCmd directly, and remove this function
func runCommandWithOutput(execCmd *exec.Cmd) (string, int, error) {
	result := cmd.RunCmd(transformCmd(execCmd))
	return result.Combined(), result.ExitCode, result.Error
}

// Temporary shim for migrating commands to the new function
func transformCmd(execCmd *exec.Cmd) cmd.Cmd {
	return cmd.Cmd{
		Command: execCmd.Args,
		Env:     execCmd.Env,
		Dir:     execCmd.Dir,
		Stdin:   execCmd.Stdin,
		Stdout:  execCmd.Stdout,
	}
}
