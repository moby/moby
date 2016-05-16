// +build linux

package vfs

import (
	"testing"

	"github.com/docker/docker/daemon/storage/test"

	"github.com/docker/docker/pkg/reexec"
)

func init() {
	reexec.Init()
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestVfsSetup and TestVfsTeardown
func TestVfsSetup(t *testing.T) {
	test.GetDriver(t, "vfs")
}

func TestVfsCreateEmpty(t *testing.T) {
	test.DriverTestCreateEmpty(t, "vfs")
}

func TestVfsCreateBase(t *testing.T) {
	test.DriverTestCreateBase(t, "vfs")
}

func TestVfsCreateSnap(t *testing.T) {
	test.DriverTestCreateSnap(t, "vfs")
}

func TestVfsTeardown(t *testing.T) {
	test.PutDriver(t)
}
