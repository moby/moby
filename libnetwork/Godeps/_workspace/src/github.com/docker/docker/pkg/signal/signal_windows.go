// +build windows

package signal

import (
	"syscall"
)

// Signals used in api/client (no windows equivalent, use
// invalid signals so they don't get handled)
const (
	SIGCHLD  = syscall.Signal(0xff)
	SIGWINCH = syscall.Signal(0xff)
	// DefaultStopSignal is the syscall signal used to stop a container in windows systems.
	DefaultStopSignal = "15"
)
