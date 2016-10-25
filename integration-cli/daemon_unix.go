// +build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/go-check/check"
)

func cleanupExecRoot(c *check.C, execRoot string) {
	// Cleanup network namespaces in the exec root of this
	// daemon because this exec root is specific to this
	// daemon instance and has no chance of getting
	// cleaned up when a new daemon is instantiated with a
	// new exec root.
	netnsPath := filepath.Join(execRoot, "netns")
	filepath.Walk(netnsPath, func(path string, info os.FileInfo, err error) error {
		if err := syscall.Unmount(path, syscall.MNT_FORCE); err != nil {
			c.Logf("unmount of %s failed: %v", path, err)
		}
		os.Remove(path)
		return nil
	})
}

func signalDaemonDump(pid int) {
	syscall.Kill(pid, syscall.SIGQUIT)
}

func signalDaemonReload(pid int) error {
	return syscall.Kill(pid, syscall.SIGHUP)
}
