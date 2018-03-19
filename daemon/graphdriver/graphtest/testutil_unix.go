// +build linux freebsd

package graphtest // import "github.com/docker/docker/daemon/graphdriver/graphtest"

import (
	"os"
	"syscall"
	"testing"

	contdriver "github.com/containerd/continuity/driver"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"golang.org/x/sys/unix"
)

func verifyFile(t testing.TB, path string, mode os.FileMode, uid, gid uint32) {
	fi, err := os.Stat(path)
	assert.NilError(t, err)

	actual := fi.Mode()
	assert.Check(t, is.Equal(mode&os.ModeType, actual&os.ModeType), path)
	assert.Check(t, is.Equal(mode&os.ModePerm, actual&os.ModePerm), path)
	assert.Check(t, is.Equal(mode&os.ModeSticky, actual&os.ModeSticky), path)
	assert.Check(t, is.Equal(mode&os.ModeSetuid, actual&os.ModeSetuid), path)
	assert.Check(t, is.Equal(mode&os.ModeSetgid, actual&os.ModeSetgid), path)

	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		assert.Check(t, is.Equal(uid, stat.Uid), path)
		assert.Check(t, is.Equal(gid, stat.Gid), path)
	}
}

func createBase(t testing.TB, driver graphdriver.Driver, name string) {
	// We need to be able to set any perms
	oldmask := unix.Umask(0)
	defer unix.Umask(oldmask)

	err := driver.CreateReadWrite(name, "", nil)
	assert.NilError(t, err)

	dirFS, err := driver.Get(name, "")
	assert.NilError(t, err)
	defer driver.Put(name)

	subdir := dirFS.Join(dirFS.Path(), "a subdir")
	assert.NilError(t, dirFS.Mkdir(subdir, 0705|os.ModeSticky))
	assert.NilError(t, dirFS.Lchown(subdir, 1, 2))

	file := dirFS.Join(dirFS.Path(), "a file")
	err = contdriver.WriteFile(dirFS, file, []byte("Some data"), 0222|os.ModeSetuid)
	assert.NilError(t, err)
}

func verifyBase(t testing.TB, driver graphdriver.Driver, name string) {
	dirFS, err := driver.Get(name, "")
	assert.NilError(t, err)
	defer driver.Put(name)

	subdir := dirFS.Join(dirFS.Path(), "a subdir")
	verifyFile(t, subdir, 0705|os.ModeDir|os.ModeSticky, 1, 2)

	file := dirFS.Join(dirFS.Path(), "a file")
	verifyFile(t, file, 0222|os.ModeSetuid, 0, 0)

	files, err := readDir(dirFS, dirFS.Path())
	assert.NilError(t, err)
	assert.Check(t, is.Len(files, 2))
}
