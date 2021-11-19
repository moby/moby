//go:build !windows
// +build !windows

package signal

import (
	"syscall"
)

// Signals used in cli/command (no windows equivalent, use
// invalid signals so they don't get handled)

const (
	// SIGCHLD is a signal sent to a process when a child process terminates, is interrupted, or resumes after being interrupted.
	SIGCHLD = syscall.SIGCHLD
	// SIGWINCH is a signal sent to a process when its controlling terminal changes its size
	SIGWINCH = syscall.SIGWINCH
	// SIGPIPE is a signal sent to a process when a pipe is written to before the other end is open for reading
	SIGPIPE = syscall.SIGPIPE
)
