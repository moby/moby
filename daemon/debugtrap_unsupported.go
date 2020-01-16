// +build !linux,!darwin,!freebsd,!windows

package daemon // import "github.com/moby/moby/daemon"

func (d *Daemon) setupDumpStackTrap(_ string) {
	return
}
