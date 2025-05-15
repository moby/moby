package daemon // import "github.com/docker/docker/testutil/daemon"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

func cleanupNetworkNamespace(tb testing.TB, d *Daemon) {
	tb.Helper()
	// Cleanup network namespaces in the exec root of this
	// daemon because this exec root is specific to this
	// daemon instance and has no chance of getting
	// cleaned up when a new daemon is instantiated with a
	// new exec root.
	filepath.WalkDir(filepath.Join(d.execRoot, "netns"), func(path string, _ os.DirEntry, _ error) error {
		if err := unix.Unmount(path, unix.MNT_DETACH); err != nil && err != unix.EINVAL && err != unix.ENOENT {
			tb.Logf("[%s] unmount of %s failed: %v", d.id, path, err)
		}
		os.Remove(path)
		return nil
	})
}

// CgroupNamespace returns the cgroup namespace the daemon is running in
func (d *Daemon) CgroupNamespace(tb testing.TB) string {
	link, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/cgroup", d.Pid()))
	assert.NilError(tb, err)

	return strings.TrimSpace(link)
}
