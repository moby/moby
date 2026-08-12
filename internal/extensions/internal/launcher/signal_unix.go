//go:build unix

package launcher

import (
	"os"
	"syscall"
)

func shutdownSignal() os.Signal {
	return syscall.SIGTERM
}
