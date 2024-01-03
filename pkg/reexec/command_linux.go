package reexec // import "github.com/docker/docker/pkg/reexec"

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// Self returns the path to the current process's binary.
// Returns "/proc/self/exe".
func Self() string {
	return "/proc/self/exe"
}

// Command returns *exec.Cmd which has Path as current binary. Also it setting
// SysProcAttr.Pdeathsig to SIGTERM.
// This will use the in-memory version (/proc/self/exe) of the current binary,
// it is thus safe to delete or replace the on-disk binary (os.Args[0]).
//
// As SysProcAttr.Pdeathsig is set, the signal will be sent to the process when
// the OS thread which created the process dies. It is the caller's
// responsibility to ensure that the creating thread is not terminated
// prematurely. See https://go.dev/issue/27505 for more details.
func Command(args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: Self(),
		Args: args,
		SysProcAttr: &syscall.SysProcAttr{
			Pdeathsig: unix.SIGTERM,
		},
	}
}
