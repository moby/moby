//go:build linux
// +build linux

package btrfs // import "github.com/docker/docker/daemon/graphdriver/btrfs"

import (
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"

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

	dir, err := d.Get("test", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := subvolSnapshot("", dir, "subvoltest1"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(dir, "subvoltest1")); err != nil {
		t.Fatal(err)
	}

	idir := path.Join(dir, "intermediate")
	if err := os.Mkdir(idir, 0777); err != nil {
		t.Fatalf("Failed to create intermediate dir %s: %v", idir, err)
	}

	if err := subvolSnapshot("", idir, "subvoltest2"); err != nil {
		t.Fatal(err)
	}

	if err := d.Put("test"); err != nil {
		t.Fatal(err)
	}

	if err := d.Remove("test"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected not exist error on nested subvol, got: %v", err)
	}
}

func TestBtrfsSubvolLongPath(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	wdir, _ := os.Getwd()

	if err := d.Create("rootsubvol", "", nil); err != nil {
		t.Fatalf("Failed to create rootsubvol: %v", err)
	}
	subvoldir, err := d.Get("rootsubvol", "")
	if err != nil {
		t.Fatal(err)
	}

	getMaxFilenameFormPattern := func(pattern string) string {
		name := strings.Repeat(pattern, (syscall.NAME_MAX / len(pattern)))
		return (name + pattern[:(syscall.NAME_MAX-len(name))])
	}

	os.Chdir(subvoldir)
	defer os.Chdir(wdir)

	for i, l := 1, len(subvoldir); l <= syscall.PathMax*2; i++ {
		dfile, err := os.OpenFile("dummyFile", os.O_RDONLY|os.O_CREATE, 0666)
		if err != nil {
			t.Fatalf("Failed to create file at %s: %v", subvoldir, err)
		}
		if err := dfile.Close(); err != nil {
			t.Fatal(err)
		}
		name := getMaxFilenameFormPattern(fmt.Sprintf("LongPathToFirstSubvol_LVL%d_", i))
		if err := os.Mkdir(name, 0777); err != nil {
			t.Fatalf("Failed to create dir %s/%s: %v", subvoldir, name, err)
		}
		if err := os.Chdir(name); err != nil {
			t.Fatal(err)
		}
		l += len(name)
		subvoldir = path.Join(subvoldir, name)
	}

	if err := subvolSnapshot("", subvoldir, "subvolLVL1"); err != nil {
		t.Fatal(err)
	}
	if err := subvolSnapshot("", subvoldir+"/..", "subvolLVL1_0"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("subvolLVL1"); err != nil {
		t.Fatal(err)
	}
	subvoldir = path.Join(subvoldir, "subvolLVL1")

	i := 1
	for l := 0; l < syscall.PathMax; i++ {
		name := getMaxFilenameFormPattern(fmt.Sprintf("LongPathToNestedSub_1_LVL%d_", i))
		if err := os.Mkdir(name, 0777); err != nil {
			t.Fatalf("Failed to create dir %s/%s: %v", subvoldir, name, err)
		}
		if err := os.Chdir(name); err != nil {
			t.Fatal(err)
		}
		l += len(name)
		subvoldir = path.Join(subvoldir, name)
	}

	if err := subvolSnapshot("", subvoldir, "subvolLVL2_1"); err != nil {
		t.Fatal(err)
	}

	for ; i > 1; i-- {
		if err := os.Chdir(".."); err != nil {
			t.Fatal(err)
		}
		subvoldir = path.Dir(subvoldir)
	}

	for i, l := 1, 0; l < syscall.PathMax*2; i++ {
		name := getMaxFilenameFormPattern(fmt.Sprintf("LongPathToNestedSub_2_LVL%d_", i))
		if err := os.Mkdir(name, 0777); err != nil {
			t.Fatalf("Failed to create dir %s/%s: %v", subvoldir, name, err)
		}
		if err := os.Chdir(name); err != nil {
			t.Fatal(err)
		}
		l += len(name)
		subvoldir = path.Join(subvoldir, name)
	}

	if err := subvolSnapshot("", subvoldir, "subvolLVL2_3"); err != nil {
		t.Fatal(err)
	}
	if err := subvolSnapshot("", subvoldir, "subvolLVL2_2"); err != nil {
		t.Fatal(err)
	}

	if err := d.Put("rootsubvol"); err != nil {
		t.Fatal(err)
	}

	if err := d.Remove("rootsubvol"); err != nil {
		t.Fatal(err)
	}
}

func TestBtrfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
