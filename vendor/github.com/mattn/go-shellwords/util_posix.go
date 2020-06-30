// +build !windows

package shellwords

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

func shellRun(line, dir string) (string, error) {
	var shell string
	if shell = os.Getenv("SHELL"); shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-c", line)
	if dir != "" {
		cmd.Dir = dir
	}
	b, err := cmd.Output()
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			b = eerr.Stderr
		}
		return "", errors.New(err.Error() + ":" + string(b))
	}
	return strings.TrimSpace(string(b)), nil
}
