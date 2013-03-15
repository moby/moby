package docker

import (
	"github.com/dotcloud/docker/fake"
	"github.com/dotcloud/docker/fs"
	"io/ioutil"
	"os"
	"testing"
)

// Note: This test is in the docker package because he needs to be run as root
func TestMount(t *testing.T) {
	dir, err := ioutil.TempDir("", "docker-fs-test-mount")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := fs.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := fake.FakeTar()
	if err != nil {
		t.Fatal(err)
	}

	image, err := store.Create(archive, nil, "foo", "Testing")
	if err != nil {
		t.Fatal(err)
	}

	// Create mount targets
	root, err := ioutil.TempDir("", "docker-fs-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	rw, err := ioutil.TempDir("", "docker-fs-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rw)

	mountpoint, err := image.Mount(root, rw)
	if err != nil {
		t.Fatal(err)
	}
	defer mountpoint.Umount()
	// Mountpoint should be marked as mounted
	if !mountpoint.Mounted() {
		t.Fatal("Mountpoint not mounted")
	}
	// There should be one mountpoint registered
	if mps, err := image.Mountpoints(); err != nil {
		t.Fatal(err)
	} else if len(mps) != 1 {
		t.Fatal("Wrong number of mountpoints registered (should be %d, not %d)", 1, len(mps))
	}
	// Unmounting should work
	if err := mountpoint.Umount(); err != nil {
		t.Fatal(err)
	}
	// De-registering should work
	if err := mountpoint.Deregister(); err != nil {
		t.Fatal(err)
	}
	if mps, err := image.Mountpoints(); err != nil {
		t.Fatal(err)
	} else if len(mps) != 0 {
		t.Fatal("Wrong number of mountpoints registered (should be %d, not %d)", 0, len(mps))
	}
	// General health check
	// if err := healthCheck(store); err != nil {
	// 	t.Fatal(err)
	// }
}
