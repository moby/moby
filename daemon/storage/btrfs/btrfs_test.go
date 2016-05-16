// +build linux

package btrfs

import (
	"os"
	"path"
	"testing"

	"github.com/docker/docker/daemon/storage/test"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestBtrfsSetup and TestBtrfsTeardown
func TestBtrfsSetup(t *testing.T) {
	test.GetDriver(t, "btrfs")
}

func TestBtrfsCreateEmpty(t *testing.T) {
	test.DriverTestCreateEmpty(t, "btrfs")
}

func TestBtrfsCreateBase(t *testing.T) {
	test.DriverTestCreateBase(t, "btrfs")
}

func TestBtrfsCreateSnap(t *testing.T) {
	test.DriverTestCreateSnap(t, "btrfs")
}

func TestBtrfsSubvolDelete(t *testing.T) {
	d := test.GetDriver(t, "btrfs")
	if err := d.CreateReadWrite("test", "", "", nil); err != nil {
		t.Fatal(err)
	}
	defer test.PutDriver(t)

	dir, err := d.Get("test", "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Put("test")

	if err := subvolCreate(dir, "subvoltest"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(dir, "subvoltest")); err != nil {
		t.Fatal(err)
	}

	if err := d.Remove("test"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(dir, "subvoltest")); !os.IsNotExist(err) {
		t.Fatalf("expected not exist error on nested subvol, got: %v", err)
	}
}

func TestBtrfsTeardown(t *testing.T) {
	test.PutDriver(t)
}
