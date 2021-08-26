//go:build !linux && !darwin && !freebsd && !windows
// +build !linux,!darwin,!freebsd,!windows

package daemon // import "github.com/docker/docker/daemon"

func (daemon *Daemon) setupDumpStackTrap(_ string) {
	return
}
