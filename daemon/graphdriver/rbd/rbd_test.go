// +build linux

package rbd

import (
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"testing"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestRbdSetup and TestRbdTeardown
func TestRbdSetup(t *testing.T) {
	graphtest.GetDriver(t, "rbd")
}

func TestRbdCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "rbd")
}

func TestRbdCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "rbd")
}

func TestRbdCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "rbd")
}

func TestRbdTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
