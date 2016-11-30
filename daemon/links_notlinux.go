// +build !linux

package daemon

import "github.com/docker/docker/container"

// sqliteMigration performs the link graph DB migration. No-op on platforms other than Linux
func (daemon *Daemon) sqliteMigration(_ map[string]*container.Container) error {
	return nil
}
