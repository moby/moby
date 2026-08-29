//go:build !unix && !windows

package launcher

import "os"

func shutdownSignal() os.Signal {
	return os.Interrupt
}
