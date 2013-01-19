package docker

import (
	"io/ioutil"
	"testing"
)

func TestFilesystem(t *testing.T) {
	rootfs, err := ioutil.TempDir("", "docker-test-root")
	if err != nil {
		t.Fatal(err)
	}
	rwpath, err := ioutil.TempDir("", "docker-test-rw")
	if err != nil {
		t.Fatal(err)
	}

	filesystem := newFilesystem(rootfs, rwpath, []string{"/var/lib/docker/images/ubuntu", "/var/lib/docker/images/test"})

	if err := filesystem.Umount(); err == nil {
		t.Errorf("Umount succeeded even though the filesystem was not mounted")
	}

	if err := filesystem.Mount(); err != nil {
		t.Fatal(err)
	}

	if err := filesystem.Umount(); err != nil {
		t.Fatal(err)
	}

	if err := filesystem.Umount(); err == nil {
		t.Errorf("Umount succeeded even though the filesystem was already umounted")
	}
}
