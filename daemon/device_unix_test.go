// +build !windows

package daemon

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/opencontainers/runc/libcontainer/devices"
)

func cleanupTest() {
	ioutilReadDir = ioutil.ReadDir
	usernsRunningInUserNS = userns.RunningInUserNS
}

// Based on test from runc (libcontainer/devices/device_unix_test.go).
func TestHostDevicesIoutilReadDirFailure(t *testing.T) {
	testError := fmt.Errorf("test error: %w", os.ErrPermission)

	// Override ioutil.ReadDir to inject error.
	ioutilReadDir = func(dirname string) ([]os.FileInfo, error) {
		return nil, testError
	}
	// Override userns.RunningInUserNS to ensure not running in user namespace.
	usernsRunningInUserNS = func() bool {
		return false
	}
	defer cleanupTest()

	_, err := HostDevices()
	if !errors.Is(err, testError) {
		t.Fatalf("Unexpected error %v, expected %v", err, testError)
	}
}

// Based on test from runc (libcontainer/devices/device_unix_test.go).
func TestHostDevicesIoutilReadDirFailureIfRunningInUserNS(t *testing.T) {
	testError := fmt.Errorf("test error: %w", os.ErrPermission)

	// Override ioutil.ReadDir to inject error.
	ioutilReadDir = func(dirname string) ([]os.FileInfo, error) {
		return nil, testError
	}
	// Override userns.RunningInUserNS to ensure running in user namespace.
	usernsRunningInUserNS = func() bool {
		return true
	}
	defer cleanupTest()

	_, err := HostDevices()
	if !errors.Is(err, nil) {
		t.Fatalf("Unexpected error %v, expected %v", err, nil)
	}
}

// Based on test from runc (libcontainer/devices/device_unix_test.go).
func TestHostDevicesIoutilReadDirDeepFailure(t *testing.T) {
	testError := fmt.Errorf("test error: %w", os.ErrPermission)
	called := false

	// Override ioutil.ReadDir to inject error after the first call.
	ioutilReadDir = func(dirname string) ([]os.FileInfo, error) {
		if called {
			return nil, testError
		}
		called = true

		// Provoke a second call.
		fi, err := os.Lstat("/tmp")
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		return []os.FileInfo{fi}, nil
	}
	// Override userns.RunningInUserNS to ensure not running in user namespace.
	usernsRunningInUserNS = func() bool {
		return false
	}
	defer cleanupTest()

	_, err := HostDevices()
	if !errors.Is(err, testError) {
		t.Fatalf("Unexpected error %v, expected %v", err, testError)
	}
}

// Based on test from runc (libcontainer/devices/device_unix_test.go).
func TestHostDevicesIoutilReadDirDeepFailureIfRunningInUserNS(t *testing.T) {
	testError := fmt.Errorf("test error: %w", os.ErrPermission)
	called := false

	// Override ioutil.ReadDir to inject error after the first call.
	ioutilReadDir = func(dirname string) ([]os.FileInfo, error) {
		if called {
			return nil, testError
		}
		called = true

		// Provoke a second call.
		fi, err := os.Lstat("/tmp")
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		return []os.FileInfo{fi}, nil
	}
	// Override userns.RunningInUserNS to ensure running in user namespace.
	usernsRunningInUserNS = func() bool {
		return true
	}
	defer cleanupTest()

	_, err := HostDevices()
	if !errors.Is(err, nil) {
		t.Fatalf("Unexpected error %v, expected %v", err, nil)
	}
}

// Based on test from runc (libcontainer/devices/device_unix_test.go).
func TestHostDevicesAllValid(t *testing.T) {
	hostDevices, err := HostDevices()
	if err != nil {
		t.Fatalf("failed to get host devices: %v", err)
	}

	for _, d := range hostDevices {
		// Devices can't have major number 0.
		if d.Major == 0 {
			t.Errorf("device entry %+v has zero major number", d)
		}
		switch d.Type {
		case devices.BlockDevice, devices.CharDevice:
		case devices.FifoDevice:
			t.Logf("fifo devices shouldn't show up from HostDevices")
			fallthrough
		default:
			t.Errorf("device entry %+v has unexpected type %v", d, d.Type)
		}
	}
}
