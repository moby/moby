package graphtest

import (
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
)

var (
	drv *Driver
)

type Driver struct {
	graphdriver.Driver
	root     string
	refCount int
}

func newDriver(t *testing.T, name string) *Driver {
	root, err := ioutil.TempDir("/var/tmp", "docker-graphtest-")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	d, err := graphdriver.GetDriver(name, root, nil)
	if err != nil {
		if err == graphdriver.ErrNotSupported || err == graphdriver.ErrPrerequisites {
			t.Skip("Driver %s not supported", name)
		}
		t.Fatal(err)
	}
	return &Driver{d, root, 1}
}

func cleanup(t *testing.T, d *Driver) {
	if err := drv.Cleanup(); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(d.root)
}

func GetDriver(t *testing.T, name string) graphdriver.Driver {
	if drv == nil {
		drv = newDriver(t, name)
	} else {
		drv.refCount++
	}
	return drv
}

func PutDriver(t *testing.T) {
	if drv == nil {
		t.Skip("No driver to put!")
	}
	drv.refCount--
	if drv.refCount == 0 {
		cleanup(t, drv)
		drv = nil
	}
}

func verifyFile(t *testing.T, path string, mode os.FileMode, uid, gid uint32) {
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if fi.Mode()&os.ModeType != mode&os.ModeType {
		t.Fatalf("Expected %s type 0x%x, got 0x%x", path, mode&os.ModeType, fi.Mode()&os.ModeType)
	}

	if fi.Mode()&os.ModePerm != mode&os.ModePerm {
		t.Fatalf("Expected %s mode %o, got %o", path, mode&os.ModePerm, fi.Mode()&os.ModePerm)
	}

	if fi.Mode()&os.ModeSticky != mode&os.ModeSticky {
		t.Fatalf("Expected %s sticky 0x%x, got 0x%x", path, mode&os.ModeSticky, fi.Mode()&os.ModeSticky)
	}

	if fi.Mode()&os.ModeSetuid != mode&os.ModeSetuid {
		t.Fatalf("Expected %s setuid 0x%x, got 0x%x", path, mode&os.ModeSetuid, fi.Mode()&os.ModeSetuid)
	}

	if fi.Mode()&os.ModeSetgid != mode&os.ModeSetgid {
		t.Fatalf("Expected %s setgid 0x%x, got 0x%x", path, mode&os.ModeSetgid, fi.Mode()&os.ModeSetgid)
	}

	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		if stat.Uid != uid {
			t.Fatalf("%s no owned by uid %d", path, uid)
		}
		if stat.Gid != gid {
			t.Fatalf("%s not owned by gid %d", path, gid)
		}
	}

}

// Creates an new image and verifies it is empty and the right metadata
func DriverTestCreateEmpty(t *testing.T, drivername string) {
	driver := GetDriver(t, drivername)
	defer PutDriver(t)

	if err := driver.Create("empty", ""); err != nil {
		t.Fatal(err)
	}

	if !driver.Exists("empty") {
		t.Fatal("Newly created image doesn't exist")
	}

	dir, err := driver.Get("empty", "")
	if err != nil {
		t.Fatal(err)
	}

	verifyFile(t, dir, 0755|os.ModeDir, 0, 0)

	// Verify that the directory is empty
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(fis) != 0 {
		t.Fatal("New directory not empty")
	}

	driver.Put("empty")

	if err := driver.Remove("empty"); err != nil {
		t.Fatal(err)
	}

}

func createBase(t *testing.T, driver graphdriver.Driver, name string) {
	// We need to be able to set any perms
	oldmask := syscall.Umask(0)
	defer syscall.Umask(oldmask)

	if err := driver.Create(name, ""); err != nil {
		t.Fatal(err)
	}

	dir, err := driver.Get(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Put(name)

	subdir := path.Join(dir, "a subdir")
	if err := os.Mkdir(subdir, 0705|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(subdir, 1, 2); err != nil {
		t.Fatal(err)
	}

	file := path.Join(dir, "a file")
	if err := ioutil.WriteFile(file, []byte("Some data"), 0222|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}
}

func verifyBase(t *testing.T, driver graphdriver.Driver, name string) {
	dir, err := driver.Get(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Put(name)

	subdir := path.Join(dir, "a subdir")
	verifyFile(t, subdir, 0705|os.ModeDir|os.ModeSticky, 1, 2)

	file := path.Join(dir, "a file")
	verifyFile(t, file, 0222|os.ModeSetuid, 0, 0)

	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(fis) != 2 {
		t.Fatal("Unexpected files in base image")
	}

}

func DriverTestCreateBase(t *testing.T, drivername string) {
	driver := GetDriver(t, drivername)
	defer PutDriver(t)

	createBase(t, driver, "Base")
	verifyBase(t, driver, "Base")

	if err := driver.Remove("Base"); err != nil {
		t.Fatal(err)
	}
}

func DriverTestCreateSnap(t *testing.T, drivername string) {
	driver := GetDriver(t, drivername)
	defer PutDriver(t)

	createBase(t, driver, "Base")

	if err := driver.Create("Snap", "Base"); err != nil {
		t.Fatal(err)
	}

	verifyBase(t, driver, "Snap")

	if err := driver.Remove("Snap"); err != nil {
		t.Fatal(err)
	}

	if err := driver.Remove("Base"); err != nil {
		t.Fatal(err)
	}
}
