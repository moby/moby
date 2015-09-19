// +build !windows

package signal

import (
	"syscall"
)

// Signals used in api/client (no windows equivalent, use
// invalid signals so they don't get handled)

const (
	// SIGCHLD is a signal sent to a process when a child process terminates, is interrupted, or resumes after being interrupted.
	SIGCHLD = syscall.SIGCHLD
	// SIGWINCH is a signal sent to a process when its controlling terminal changes its size
	SIGWINCH = syscall.SIGWINCH
	// DefaultStopSignal is the syscall signal used to stop a container in unix systems.
	DefaultStopSignal = "SIGTERM"
)
