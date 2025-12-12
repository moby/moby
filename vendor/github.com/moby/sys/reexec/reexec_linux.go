package reexec

import (
	"context"
	"os/exec"
	"syscall"
)

func command(args ...string) *exec.Cmd {
	// We try to stay close to exec.Command's behavior, but after
	// constructing the cmd, we remove "Self()" from cmd.Args, which
	// is prepended by exec.Command.
	cmd := exec.Command(Self(), args...)
	cmd.Args = cmd.Args[1:]
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	return cmd
}

func commandContext(ctx context.Context, args ...string) *exec.Cmd {
	// We try to stay close to exec.Command's behavior, but after
	// constructing the cmd, we remove "Self()" from cmd.Args, which
	// is prepended by exec.Command.
	cmd := exec.CommandContext(ctx, Self(), args...)
	cmd.Args = cmd.Args[1:]
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	return cmd
}
