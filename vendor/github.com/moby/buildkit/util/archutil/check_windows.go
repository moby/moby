//go:build windows
// +build windows

package archutil

import (
	"errors"
	"os/exec"
)

func withChroot(cmd *exec.Cmd, dir string) {
}

func check(arch, bin string) (string, error) {
	return "", errors.New("binfmt is not supported on Windows")
}
