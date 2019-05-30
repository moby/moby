// +build !windows

package chrootarchive

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/archive"
	"golang.org/x/sys/unix"
	"gotest.tools/assert"
)

// Test for CVE-2018-15664
// Assures that in the case where an "attacker" controlled path is a symlink to
// some path outside of a container's rootfs that we do not copy data to a
// container path that will actually overwrite data on the host
func TestUntarWithMaliciousSymlinks(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	root := filepath.Join(dir, "root")

	err = os.MkdirAll(root, 0755)
	assert.NilError(t, err)

	// Add a file into a directory above root
	// Ensure that we can't access this file while tarring.
	err = ioutil.WriteFile(filepath.Join(dir, "host-file"), []byte("I am a host file"), 0644)
	assert.NilError(t, err)

	// Create some data which which will be copied into the "container" root into
	// the symlinked path.
	// Before this change, the copy would overwrite the "host" content.
	// With this change it should not.
	data := filepath.Join(dir, "data")
	err = os.MkdirAll(data, 0755)
	assert.NilError(t, err)
	err = ioutil.WriteFile(filepath.Join(data, "local-file"), []byte("pwn3d"), 0644)
	assert.NilError(t, err)

	safe := filepath.Join(root, "safe")
	err = unix.Symlink(dir, safe)
	assert.NilError(t, err)

	rdr, err := archive.TarWithOptions(data, &archive.TarOptions{IncludeFiles: []string{"local-file"}, RebaseNames: map[string]string{"local-file": "host-file"}})
	assert.NilError(t, err)

	// Use tee to test both the good case and the bad case w/o recreating the archive
	bufRdr := bytes.NewBuffer(nil)
	tee := io.TeeReader(rdr, bufRdr)

	err = UntarWithRoot(tee, safe, nil, root)
	assert.Assert(t, err != nil)
	assert.ErrorContains(t, err, "open /safe/host-file: no such file or directory")

	// Make sure the "host" file is still in tact
	// Before the fix the host file would be overwritten
	hostData, err := ioutil.ReadFile(filepath.Join(dir, "host-file"))
	assert.NilError(t, err)
	assert.Equal(t, string(hostData), "I am a host file")

	// Now test by chrooting to an attacker controlled path
	// This should succeed as is and overwrite a "host" file
	// Note that this would be a mis-use of this function.
	err = UntarWithRoot(bufRdr, safe, nil, safe)
	assert.NilError(t, err)

	hostData, err = ioutil.ReadFile(filepath.Join(dir, "host-file"))
	assert.NilError(t, err)
	assert.Equal(t, string(hostData), "pwn3d")
}
