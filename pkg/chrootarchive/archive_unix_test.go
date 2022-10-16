//go:build !windows
// +build !windows

package chrootarchive

import (
	gotar "archive/tar"
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/archive"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// Test for CVE-2018-15664
// Assures that in the case where an "attacker" controlled path is a symlink to
// some path outside of a container's rootfs that we do not copy data to a
// container path that will actually overwrite data on the host
func TestUntarWithMaliciousSymlinks(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	dir := t.TempDir()

	root := filepath.Join(dir, "root")

	err := os.Mkdir(root, 0o755)
	assert.NilError(t, err)

	// Add a file into a directory above root
	// Ensure that we can't access this file while tarring.
	err = os.WriteFile(filepath.Join(dir, "host-file"), []byte("I am a host file"), 0644)
	assert.NilError(t, err)

	// Create some data which which will be copied into the "container" root into
	// the symlinked path.
	// Before this change, the copy would overwrite the "host" content.
	// With this change it should not.
	data := filepath.Join(dir, "data")
	err = os.Mkdir(data, 0o755)
	assert.NilError(t, err)
	err = os.WriteFile(filepath.Join(data, "local-file"), []byte("pwn3d"), 0644)
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
	hostData, err := os.ReadFile(filepath.Join(dir, "host-file"))
	assert.NilError(t, err)
	assert.Equal(t, string(hostData), "I am a host file")

	// Now test by chrooting to an attacker controlled path
	// This should succeed as is and overwrite a "host" file
	// Note that this would be a mis-use of this function.
	err = UntarWithRoot(bufRdr, safe, nil, safe)
	assert.NilError(t, err)

	hostData, err = os.ReadFile(filepath.Join(dir, "host-file"))
	assert.NilError(t, err)
	assert.Equal(t, string(hostData), "pwn3d")
}

// Test for CVE-2018-15664
// Assures that in the case where an "attacker" controlled path is a symlink to
// some path outside of a container's rootfs that we do not unwittingly leak
// host data into the archive.
func TestTarWithMaliciousSymlinks(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)
	// defer os.RemoveAll(dir)
	t.Log(dir)

	root := filepath.Join(dir, "root")

	err = os.Mkdir(root, 0o755)
	assert.NilError(t, err)

	hostFileData := []byte("I am a host file")

	// Add a file into a directory above root
	// Ensure that we can't access this file while tarring.
	err = os.WriteFile(filepath.Join(dir, "host-file"), hostFileData, 0644)
	assert.NilError(t, err)

	safe := filepath.Join(root, "safe")
	err = unix.Symlink(dir, safe)
	assert.NilError(t, err)

	data := filepath.Join(dir, "data")
	err = os.Mkdir(data, 0o755)
	assert.NilError(t, err)

	type testCase struct {
		p        string
		includes []string
	}

	cases := []testCase{
		{p: safe, includes: []string{"host-file"}},
		{p: safe + "/", includes: []string{"host-file"}},
		{p: safe, includes: nil},
		{p: safe + "/", includes: nil},
		{p: root, includes: []string{"safe/host-file"}},
		{p: root, includes: []string{"/safe/host-file"}},
		{p: root, includes: nil},
	}

	maxBytes := len(hostFileData)

	for _, tc := range cases {
		t.Run(path.Join(tc.p+"_"+strings.Join(tc.includes, "_")), func(t *testing.T) {
			// Here if we use archive.TarWithOptions directly or change the "root" parameter
			// to be the same as "safe", data from the host will be leaked into the archive
			var opts *archive.TarOptions
			if tc.includes != nil {
				opts = &archive.TarOptions{
					IncludeFiles: tc.includes,
				}
			}
			rdr, err := Tar(tc.p, opts, root)
			assert.NilError(t, err)
			defer rdr.Close()

			tr := gotar.NewReader(rdr)
			assert.Assert(t, !isDataInTar(t, tr, hostFileData, int64(maxBytes)), "host data leaked to archive")
		})
	}
}

func isDataInTar(t *testing.T, tr *gotar.Reader, compare []byte, maxBytes int64) bool {
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)

		if h.Size == 0 {
			continue
		}
		assert.Assert(t, h.Size <= maxBytes, "%s: file size exceeds max expected size %d: %d", h.Name, maxBytes, h.Size)

		data := make([]byte, int(h.Size))
		_, err = io.ReadFull(tr, data)
		assert.NilError(t, err)
		if bytes.Contains(data, compare) {
			return true
		}
	}

	return false
}
