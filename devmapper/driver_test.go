package devmapper

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func init() {
	// Reduce the size the the base fs and loopback for the tests
	DefaultDataLoopbackSize = 300 * 1024 * 1024
	DefaultMetaDataLoopbackSize = 200 * 1024 * 1024
	DefaultBaseFsSize = 300 * 1024 * 1024

}

func mkTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "docker-test-devmapper-")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func newDriver(t *testing.T) *Driver {
	home := mkTestDirectory(t)
	d, err := Init(home)
	if err != nil {
		t.Fatal(err)
	}
	return d.(*Driver)
}

func cleanup(d *Driver) {
	d.Cleanup()
	os.RemoveAll(d.home)
}

func TestInit(t *testing.T) {
	home := mkTestDirectory(t)
	defer os.RemoveAll(home)
	driver, err := Init(home)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := driver.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	id := "foo"
	if err := driver.Create(id, ""); err != nil {
		t.Fatal(err)
	}
	dir, err := driver.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if !st.IsDir() {
		t.Fatalf("Get(%V) did not return a directory", id)
	}
}

func TestDriverName(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if d.String() != "devicemapper" {
		t.Fatalf("Expected driver name to be devicemapper got %s", d.String())
	}
}

func TestDriverCreate(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
}

func TestDriverRemove(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	if err := d.Remove("1"); err != nil {
		t.Fatal(err)
	}
}

func TestCleanup(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(d.home)

	mountPoints := make([]string, 2)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	// Mount the id
	p, err := d.Get("1")
	if err != nil {
		t.Fatal(err)
	}
	mountPoints[0] = p

	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	p, err = d.Get("2")
	if err != nil {
		t.Fatal(err)
	}
	mountPoints[1] = p

	// Ensure that all the mount points are currently mounted
	for _, p := range mountPoints {
		if mounted, err := Mounted(p); err != nil {
			t.Fatal(err)
		} else if !mounted {
			t.Fatalf("Expected %s to be mounted", p)
		}
	}

	// Ensure that devices are active
	for _, p := range []string{"1", "2"} {
		if !d.HasActivatedDevice(p) {
			t.Fatalf("Expected %s to have an active device", p)
		}
	}

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}

	// Ensure that all the mount points are no longer mounted
	for _, p := range mountPoints {
		if mounted, err := Mounted(p); err != nil {
			t.Fatal(err)
		} else if mounted {
			t.Fatalf("Expected %s to not be mounted", p)
		}
	}

	// Ensure that devices are no longer activated
	for _, p := range []string{"1", "2"} {
		if d.HasActivatedDevice(p) {
			t.Fatalf("Expected %s not be an active device", p)
		}
	}
}

func TestNotMounted(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	mounted, err := Mounted(path.Join(d.home, "mnt", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if mounted {
		t.Fatal("Id 1 should not be mounted")
	}
}

func TestMounted(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}

	mounted, err := Mounted(path.Join(d.home, "mnt", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if !mounted {
		t.Fatal("Id 1 should be mounted")
	}
}

func TestInitCleanedDriver(t *testing.T) {
	d := newDriver(t)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}

	driver, err := Init(d.home)
	if err != nil {
		t.Fatal(err)
	}
	d = driver.(*Driver)
	defer cleanup(d)

	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}
}

func TestMountMountedDriver(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	// Perform get on same id to ensure that it will
	// not be mounted twice
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}
}

func TestGetReturnsValidDevice(t *testing.T) {
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	if !d.HasDevice("1") {
		t.Fatalf("Expected id 1 to be in device set")
	}

	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}

	if !d.HasActivatedDevice("1") {
		t.Fatalf("Expected id 1 to be activated")
	}

	if !d.HasInitializedDevice("1") {
		t.Fatalf("Expected id 1 to be initialized")
	}
}

func TestDriverGetSize(t *testing.T) {
	t.Skipf("Size is currently not implemented")

	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	mountPoint, err := d.Get("1")
	if err != nil {
		t.Fatal(err)
	}

	size := int64(1024)

	f, err := os.Create(path.Join(mountPoint, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	f.Close()

	diffSize, err := d.Size("1")
	if err != nil {
		t.Fatal(err)
	}
	if diffSize != size {
		t.Fatalf("Expected size %d got %d", size, diffSize)
	}
}
