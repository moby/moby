// +build !windows

package notary

import (
	"os"
	"syscall"
)

// NotarySupportedSignals contains the signals we would like to capture:
// - SIGUSR1, indicates a increment of the log level.
// - SIGUSR2, indicates a decrement of the log level.
var NotarySupportedSignals = []os.Signal{
	syscall.SIGUSR1,
	syscall.SIGUSR2,
}
