// +build !linux,!darwin,!freebsd,!windows

package daemon // import "github.com/docker/docker/daemon"

func (d *Daemon) setupDumpStackTrap(_ string) {
	return
}
