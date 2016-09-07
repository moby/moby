// +build linux freebsd

package graphtest

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"reflect"
	"syscall"
	"unsafe"

	units "src/github.com/docker/go-units"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

var (
	drv *Driver
)

// Driver conforms to graphdriver.Driver interface and
// contains information such as root and reference count of the number of clients using it.
// This helps in testing drivers added into the framework.
type Driver struct {
	graphdriver.Driver
	root     string
	refCount int
}

func newDriver(c *check.C, name string, options []string) *Driver {
	root, err := ioutil.TempDir("", "docker-graphtest-")
	if err != nil {
		c.Fatal(err)
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		c.Fatal(err)
	}

	d, err := graphdriver.GetDriver(name, root, options, nil, nil)
	if err != nil {
		c.Logf("graphdriver: %v\n", err)
		if err == graphdriver.ErrNotSupported || err == graphdriver.ErrPrerequisites || err == graphdriver.ErrIncompatibleFS {
			c.Skip("Driver " + name + " not supported")
		}
		c.Fatal(err)
	}
	return &Driver{d, root, 1}
}

func cleanup(c *check.C, d *Driver) {
	if err := drv.Cleanup(); err != nil {
		c.Fatal(err)
	}
	os.RemoveAll(d.root)
}

// GetDriver create a new driver with given name or return an existing driver with the name updating the reference count.
func GetDriver(c *check.C, name string, options ...string) graphdriver.Driver {
	if drv == nil {
		drv = newDriver(c, name, options)
	} else {
		drv.refCount++
	}
	return drv
}

// PutDriver removes the driver if it is no longer used and updates the reference count.
func PutDriver(c *check.C) {
	if drv == nil {
		c.Skip("No driver to put!")
	}
	drv.refCount--
	if drv.refCount == 0 {
		cleanup(c, drv)
		drv = nil
	}
}

// DriverTestCreateEmpty creates a new image and verifies it is empty and the right metadata
func DriverTestCreateEmpty(c *check.C, drivername string, driverOptions ...string) {
	driver := GetDriver(c, drivername, driverOptions...)
	defer PutDriver(c)

	if err := driver.Create("empty", "", "", nil); err != nil {
		c.Fatal(err)
	}

	defer func() {
		if err := driver.Remove("empty"); err != nil {
			c.Fatal(err)
		}
	}()

	if !driver.Exists("empty") {
		c.Fatal("Newly created image doesn't exist")
	}

	dir, err := driver.Get("empty", "")
	if err != nil {
		c.Fatal(err)
	}

	verifyFile(c, dir, 0755|os.ModeDir, 0, 0)

	// Verify that the directory is empty
	fis, err := readDir(dir)
	if err != nil {
		c.Fatal(err)
	}

	if len(fis) != 0 {
		c.Fatal("New directory not empty")
	}

	driver.Put("empty")
}

// DriverTestCreateBase create a base driver and verify.
func DriverTestCreateBase(c *check.C, drivername string, driverOptions ...string) {
	driver := GetDriver(c, drivername, driverOptions...)
	defer PutDriver(c)

	createBase(c, driver, "Base")
	defer func() {
		if err := driver.Remove("Base"); err != nil {
			c.Fatal(err)
		}
	}()
	verifyBase(c, driver, "Base")
}

// DriverTestCreateSnap Create a driver and snap and verify.
func DriverTestCreateSnap(c *check.C, drivername string, driverOptions ...string) {
	driver := GetDriver(c, drivername, driverOptions...)
	defer PutDriver(c)

	createBase(c, driver, "Base")

	defer func() {
		if err := driver.Remove("Base"); err != nil {
			c.Fatal(err)
		}
	}()

	if err := driver.Create("Snap", "Base", "", nil); err != nil {
		c.Fatal(err)
	}

	defer func() {
		if err := driver.Remove("Snap"); err != nil {
			c.Fatal(err)
		}
	}()

	verifyBase(c, driver, "Snap")
}

// DriverTestDeepLayerRead reads a file from a lower layer under a given number of layers
func DriverTestDeepLayerRead(c *check.C, layerCount int, drivername string, driverOptions ...string) {
	driver := GetDriver(c, drivername, driverOptions...)
	defer PutDriver(c)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	content := []byte("test content")
	if err := addFile(driver, base, "testfile.txt", content); err != nil {
		c.Fatal(err)
	}

	topLayer, err := addManyLayers(driver, base, layerCount)
	if err != nil {
		c.Fatal(err)
	}

	err = checkManyLayers(driver, topLayer, layerCount)
	if err != nil {
		c.Fatal(err)
	}

	if err := checkFile(driver, topLayer, "testfile.txt", content); err != nil {
		c.Fatal(err)
	}
}

// DriverTestDiffApply tests diffing and applying produces the same layer
func DriverTestDiffApply(c *check.C, fileCount int, drivername string, driverOptions ...string) {
	driver := GetDriver(c, drivername, driverOptions...)
	defer PutDriver(c)
	base := stringid.GenerateRandomID()
	upper := stringid.GenerateRandomID()
	deleteFile := "file-remove.txt"
	deleteFileContent := []byte("This file should get removed in upper!")
	deleteDir := "var/lib"

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, base, fileCount, 3); err != nil {
		c.Fatal(err)
	}

	if err := addFile(driver, base, deleteFile, deleteFileContent); err != nil {
		c.Fatal(err)
	}

	if err := addDirectory(driver, base, deleteDir); err != nil {
		c.Fatal(err)
	}

	if err := driver.Create(upper, base, "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, upper, fileCount, 6); err != nil {
		c.Fatal(err)
	}

	if err := removeAll(driver, upper, deleteFile, deleteDir); err != nil {
		c.Fatal(err)
	}

	diffSize, err := driver.DiffSize(upper, "")
	if err != nil {
		c.Fatal(err)
	}

	diff := stringid.GenerateRandomID()
	if err := driver.Create(diff, base, "", nil); err != nil {
		c.Fatal(err)
	}

	if err := checkManyFiles(driver, diff, fileCount, 3); err != nil {
		c.Fatal(err)
	}

	if err := checkFile(driver, diff, deleteFile, deleteFileContent); err != nil {
		c.Fatal(err)
	}

	arch, err := driver.Diff(upper, base)
	if err != nil {
		c.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)
	if _, err := buf.ReadFrom(arch); err != nil {
		c.Fatal(err)
	}
	if err := arch.Close(); err != nil {
		c.Fatal(err)
	}

	applyDiffSize, err := driver.ApplyDiff(diff, base, bytes.NewReader(buf.Bytes()))
	if err != nil {
		c.Fatal(err)
	}

	if applyDiffSize != diffSize {
		c.Fatalf("Apply diff size different, got %d, expected %d", applyDiffSize, diffSize)
	}

	if err := checkManyFiles(driver, diff, fileCount, 6); err != nil {
		c.Fatal(err)
	}

	if err := checkFileRemoved(driver, diff, deleteFile); err != nil {
		c.Fatal(err)
	}

	if err := checkFileRemoved(driver, diff, deleteDir); err != nil {
		c.Fatal(err)
	}
}

// DriverTestChanges tests computed changes on a layer matches changes made
func DriverTestChanges(c *check.C, drivername string, driverOptions ...string) {
	driver := GetDriver(c, drivername, driverOptions...)
	defer PutDriver(c)
	base := stringid.GenerateRandomID()
	upper := stringid.GenerateRandomID()

	if err := driver.Create(base, "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := addManyFiles(driver, base, 20, 3); err != nil {
		c.Fatal(err)
	}

	if err := driver.Create(upper, base, "", nil); err != nil {
		c.Fatal(err)
	}

	expectedChanges, err := changeManyFiles(driver, upper, 20, 6)
	if err != nil {
		c.Fatal(err)
	}

	changes, err := driver.Changes(upper, base)
	if err != nil {
		c.Fatal(err)
	}

	if err = checkChanges(expectedChanges, changes); err != nil {
		c.Fatal(err)
	}
}

func writeRandomFile(path string, size uint64) error {
	buf := make([]int64, size/8)

	r := rand.NewSource(0)
	for i := range buf {
		buf[i] = r.Int63()
	}

	// Cast to []byte
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&buf))
	header.Len *= 8
	header.Cap *= 8
	data := *(*[]byte)(unsafe.Pointer(&header))

	return ioutil.WriteFile(path, data, 0700)
}

// DriverTestSetQuota Create a driver and test setting quota.
func DriverTestSetQuota(c *check.C, drivername string) {
	driver := GetDriver(c, drivername)
	defer PutDriver(c)

	createBase(c, driver, "Base")
	storageOpt := make(map[string]string, 1)
	storageOpt["size"] = "50M"
	if err := driver.Create("zfsTest", "Base", "", storageOpt); err != nil {
		c.Fatal(err)
	}

	mountPath, err := driver.Get("zfsTest", "")
	if err != nil {
		c.Fatal(err)
	}

	quota := uint64(50 * units.MiB)
	err = writeRandomFile(path.Join(mountPath, "file"), quota*2)
	if pathError, ok := err.(*os.PathError); ok && pathError.Err != syscall.EDQUOT {
		c.Fatalf("expect write() to fail with %v, got %v", syscall.EDQUOT, err)
	}

}
