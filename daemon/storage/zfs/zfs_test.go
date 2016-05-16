// +build linux

package zfs

import (
	"testing"

	"github.com/docker/docker/daemon/storage/test"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestZfsSetup and TestZfsTeardown
func TestZfsSetup(t *testing.T) {
	test.GetDriver(t, "zfs")
}

func TestZfsCreateEmpty(t *testing.T) {
	test.DriverTestCreateEmpty(t, "zfs")
}

func TestZfsCreateBase(t *testing.T) {
	test.DriverTestCreateBase(t, "zfs")
}

func TestZfsCreateSnap(t *testing.T) {
	test.DriverTestCreateSnap(t, "zfs")
}

func TestZfsSetQuota(t *testing.T) {
	graphtest.DriverTestSetQuota(t, "zfs")
}

func TestZfsTeardown(t *testing.T) {
	test.PutDriver(t)
}
