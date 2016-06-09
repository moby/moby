// +build linux

package overlay

import (
	"testing"

	"github.com/docker/docker/daemon/storage/test"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestOverlaySetup and TestOverlayTeardown
func TestOverlaySetup(t *testing.T) {
	test.GetDriver(t, "overlay")
}

func TestOverlayCreateEmpty(t *testing.T) {
	test.DriverTestCreateEmpty(t, "overlay")
}

func TestOverlayCreateBase(t *testing.T) {
	test.DriverTestCreateBase(t, "overlay")
}

func TestOverlayCreateSnap(t *testing.T) {
	test.DriverTestCreateSnap(t, "overlay")
}

func TestOverlayTeardown(t *testing.T) {
	test.PutDriver(t)
}
