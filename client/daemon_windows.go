package main

import (
	"fmt"
)

// CmdDaemon reports on an error on windows, because there is no exec
func (p DaemonProxy) CmdDaemon(args ...string) error {
	return fmt.Errorf(
		"`docker daemon` does not exist on windows. Please run `dockerd` directly")
}
