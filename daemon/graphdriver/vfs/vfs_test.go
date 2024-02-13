//go:build linux

package vfs // import "github.com/docker/docker/daemon/graphdriver/vfs"

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/moby/sys/mount"
	"gotest.tools/v3/assert"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/graphtest"
)

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestVfsSetup and TestVfsTeardown
func TestVfsSetup(t *testing.T) {
	graphtest.GetDriver(t, "vfs")
}

func TestVfsCreateEmpty(t *testing.T) {
	graphtest.DriverTestCreateEmpty(t, "vfs")
}

func TestVfsCreateBase(t *testing.T) {
	graphtest.DriverTestCreateBase(t, "vfs")
}

func TestVfsCreateSnap(t *testing.T) {
	graphtest.DriverTestCreateSnap(t, "vfs")
}

func TestVfsSetQuota(t *testing.T) {
	graphtest.DriverTestSetQuota(t, "vfs", false)
}

func TestVfsTeardown(t *testing.T) {
	graphtest.PutDriver(t)
}

func TestXattrUnsupportedByBackingFS(t *testing.T) {
	rootdir := t.TempDir()
	// The ramfs filesystem is unconditionally compiled into the kernel,
	// and does not support extended attributes.
	err := mount.Mount("ramfs", rootdir, "ramfs", "")
	if errors.Is(err, syscall.EPERM) {
		t.Skip("test requires the ability to mount a filesystem")
	}
	assert.NilError(t, err)
	defer mount.Unmount(rootdir)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	const (
		filename = "test.txt"
		content  = "hello world\n"
	)
	assert.NilError(t, tw.WriteHeader(&tar.Header{
		Name: filename,
		Mode: 0o644,
		Size: int64(len(content)),
		PAXRecords: map[string]string{
			"SCHILY.xattr.user.test": "helloxattr",
		},
	}))
	_, err = io.WriteString(tw, content)
	assert.NilError(t, err)
	assert.NilError(t, tw.Close())
	testlayer := buf.Bytes()

	for _, tt := range []struct {
		name        string
		opts        []string
		expectErrIs error
	}{
		{
			name:        "Default",
			expectErrIs: syscall.EOPNOTSUPP,
		},
		{
			name: "vfs.xattrs=i_want_broken_containers",
			opts: []string{"vfs.xattrs=i_want_broken_containers"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			subdir := filepath.Join(rootdir, tt.name)
			assert.NilError(t, os.Mkdir(subdir, 0o755))
			d, err := graphdriver.GetDriver("vfs", nil,
				graphdriver.Options{DriverOptions: tt.opts, Root: subdir})
			assert.NilError(t, err)
			defer d.Cleanup()

			assert.NilError(t, d.Create("test", "", nil))
			_, err = d.ApplyDiff("test", "", bytes.NewReader(testlayer))
			assert.ErrorIs(t, err, tt.expectErrIs)

			if err == nil {
				path, err := d.Get("test", "")
				assert.NilError(t, err)
				defer d.Put("test")
				actual, err := os.ReadFile(filepath.Join(path, filename))
				assert.NilError(t, err)
				assert.Equal(t, string(actual), content)
			}
		})
	}
}
