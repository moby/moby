// +build linux

package btrfs // import "github.com/docker/docker/daemon/graphdriver/btrfs"

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
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
	defer graphtest.PutDriver(t)
	if err := d.CreateReadWrite("test", "", nil); err != nil {
		t.Fatal(err)
	}

	dirFS, err := d.Get("test", "")
	if err != nil {
		t.Fatal(err)
	}

	dir := dirFS.Path()

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

func TestBtrfsSubvolLongPath(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	wdir, _ := os.Getwd()

	if err := d.Create("rootsubvol", "", nil); err != nil {
		t.Fatalf("Failed to create rootsubvol: %v", err)
	}
	rootFS, err := d.Get("rootsubvol", "")
	if err != nil {
		t.Fatal(err)
	}

	subvoldir := rootFS.Path()

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
	if err := subvolSetPropRO(subvoldir, false); err != nil {
		t.Fatal(err)
	}

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

func TestBtrfsSubvolSELinux(t *testing.T) {
	pl, fl, err := label.InitLabels([]string{"role:test_r"})
	if err != nil {
		t.Fatalf("Fail to get init labels: %v", err)
	}
	if pl == "" && fl == "" {
		t.Skip("SELinux disabled or \"selinux\" buildtag not set")
	}
	if strings.SplitN(pl, ":", 3)[1] != "test_r" {
		t.Fatalf("test role was not set: %s", pl)
	}

	tstl := strings.SplitN(fl, ":", 3)
	tstl[2] = "test_file_t:s0"
	filetestlabel := strings.Join(tstl, ":")

	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	createOpts := &graphdriver.CreateOpts{MountLabel: filetestlabel}

	if err := d.Create("SEsubvol", "", createOpts); err != nil {
		t.Fatal(err)
	}

	x := d.(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	subvoldir := x.subvolumesDirID("SEsubvol")

	subvollabel, err := selinux.FileLabel(subvoldir)
	if err != nil {
		t.Fatal(err)
	}
	if subvollabel != filetestlabel {
		t.Logf("subvol label %s not match with MountLabel %s", subvollabel, filetestlabel)
		t.Fail()
	}

	if err := d.Remove("SEsubvol"); err != nil {
		t.Fatal(err)
	}
}

func TestBtrfsSetQuota(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	graphtest.DriverTestSetQuota(t, "btrfs", false)

	x := d.(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	subdir := x.subvolumesDir()
	files, err := ioutil.ReadDir(subdir)
	if err != nil {
		t.Fatal(err)
	}
	for _, subvol := range files {
		if err := d.Remove(subvol.Name()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBtrfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
