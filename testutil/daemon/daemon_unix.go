// +build !windows

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

func cleanupNetworkNamespace(t testing.TB, d *Daemon) {
	t.Helper()
	// Cleanup network namespaces in the exec root of this
	// daemon because this exec root is specific to this
	// daemon instance and has no chance of getting
	// cleaned up when a new daemon is instantiated with a
	// new exec root.
	netnsPath := filepath.Join(d.execRoot, "netns")
	filepath.Walk(netnsPath, func(path string, info os.FileInfo, err error) error {
		if err := unix.Unmount(path, unix.MNT_DETACH); err != nil && err != unix.EINVAL && err != unix.ENOENT {
			t.Logf("[%s] unmount of %s failed: %v", d.id, path, err)
		}
		os.Remove(path)
		return nil
	})
}

// CgroupNamespace returns the cgroup namespace the daemon is running in
func (d *Daemon) CgroupNamespace(t testing.TB) string {
	link, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/cgroup", d.Pid()))
	assert.NilError(t, err)

	return strings.TrimSpace(link)
}

// SignalDaemonDump sends a signal to the daemon to write a dump file
func SignalDaemonDump(pid int) {
	unix.Kill(pid, unix.SIGQUIT)
}

func signalDaemonReload(pid int) error {
	return unix.Kill(pid, unix.SIGHUP)
}
