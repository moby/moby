package daemon

import (
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// validateBindDaemonRoot ensures that if a given mountpoint's source is within
// the daemon root path, that the propagation is setup to prevent a container
// from holding private refereneces to a mount within the daemon root, which
// can cause issues when the daemon attempts to remove the mountpoint.
func (daemon *Daemon) validateBindDaemonRoot(m mount.Mount) (bool, error) {
	if m.Type != mount.TypeBind {
		return false, nil
	}

	// check if the source is within the daemon root, or if the daemon root is within the source
	if !strings.HasPrefix(m.Source, daemon.root) && !strings.HasPrefix(daemon.root, m.Source) {
		return false, nil
	}

	if m.BindOptions == nil {
		return true, nil
	}

	switch m.BindOptions.Propagation {
	case mount.PropagationRSlave, mount.PropagationRShared, "":
		return m.BindOptions.Propagation == "", nil
	default:
	}

	return false, errdefs.InvalidParameter(errors.Errorf(`invalid mount config: must use either propagation mode "rslave" or "rshared" when mount source is within the daemon root, daemon root: %q, bind mount source: %q, propagation: %q`, daemon.root, m.Source, m.BindOptions.Propagation))
}
