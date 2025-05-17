//go:build linux || freebsd

package graphtest // import "github.com/docker/docker/daemon/graphdriver/graphtest"

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	contdriver "github.com/containerd/continuity/driver"
	"github.com/docker/docker/daemon/graphdriver"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func verifyFile(tb testing.TB, path string, mode os.FileMode, uid, gid uint32) {
	fi, err := os.Stat(path)
	assert.NilError(tb, err)

	actual := fi.Mode()
	assert.Check(tb, is.Equal(mode&os.ModeType, actual&os.ModeType), path)
	assert.Check(tb, is.Equal(mode&os.ModePerm, actual&os.ModePerm), path)
	assert.Check(tb, is.Equal(mode&os.ModeSticky, actual&os.ModeSticky), path)
	assert.Check(tb, is.Equal(mode&os.ModeSetuid, actual&os.ModeSetuid), path)
	assert.Check(tb, is.Equal(mode&os.ModeSetgid, actual&os.ModeSetgid), path)

	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		assert.Check(tb, is.Equal(uid, stat.Uid), path)
		assert.Check(tb, is.Equal(gid, stat.Gid), path)
	}
}

func createBase(tb testing.TB, driver graphdriver.Driver, name string) {
	// We need to be able to set any perms
	oldmask := unix.Umask(0)
	defer unix.Umask(oldmask)

	err := driver.CreateReadWrite(name, "", nil)
	assert.NilError(tb, err)

	dirFS, err := driver.Get(name, "")
	assert.NilError(tb, err)
	defer driver.Put(name)

	subdir := filepath.Join(dirFS, "a subdir")
	assert.NilError(tb, os.Mkdir(subdir, 0o705|os.ModeSticky))
	assert.NilError(tb, contdriver.LocalDriver.Lchown(subdir, 1, 2))

	file := filepath.Join(dirFS, "a file")
	err = os.WriteFile(file, []byte("Some data"), 0o222|os.ModeSetuid)
	assert.NilError(tb, err)
}

func verifyBase(tb testing.TB, driver graphdriver.Driver, name string) {
	dirFS, err := driver.Get(name, "")
	assert.NilError(tb, err)
	defer driver.Put(name)

	subdir := filepath.Join(dirFS, "a subdir")
	verifyFile(tb, subdir, 0o705|os.ModeDir|os.ModeSticky, 1, 2)

	file := filepath.Join(dirFS, "a file")
	verifyFile(tb, file, 0o222|os.ModeSetuid, 0, 0)

	files, err := readDir(dirFS)
	assert.NilError(tb, err)
	assert.Check(tb, is.Len(files, 2))
}
