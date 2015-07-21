package cephfs_test

import "testing"
import "github.com/noahdesu/go-ceph/cephfs"
import "github.com/stretchr/testify/assert"

func TestCreateMount(t *testing.T) {
	mount, err := cephfs.CreateMount()
	assert.NoError(t, err)
	assert.NotNil(t, mount)
}

func TestMountRoot(t *testing.T) {
	mount, err := cephfs.CreateMount()
	assert.NoError(t, err)
	assert.NotNil(t, mount)

	err = mount.ReadDefaultConfigFile()
	assert.NoError(t, err)

	err = mount.Mount()
	assert.NoError(t, err)
}

func TestSyncFs(t *testing.T) {
	mount, err := cephfs.CreateMount()
	assert.NoError(t, err)
	assert.NotNil(t, mount)

	err = mount.ReadDefaultConfigFile()
	assert.NoError(t, err)

	err = mount.Mount()
	assert.NoError(t, err)

	err = mount.SyncFs()
	assert.NoError(t, err)
}

func TestChangeDir(t *testing.T) {
	mount, err := cephfs.CreateMount()
	assert.NoError(t, err)
	assert.NotNil(t, mount)

	err = mount.ReadDefaultConfigFile()
	assert.NoError(t, err)

	err = mount.Mount()
	assert.NoError(t, err)

	dir1 := mount.CurrentDir()
	assert.NotNil(t, dir1)

	err = mount.MakeDir("/asdf", 0755)
	assert.NoError(t, err)

	err = mount.ChangeDir("/asdf")
	assert.NoError(t, err)

	dir2 := mount.CurrentDir()
	assert.NotNil(t, dir2)

	assert.NotEqual(t, dir1, dir2)
	assert.Equal(t, dir1, "/")
	assert.Equal(t, dir2, "/asdf")
}
