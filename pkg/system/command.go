package system

import (
	"os/exec"
	"syscall"
)

func GetExitCode(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
}
