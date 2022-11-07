//go:build linux || freebsd
// +build linux freebsd

package testutils

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/docker/docker/libnetwork/ns"
	"github.com/pkg/errors"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// OSContext is a handle to a test OS context.
type OSContext struct {
	origNS, newNS netns.NsHandle

	tid    int
	caller string // The file:line where SetupTestOSContextEx was called, for interpolating into error messages.
}

// SetupTestOSContextEx joins the current goroutine to a new network namespace.
//
// Compared to [SetupTestOSContext], this function allows goroutines to be
// spawned which are associated with the same OS context via the returned
// OSContext value.
//
// Example usage:
//
//	c := SetupTestOSContext(t)
//	defer c.Cleanup(t)
func SetupTestOSContextEx(t *testing.T) *OSContext {
	runtime.LockOSThread()
	origNS, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("Failed to open initial netns: %v", err)
	}

	c := OSContext{
		tid:    unix.Gettid(),
		origNS: origNS,
	}
	c.newNS, err = netns.New()
	if err != nil {
		// netns.New() is not atomic: it could have encountered an error
		// after unsharing the current thread's network namespace.
		c.restore(t)
		t.Fatalf("Failed to enter netns: %v", err)
	}

	// Since we are switching to a new test namespace make
	// sure to re-initialize initNs context
	ns.Init()

	nl := ns.NlHandle()
	lo, err := nl.LinkByName("lo")
	if err != nil {
		c.restore(t)
		t.Fatalf("Failed to get handle to loopback interface 'lo' in new netns: %v", err)
	}
	if err := nl.LinkSetUp(lo); err != nil {
		c.restore(t)
		t.Fatalf("Failed to enable loopback interface in new netns: %v", err)
	}

	_, file, line, ok := runtime.Caller(0)
	if ok {
		c.caller = file + ":" + strconv.Itoa(line)
	}

	return &c
}

// Cleanup tears down the OS context. It must be called from the same goroutine
// as the [SetupTestOSContextEx] call which returned c.
//
// Explicit cleanup is required as (*testing.T).Cleanup() makes no guarantees
// about which goroutine the cleanup functions are invoked on.
func (c *OSContext) Cleanup(t *testing.T) {
	t.Helper()
	if unix.Gettid() != c.tid {
		t.Fatalf("c.Cleanup() must be called from the same goroutine as SetupTestOSContextEx() (%s)", c.caller)
	}
	if err := c.newNS.Close(); err != nil {
		t.Logf("Warning: netns closing failed (%v)", err)
	}
	c.restore(t)
	ns.Init()
}

func (c *OSContext) restore(t *testing.T) {
	t.Helper()
	if err := netns.Set(c.origNS); err != nil {
		t.Logf("Warning: failed to restore thread netns (%v)", err)
	} else {
		runtime.UnlockOSThread()
	}

	if err := c.origNS.Close(); err != nil {
		t.Logf("Warning: netns closing failed (%v)", err)
	}
}

// Set sets the OS context of the calling goroutine to c and returns a teardown
// function to restore the calling goroutine's OS context and release resources.
// The teardown function accepts an optional Logger argument.
//
// This is a lower-level interface which is less ergonomic than c.Go() but more
// composable with other goroutine-spawning utilities such as [sync.WaitGroup]
// or [golang.org/x/sync/errgroup.Group].
//
// Example usage:
//
//	func TestFoo(t *testing.T) {
//		osctx := testutils.SetupTestOSContextEx(t)
//		defer osctx.Cleanup(t)
//		var eg errgroup.Group
//		eg.Go(func() error {
//			teardown, err := osctx.Set()
//			if err != nil {
//				return err
//			}
//			defer teardown(t)
//			// ...
//		})
//		if err := eg.Wait(); err != nil {
//			t.Fatalf("%+v", err)
//		}
//	}
func (c *OSContext) Set() (func(Logger), error) {
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		return nil, errors.Wrap(err, "failed to open initial netns for goroutine")
	}
	if err := errors.WithStack(netns.Set(c.newNS)); err != nil {
		runtime.UnlockOSThread()
		return nil, errors.Wrap(err, "failed to set goroutine network namespace")
	}

	tid := unix.Gettid()
	_, file, line, callerOK := runtime.Caller(0)

	return func(log Logger) {
		if unix.Gettid() != tid {
			msg := "teardown function must be called from the same goroutine as c.Set()"
			if callerOK {
				msg += fmt.Sprintf(" (%s:%d)", file, line)
			}
			panic(msg)
		}

		if err := netns.Set(orig); err != nil && log != nil {
			log.Logf("Warning: failed to restore goroutine thread netns (%v)", err)
		} else {
			runtime.UnlockOSThread()
		}

		if err := orig.Close(); err != nil && log != nil {
			log.Logf("Warning: netns closing failed (%v)", err)
		}
	}, nil
}
