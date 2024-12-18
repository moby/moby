//go:build !linux

package reexec

import (
	"os/exec"
)

func command(args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: Self(),
		Args: args,
	}
}
