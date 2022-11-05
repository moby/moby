//go:build linux || freebsd
// +build linux freebsd

package testutils

import (
	"runtime"
	"testing"

	"github.com/docker/docker/libnetwork/ns"
	"github.com/vishvananda/netns"
)

// SetupTestOSContext joins a new network namespace, and returns its associated
// teardown function.
//
// Example usage:
//
//	defer SetupTestOSContext(t)()
func SetupTestOSContext(t *testing.T) func() {
	origNS, err := netns.Get()
	if err != nil {
		t.Fatalf("Failed to open initial netns: %v", err)
	}
	restore := func() {
		if err := netns.Set(origNS); err != nil {
			t.Logf("Warning: failed to restore thread netns (%v)", err)
		} else {
			runtime.UnlockOSThread()
		}

		if err := origNS.Close(); err != nil {
			t.Logf("Warning: netns closing failed (%v)", err)
		}
	}

	runtime.LockOSThread()
	newNS, err := netns.New()
	if err != nil {
		// netns.New() is not atomic: it could have encountered an error
		// after unsharing the current thread's network namespace.
		restore()
		t.Fatalf("Failed to enter netns: %v", err)
	}

	// Since we are switching to a new test namespace make
	// sure to re-initialize initNs context
	ns.Init()

	nl := ns.NlHandle()
	lo, err := nl.LinkByName("lo")
	if err != nil {
		restore()
		t.Fatalf("Failed to get handle to loopback interface 'lo' in new netns: %v", err)
	}
	if err := nl.LinkSetUp(lo); err != nil {
		restore()
		t.Fatalf("Failed to enable loopback interface in new netns: %v", err)
	}

	return func() {
		if err := newNS.Close(); err != nil {
			t.Logf("Warning: netns closing failed (%v)", err)
		}
		restore()
		ns.Init()
	}
}
