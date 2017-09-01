// +build !solaris

package osutils

import (
	"syscall"
)

// SetPDeathSig sets the parent death signal to SIGKILL so that if the
// shim dies the container process also dies.
func SetPDeathSig() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
}
