// +build !linux

package daemon

import (
	"gotest.tools/assert"
	"gotest.tools/icmd"
)

func (d *Daemon) ensureNetworking(t assert.TestingT) {
}

func (d *Daemon) configureCmd() error {
	return nil
}

// Exec generates a command which, when executed, will run in the daemon's context
func (d *Daemon) Exec(command string, args ...string) icmd.Cmd {
	return icmd.Command(command, args...)
}
