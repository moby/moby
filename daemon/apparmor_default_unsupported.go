// +build !linux

package daemon // import "github.com/docker/docker/daemon"

func (daemon *Daemon) installDefaultAppArmorProfile() error {
	return nil
}

func (daemon *Daemon) ensureDefaultAppArmorProfile() error {
	return nil
}
