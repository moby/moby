//go:build linux
// +build linux

package aufs // import "github.com/docker/docker/daemon/graphdriver/aufs"

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/moby/sys/mount"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Unmount the target specified.
func Unmount(target string) error {
	const retries = 5

	// auplink flush
	for i := 0; ; i++ {
		out, err := exec.Command("auplink", target, "flush").CombinedOutput()
		if err == nil {
			break
		}
		rc := 0
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				rc = status.ExitStatus()
			}
		}
		if i >= retries || rc != int(unix.EINVAL) {
			logger.WithError(err).WithField("method", "Unmount").Warnf("auplink flush failed: %s", out)
			break
		}
		// auplink failed to find target in /proc/self/mounts because
		// kernel can't guarantee continuity while reading from it
		// while mounts table is being changed
		logger.Debugf("auplink flush error (retrying %d/%d): %s", i+1, retries, out)
	}

	// unmount
	var err error
	for i := 0; i < retries; i++ {
		err = mount.Unmount(target)
		if err != nil && errors.Is(err, unix.EBUSY) {
			logger.Debugf("aufs unmount %s failed with EBUSY (retrying %d/%d)", target, i+1, retries)
			time.Sleep(100 * time.Millisecond)
			continue // try again
		}
		break
	}

	// either no error occurred, or another error
	return err
}
