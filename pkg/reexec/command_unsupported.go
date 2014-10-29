// +build !linux

package reexec

import (
	"os/exec"
)

func Command(args ...string) *exec.Cmd {
	return nil
}
