// +build !windows

package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/volume/store"
)

// VolumeRm removes the volume with the given name.
// If the volume is referenced by a container it is not removed
// This is called directly from the remote API
func (daemon *Daemon) VolumeRm(name string) error {
	v, err := daemon.volumes.Get(name)
	if err != nil {
		return err
	}
	if err := daemon.volumes.Remove(v); err != nil {
		if strings.Contains(err.Error(), store.ErrVolumeInUse.Error()) {
			return fmt.Errorf("Conflict: %s", err)
		}
		return fmt.Errorf("Error while removing volume %s: %v", name, err)
	}
	return nil
}
