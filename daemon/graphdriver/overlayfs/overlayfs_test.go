package overlayfs

import (
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"testing"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestOverlayfsSetup and TestOverlayfsTeardown
func TestOverlayfsSetup(t *testing.T) {
	graphtest.GetDriver(t, "overlayfs")
}

func TestOverlayfsCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "overlayfs")
}

func TestOverlayfsCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "overlayfs")
}

func TestOverlayfsCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "overlayfs")
}

func TestOverlayfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
