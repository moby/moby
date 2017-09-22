// +build !linux

package runc

import (
	"context"
	"os/exec"
)

func (r *Runc) command(context context.Context, args ...string) *exec.Cmd {
	command := r.Command
	if command == "" {
		command = DefaultCommand
	}
	return exec.CommandContext(context, command, append(r.args(), args...)...)
}
