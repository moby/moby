// +build linux

package btrfs // import "github.com/docker/docker/daemon/graphdriver/btrfs"

import (
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/graphtest"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestBtrfsSetup and TestBtrfsTeardown
func TestBtrfsSetup(t *testing.T) {
	graphtest.GetDriver(t, "btrfs")
}

func TestBtrfsCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "btrfs")
}

func TestBtrfsCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "btrfs")
}

func TestBtrfsCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "btrfs")
}

func TestBtrfsSubvolDelete(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	if err := d.CreateReadWrite("test", "", nil); err != nil {
		t.Fatal(err)
	}
	defer graphtest.PutDriver(t)

	dirFS, err := d.Get("test", "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Put("test")

	dir := dirFS.Path()

	if err := subvolSnapshot("", dir, "subvoltest"); err != nil {
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

func TestBtrfsSubvolRO(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	x := d.(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	subdir := x.subvolumesDirID("0subvol")
	snapdir := x.subvolumesDirID("1snap")

	if err := d.Create("0subvol", "", nil); err != nil {
		t.Fatal(err)
	}
	// Try write into RO subvolume
	if err := ioutil.WriteFile(path.Join(subdir, "testfile0"), []byte("test"), 0700); err.(*os.PathError).Err != syscall.EROFS {
		t.Fatal(err)
	}

	if err := d.Create("1snap", "0subvol", nil); err != nil {
		t.Fatal(err)
	}
	// Try write into RO snapshot
	if err := ioutil.WriteFile(path.Join(subdir, "testfile1"), []byte("test"), 0700); err.(*os.PathError).Err != syscall.EROFS {
		t.Fatal(err)
	}

	if _, err := d.Get("1snap", ""); err != nil {
		t.Fatal(err)
	}
	// Write into RW snapshot
	filepath := path.Join(snapdir, "testfile2")
	if err := ioutil.WriteFile(filepath, []byte("test"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := d.Put("1snap"); err != nil {
		t.Fatal(err)
	}
	// Try delete from RO snapshot
	if err := os.Remove(filepath); err.(*os.PathError).Err != syscall.EROFS {
		t.Fatal(err)
	}

	if err := d.Remove("1snap"); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Get("0subvol", ""); err != nil {
		t.Fatal(err)
	}
	// Write into RW subvolume
	filepath = path.Join(subdir, "testfile3")
	if err := ioutil.WriteFile(filepath, []byte("test"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := d.Put("0subvol"); err != nil {
		t.Fatal(err)
	}
	// Try delete from RO subvolume
	if err := os.Remove(filepath); err.(*os.PathError).Err != syscall.EROFS {
		t.Fatal(err)
	}

	if err := d.Remove("0subvol"); err != nil {
		t.Fatal(err)
	}
}

func TestBtrfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
