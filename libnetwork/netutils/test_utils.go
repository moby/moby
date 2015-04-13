package netutils

import (
	"runtime"
	"syscall"
	"testing"
)

// SetupTestNetNS joins a new network namespace, and returns its associated
// teardown function.
//
// Example usage:
//
//     defer SetupTestNetNS(t)()
//
func SetupTestNetNS(t *testing.T) func() {
	runtime.LockOSThread()
	if err := syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
		t.Fatalf("Failed to enter netns: %v", err)
	}

	fd, err := syscall.Open("/proc/self/ns/net", syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal("Failed to open netns file")
	}

	return func() {
		if err := syscall.Close(fd); err != nil {
			t.Logf("Warning: netns closing failed (%v)", err)
		}
		runtime.UnlockOSThread()
	}
}
