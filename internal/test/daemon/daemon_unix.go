// +build !windows

package daemon // import "github.com/docker/docker/internal/test/daemon"

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/internal/test"
	"golang.org/x/sys/unix"
)

func cleanupNetworkNamespace(t testingT, execRoot string) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	// Cleanup network namespaces in the exec root of this
	// daemon because this exec root is specific to this
	// daemon instance and has no chance of getting
	// cleaned up when a new daemon is instantiated with a
	// new exec root.
	netnsPath := filepath.Join(execRoot, "netns")
	filepath.Walk(netnsPath, func(path string, info os.FileInfo, err error) error {
		if err := unix.Unmount(path, unix.MNT_FORCE); err != nil {
			t.Logf("unmount of %s failed: %v", path, err)
		}
		os.Remove(path)
		return nil
	})
}

// SignalDaemonDump sends a signal to the daemon to write a dump file
func SignalDaemonDump(pid int) {
	unix.Kill(pid, unix.SIGQUIT)
}

func signalDaemonReload(pid int) error {
	return unix.Kill(pid, unix.SIGHUP)
}
