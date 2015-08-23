// +build !windows

package signal

import (
	"syscall"
)

// Signals used in api/client (no windows equivalent, use
// invalid signals so they don't get handled)

// SIGCHLD is a signal sent to a process when a child process terminates, is interrupted, or resumes after being interrupted.
const SIGCHLD = syscall.SIGCHLD

// SIGWINCH is a signal sent to a process when its controlling terminal changes its size
const SIGWINCH = syscall.SIGWINCH
