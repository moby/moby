package icmd

import (
	"errors"

	exec "golang.org/x/sys/execabs"
)

func processExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ProcessState == nil {
			return 0
		}
		if code := exitErr.ProcessState.ExitCode(); code != -1 {
			return code
		}
	}
	return 127
}
