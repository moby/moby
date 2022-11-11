package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"github.com/docker/docker/internal/mounttree"
	"github.com/docker/docker/internal/unshare"
	"github.com/moby/sys/mount"
	"golang.org/x/sys/unix"
)

// goInChroot starts fn in a goroutine where the root directory, current working
// directory and umask are unshared from other goroutines and the root directory
// has been changed to path. These changes are only visible to the goroutine in
// which fn is executed. Any other goroutines, including ones started from fn,
// will see the same root directory and file system attributes as the rest of
// the process.
func goInChroot(path string, fn func()) error {
	return unshare.Go(
		unix.CLONE_FS|unix.CLONE_NEWNS,
		func() error {
			// Make everything in new ns slave.
			// Don't use `private` here as this could race where the mountns gets a
			//   reference to a mount and an unmount from the host does not propagate,
			//   which could potentially cause transient errors for other operations,
			//   even though this should be relatively small window here `slave` should
			//   not cause any problems.
			if err := mount.MakeRSlave("/"); err != nil {
				return err
			}

			return mounttree.SwitchRoot(path)
		},
		fn,
	)
}
