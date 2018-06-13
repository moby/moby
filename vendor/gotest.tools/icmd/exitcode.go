package icmd

import (
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
)

// getExitCode returns the ExitStatus of a process from the error returned by
// exec.Run(). If the exit status could not be parsed an error is returned.
func getExitCode(err error) (int, error) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if procExit, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return procExit.ExitStatus(), nil
		}
	}
	return 0, errors.Wrap(err, "failed to get exit code")
}

func processExitCode(err error) (exitCode int) {
	if err == nil {
		return 0
	}
	exitCode, exiterr := getExitCode(err)
	if exiterr != nil {
		// TODO: Fix this so we check the error's text.
		// we've failed to retrieve exit code, so we set it to 127
		return 127
	}
	return exitCode
}
