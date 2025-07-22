//go:build !linux && !darwin && !freebsd && !windows

package daemon

func (daemon *Daemon) setupDumpStackTrap(_ string) {
	return
}
