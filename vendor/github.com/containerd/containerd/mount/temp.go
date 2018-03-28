package mount

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
)

var tempMountLocation = os.TempDir()

// WithTempMount mounts the provided mounts to a temp dir, and pass the temp dir to f.
// The mounts are valid during the call to the f.
// Finally we will unmount and remove the temp dir regardless of the result of f.
func WithTempMount(ctx context.Context, mounts []Mount, f func(root string) error) (err error) {
	root, uerr := ioutil.TempDir(tempMountLocation, "containerd-mount")
	if uerr != nil {
		return errors.Wrapf(uerr, "failed to create temp dir")
	}
	// We use Remove here instead of RemoveAll.
	// The RemoveAll will delete the temp dir and all children it contains.
	// When the Unmount fails, RemoveAll will incorrectly delete data from
	// the mounted dir. However, if we use Remove, even though we won't
	// successfully delete the temp dir and it may leak, we won't loss data
	// from the mounted dir.
	// For details, please refer to #1868 #1785.
	defer func() {
		if uerr = os.Remove(root); uerr != nil {
			log.G(ctx).WithError(uerr).WithField("dir", root).Errorf("failed to remove mount temp dir")
		}
	}()

	// We should do defer first, if not we will not do Unmount when only a part of Mounts are failed.
	defer func() {
		if uerr = UnmountAll(root, 0); uerr != nil {
			uerr = errors.Wrapf(uerr, "failed to unmount %s", root)
			if err == nil {
				err = uerr
			} else {
				err = errors.Wrap(err, uerr.Error())
			}
		}
	}()
	if uerr = All(mounts, root); uerr != nil {
		return errors.Wrapf(uerr, "failed to mount %s", root)
	}
	return errors.Wrapf(f(root), "mount callback failed on %s", root)
}
