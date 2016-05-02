// +build !daemon

package main

import (
	"fmt"
	"runtime"
	"strings"
)

// CmdDaemon reports on an error on windows, because there is no exec
func (p DaemonProxy) CmdDaemon(args ...string) error {
	return fmt.Errorf(
		"`docker daemon` is not supported on %s. Please run `dockerd` directly",
		strings.Title(runtime.GOOS))
}
