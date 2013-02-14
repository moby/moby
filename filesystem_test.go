package docker

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func newTestFilesystem(t *testing.T, layers []string) (rootfs string, fs *Filesystem) {
	rootfs, err := ioutil.TempDir("", "docker-test-root")
	if err != nil {
		t.Fatal(err)
	}
	rwpath, err := ioutil.TempDir("", "docker-test-rw")
	if err != nil {
		t.Fatal(err)
	}
	fs = newFilesystem(rootfs, rwpath, layers)
	return
}

func TestFilesystem(t *testing.T) {
	_, filesystem := newTestFilesystem(t, []string{testLayerPath})
	if err := filesystem.Umount(); err == nil {
		t.Errorf("Umount succeeded even though the filesystem was not mounted")
	}

	if filesystem.IsMounted() {
		t.Fatal("Filesystem should not be mounted")
	}

	if err := filesystem.Mount(); err != nil {
		t.Fatal(err)
	}

	if !filesystem.IsMounted() {
		t.Fatal("Filesystem should be mounted")
	}

	if err := filesystem.Mount(); err == nil {
		t.Errorf("Double mount succeeded")
	}

	if !filesystem.IsMounted() {
		t.Fatal("Filesystem should be mounted")
	}

	if err := filesystem.Umount(); err != nil {
		t.Fatal(err)
	}

	if filesystem.IsMounted() {
		t.Fatal("Filesystem should not be mounted")
	}

	if err := filesystem.Umount(); err == nil {
		t.Errorf("Umount succeeded even though the filesystem was already umounted")
	}

	if filesystem.IsMounted() {
		t.Fatal("Filesystem should not be mounted")
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
	rootfs, filesystem := newTestFilesystem(t, []string{testLayerPath, fakeLayer})

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

func TestChanges(t *testing.T) {
	rootfs, filesystem := newTestFilesystem(t, []string{testLayerPath})
	// Mount it
	if err := filesystem.Mount(); err != nil {
		t.Fatal(err)
	}
	defer filesystem.Umount()

	var changes []Change
	var err error

	// Test without changes
	changes, err = filesystem.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Errorf("Unexpected changes :%v", changes)
	}

	// Test simple change
	file, err := os.Create(path.Join(rootfs, "test_change"))
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	changes, err = filesystem.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Errorf("Unexpected changes :%v", changes)
	}
	if changes[0].Path != "/test_change" || changes[0].Kind != ChangeAdd {
		t.Errorf("Unexpected changes :%v", changes)
	}

	// Test subdirectory change
	if err := os.Mkdir(path.Join(rootfs, "sub_change"), 0700); err != nil {
		t.Fatal(err)
	}

	file, err = os.Create(path.Join(rootfs, "sub_change", "test"))
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	changes, err = filesystem.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Errorf("Unexpected changes: %v", changes)
	}
	if changes[0].Path != "/sub_change" || changes[0].Kind != ChangeAdd || changes[1].Path != "/sub_change/test" || changes[1].Kind != ChangeAdd {
		t.Errorf("Unexpected changes: %v", changes)
	}

	// Test permission change
	if err := os.Chmod(path.Join(rootfs, "root"), 0000); err != nil {
		t.Fatal(err)
	}
	changes, err = filesystem.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 4 {
		t.Errorf("Unexpected changes: %v", changes)
	}
	if changes[0].Path != "/root" || changes[0].Kind != ChangeModify {
		t.Errorf("Unexpected changes: %v", changes)
	}

	// Test removal
	if err := os.Remove(path.Join(rootfs, "etc", "passwd")); err != nil {
		t.Fatal(err)
	}
	changes, err = filesystem.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 6 {
		t.Errorf("Unexpected changes: %v", changes)
	}
	if changes[0].Path != "/etc" || changes[0].Kind != ChangeModify || changes[1].Path != "/etc/passwd" || changes[1].Kind != ChangeDelete {
		t.Errorf("Unexpected changes: %v", changes)
	}

	// Test sub-directory removal
	if err := os.Remove(path.Join(rootfs, "usr", "bin", "sudo")); err != nil {
		t.Fatal(err)
	}
	changes, err = filesystem.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 8 {
		t.Errorf("Unexpected changes: %v", changes)
	}
	if changes[6].Path != "/usr/bin" || changes[6].Kind != ChangeModify || changes[7].Path != "/usr/bin/sudo" || changes[7].Kind != ChangeDelete {
		t.Errorf("Unexpected changes: %v", changes)
	}
}
