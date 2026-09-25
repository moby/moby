//go:build !linux

package daemon

import "github.com/moby/moby/v2/daemon/config"

func (daemon *Daemon) loadDefaultAppArmorProfileIfMissing() error {
	return nil
}

// DefaultApparmorProfile returns an empty string.
func DefaultApparmorProfile() string {
	return ""
}

func (daemon *Daemon) installDefaultAppArmorProfile() error {
	return nil
}

func (daemon *Daemon) setupAppArmorProfile(*config.Config) error {
	return nil
}
