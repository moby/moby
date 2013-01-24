package docker

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func newTestFilesystem(t *testing.T, layers []string) (rootfs string, rwpath string, fs *Filesystem) {
	rootfs, err := ioutil.TempDir("", "docker-test-root")
	if err != nil {
		t.Fatal(err)
	}
	rwpath, err = ioutil.TempDir("", "docker-test-rw")
	if err != nil {
		t.Fatal(err)
	}
	fs = newFilesystem(rootfs, rwpath, layers)
	return
}

func TestFilesystem(t *testing.T) {
	_, _, filesystem := newTestFilesystem(t, []string{"/var/lib/docker/images/ubuntu"})
	if err := filesystem.Umount(); err == nil {
		t.Errorf("Umount succeeded even though the filesystem was not mounted")
	}

	if err := filesystem.Mount(); err != nil {
		t.Fatal(err)
	}

	if err := filesystem.Mount(); err == nil {
		t.Errorf("Double mount succeeded")
	}

	if err := filesystem.Umount(); err != nil {
		t.Fatal(err)
	}

	if err := filesystem.Umount(); err == nil {
		t.Errorf("Umount succeeded even though the filesystem was already umounted")
	}
}

func TestFilesystemMultiLayer(t *testing.T) {
	// Create a fake layer
	fakeLayer, err := ioutil.TempDir("", "docker-layer")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello world")
	if err := ioutil.WriteFile(path.Join(fakeLayer, "test_file"), data, 0700); err != nil {
		t.Fatal(err)
	}

	// Create the layered filesystem and add our fake layer on top
	rootfs, _, filesystem := newTestFilesystem(t, []string{"/var/lib/docker/images/ubuntu", fakeLayer})

	// Mount it
	if err := filesystem.Mount(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := filesystem.Umount(); err != nil {
			t.Fatal(err)
		}
	}()

	// Check to see whether we can access our fake layer
	if _, err := os.Stat(path.Join(rootfs, "test_file")); err != nil {
		t.Fatal(err)
	}
	fsdata, err := ioutil.ReadFile(path.Join(rootfs, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, fsdata) {
		t.Error(string(fsdata))
	}
}
