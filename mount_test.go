package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/dotcloud/docker/fs"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}

// Look for inconsistencies in a store.
func healthCheck(store *fs.Store) error {
	parents := make(map[string]bool)
	paths, err := store.Paths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		images, err := store.List(path)
		if err != nil {
			return err
		}
		IDs := make(map[string]bool) // All IDs for this path
		for _, img := range images {
			// Check for duplicate IDs per path
			if _, exists := IDs[img.Id]; exists {
				return fmt.Errorf("Duplicate ID: %s", img.Id)
			} else {
				IDs[img.Id] = true
			}
			// Store parent for 2nd pass
			if parent := img.Parent; parent != "" {
				parents[parent] = true
			}
		}
	}
	// Check non-existing parents
	for parent := range parents {
		if _, exists := parents[parent]; !exists {
			return fmt.Errorf("Reference to non-registered parent: %s", parent)
		}
	}
	return nil
}

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

	archive, err := fakeTar()
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
	if err := healthCheck(store); err != nil {
		t.Fatal(err)
	}
}
