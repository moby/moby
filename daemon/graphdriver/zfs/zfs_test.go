package zfs

import (
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"testing"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestZFSSetup and TestZFSTeardown
func TestZFSSetup(t *testing.T) {
	graphtest.GetDriver(t, "zfs")
}

func TestZFSCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "zfs")
}

func TestZFSCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "zfs")
}

func TestZFSCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "zfs")
}

func TestZFSTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
