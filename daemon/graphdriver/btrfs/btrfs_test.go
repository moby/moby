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
	"gotest.tools/v3/assert"
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

	assert.NilError(t, d.CreateReadWrite("test", "", nil))

	dirFS, err := d.Get("test", "")
	assert.NilError(t, err)

	dir := dirFS.Path()

	assert.NilError(t, subvolSnapshot("", dir, "subvoltest1"))

	_, err = os.Stat(path.Join(dir, "subvoltest1"))
	assert.NilError(t, err)

	idir := path.Join(dir, "intermediate")
	assert.NilError(t, os.Mkdir(idir, 0777))

	assert.NilError(t, subvolSnapshot("", idir, "subvoltest2"))

	assert.NilError(t, d.Put("test"))

	assert.NilError(t, d.Remove("test"))

	_, err = os.Stat(dir)
	assert.ErrorType(t, err, os.IsNotExist, "expected not exist error on subvol dir")
}

func TestBtrfsSubvolRO(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	x := d.(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	subdir := x.subvolumesDirID("0subvol")
	snapdir := x.subvolumesDirID("1snap")

	assert.NilError(t, d.Create("0subvol", "", nil))
	// Try write into RO subvolume
	err := ioutil.WriteFile(path.Join(subdir, "testfile0"), []byte("test"), 0700)
	assert.Equal(t, err.(*os.PathError).Err, syscall.EROFS)

	assert.NilError(t, d.Create("1snap", "0subvol", nil))
	// Try write into RO snapshot
	err = ioutil.WriteFile(path.Join(subdir, "testfile1"), []byte("test"), 0700)
	assert.Equal(t, err.(*os.PathError).Err, syscall.EROFS)

	_, err = d.Get("1snap", "")
	assert.NilError(t, err)
	// Write into RW snapshot
	filepath := path.Join(snapdir, "testfile2")
	assert.NilError(t, ioutil.WriteFile(filepath, []byte("test"), 0700))

	assert.NilError(t, d.Put("1snap"))
	// Try delete from RO snapshot
	err = os.Remove(filepath)
	assert.Equal(t, err.(*os.PathError).Err, syscall.EROFS)

	assert.NilError(t, d.Remove("1snap"))

	_, err = d.Get("0subvol", "")
	assert.NilError(t, err)
	// Write into RW subvolume
	filepath = path.Join(subdir, "testfile3")
	assert.NilError(t, ioutil.WriteFile(filepath, []byte("test"), 0700))

	assert.NilError(t, d.Put("0subvol"))
	// Try delete from RO subvolume
	err = os.Remove(filepath)
	assert.Equal(t, err.(*os.PathError).Err, syscall.EROFS)

	assert.NilError(t, d.Remove("0subvol"))
}

// Testing various cases where distance from / or between subvolumes more than PATH_MAX
func TestBtrfsSubvolLongPath(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	wdir, _ := os.Getwd()

	assert.NilError(t, d.Create("rootsubvol", "", nil), "Failed to create rootsubvol")
	rootFS, err := d.Get("rootsubvol", "")
	assert.NilError(t, err)

	subvoldir := rootFS.Path()

	getMaxFilenameFormPattern := func(pattern string) string {
		name := strings.Repeat(pattern, (syscall.NAME_MAX / len(pattern)))
		return (name + pattern[:(syscall.NAME_MAX-len(name))])
	}

	os.Chdir(subvoldir)
	defer os.Chdir(wdir)

	for i, l := 1, len(subvoldir); l <= syscall.PathMax*2; i++ {
		dfile, err := os.OpenFile("dummyFile", os.O_RDONLY|os.O_CREATE, 0666)
		assert.NilError(t, err, "Failed to create file at %s", subvoldir)
		assert.NilError(t, dfile.Close())
		name := getMaxFilenameFormPattern(fmt.Sprintf("LongPathToFirstSubvol_LVL%d_", i))
		assert.NilError(t, os.Mkdir(name, 0777))
		assert.NilError(t, os.Chdir(name))
		l += len(name)
		subvoldir = path.Join(subvoldir, name)
	}

	assert.NilError(t, subvolSnapshot("", subvoldir, "subvolLVL1"))
	assert.NilError(t, subvolSnapshot("", subvoldir+"/..", "subvolLVL1_0"))
	assert.NilError(t, os.Chdir("subvolLVL1"))
	subvoldir = path.Join(subvoldir, "subvolLVL1")
	assert.NilError(t, subvolSetPropRO(subvoldir, false))

	i := 1
	for l := 0; l < syscall.PathMax; i++ {
		name := getMaxFilenameFormPattern(fmt.Sprintf("LongPathToNestedSub_1_LVL%d_", i))
		assert.NilError(t, os.Mkdir(name, 0777))
		assert.NilError(t, os.Chdir(name))
		l += len(name)
		subvoldir = path.Join(subvoldir, name)
	}

	assert.NilError(t, subvolSnapshot("", subvoldir, "subvolLVL2_1"))

	for ; i > 1; i-- {
		assert.NilError(t, os.Chdir(".."))
		subvoldir = path.Dir(subvoldir)
	}

	for i, l := 1, 0; l < syscall.PathMax*2; i++ {
		name := getMaxFilenameFormPattern(fmt.Sprintf("LongPathToNestedSub_2_LVL%d_", i))
		assert.NilError(t, os.Mkdir(name, 0777))
		assert.NilError(t, os.Chdir(name))
		l += len(name)
		subvoldir = path.Join(subvoldir, name)
	}

	assert.NilError(t, subvolSnapshot("", subvoldir, "subvolLVL2_3"))
	assert.NilError(t, subvolSnapshot("", subvoldir, "subvolLVL2_2"))

	assert.NilError(t, d.Put("rootsubvol"))

	assert.NilError(t, d.Remove("rootsubvol"))
}

func TestBtrfsSubvolSELinux(t *testing.T) {
	pl, fl, err := label.InitLabels([]string{"role:test_r"})
	assert.NilError(t, err, "Fail to get init labels")
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

	assert.NilError(t, d.Create("SEsubvol", "", createOpts))

	x := d.(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	subvoldir := x.subvolumesDirID("SEsubvol")

	subvollabel, err := selinux.FileLabel(subvoldir)
	assert.NilError(t, err)
	if subvollabel != filetestlabel {
		t.Logf("subvol label %s not match with MountLabel %s", subvollabel, filetestlabel)
		t.Fail()
	}

	assert.NilError(t, d.Remove("SEsubvol"))
}

func TestBtrfsSetQuota(t *testing.T) {
	d := graphtest.GetDriver(t, "btrfs")
	defer graphtest.PutDriver(t)

	graphtest.DriverTestSetQuota(t, "btrfs", false)

	x := d.(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	subdir := x.subvolumesDir()
	files, err := ioutil.ReadDir(subdir)
	assert.NilError(t, err)
	for _, subvol := range files {
		assert.NilError(t, d.Remove(subvol.Name()))
	}
}

func TestBtrfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}
