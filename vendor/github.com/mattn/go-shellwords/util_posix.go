// +build !windows,go1.6

package shellwords

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

func shellRun(line, dir string) (string, error) {
	shell := os.Getenv("SHELL")
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
