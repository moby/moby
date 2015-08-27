// +build linux

package ploop

import (
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"testing"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestPloopSetup and TestPloopTeardown
func TestPloopSetup(t *testing.T) {
	graphtest.GetDriver(t, "ploop")
}

func TestPloopCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "ploop")
}

func TestPloopCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "ploop")
}

func TestPloopCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "ploop")
}

func TestPloopTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
