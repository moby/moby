package archive // import "github.com/docker/docker/pkg/archive"

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/docker/docker/pkg/system"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// setupOverlayTestDir creates files in a directory with overlay whiteouts
// Tree layout
//
//	.
//	├── d1     # opaque, 0700
//	│   └── f1 # empty file, 0600
//	├── d2     # opaque, 0750
//	│   └── f1 # empty file, 0660
//	└── d3     # 0700
//	    └── f1 # whiteout, 0644
func setupOverlayTestDir(t *testing.T, src string) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	skip.If(t, userns.RunningInUserNS(), "skipping test that requires initial userns (trusted.overlay.opaque xattr cannot be set in userns, even with Ubuntu kernel)")
	// Create opaque directory containing single file and permission 0700
	err := os.Mkdir(filepath.Join(src, "d1"), 0o700)
	assert.NilError(t, err)

	err = system.Lsetxattr(filepath.Join(src, "d1"), "trusted.overlay.opaque", []byte("y"), 0)
	assert.NilError(t, err)

	err = os.WriteFile(filepath.Join(src, "d1", "f1"), []byte{}, 0o600)
	assert.NilError(t, err)

	// Create another opaque directory containing single file but with permission 0750
	err = os.Mkdir(filepath.Join(src, "d2"), 0o750)
	assert.NilError(t, err)

	err = system.Lsetxattr(filepath.Join(src, "d2"), "trusted.overlay.opaque", []byte("y"), 0)
	assert.NilError(t, err)

	err = os.WriteFile(filepath.Join(src, "d2", "f1"), []byte{}, 0o660)
	assert.NilError(t, err)

	// Create regular directory with deleted file
	err = os.Mkdir(filepath.Join(src, "d3"), 0o700)
	assert.NilError(t, err)

	err = system.Mknod(filepath.Join(src, "d3", "f1"), unix.S_IFCHR, 0)
	assert.NilError(t, err)
}

func checkOpaqueness(t *testing.T, path string, opaque string) {
	xattrOpaque, err := system.Lgetxattr(path, "trusted.overlay.opaque")
	assert.NilError(t, err)

	if string(xattrOpaque) != opaque {
		t.Fatalf("Unexpected opaque value: %q, expected %q", string(xattrOpaque), opaque)
	}
}

func checkOverlayWhiteout(t *testing.T, path string) {
	stat, err := os.Stat(path)
	assert.NilError(t, err)

	statT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("Unexpected type: %t, expected *syscall.Stat_t", stat.Sys())
	}
	if statT.Rdev != 0 {
		t.Fatalf("Non-zero device number for whiteout")
	}
}

func checkFileMode(t *testing.T, path string, perm os.FileMode) {
	stat, err := os.Stat(path)
	assert.NilError(t, err)

	if stat.Mode() != perm {
		t.Fatalf("Unexpected file mode for %s: %o, expected %o", path, stat.Mode(), perm)
	}
}

func TestOverlayTarUntar(t *testing.T) {
	restore := overrideUmask(0)
	defer restore()

	src, err := os.MkdirTemp("", "docker-test-overlay-tar-src")
	assert.NilError(t, err)
	defer os.RemoveAll(src)

	setupOverlayTestDir(t, src)

	dst, err := os.MkdirTemp("", "docker-test-overlay-tar-dst")
	assert.NilError(t, err)
	defer os.RemoveAll(dst)

	options := &TarOptions{
		Compression:    Uncompressed,
		WhiteoutFormat: OverlayWhiteoutFormat,
	}
	reader, err := TarWithOptions(src, options)
	assert.NilError(t, err)
	archive, err := io.ReadAll(reader)
	reader.Close()
	assert.NilError(t, err)

	// The archive should encode opaque directories and file whiteouts
	// in AUFS format.
	entries := make(map[string]struct{})
	rdr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := rdr.Next()
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)
		assert.Check(t, is.Equal(h.Devmajor, int64(0)), "unexpected device file in archive")
		assert.Check(t, is.DeepEqual(h.PAXRecords, map[string]string(nil), cmpopts.EquateEmpty()))
		entries[h.Name] = struct{}{}
	}

	assert.DeepEqual(t, entries, map[string]struct{}{
		"d1/":                         {},
		"d1/" + WhiteoutOpaqueDir:     {},
		"d1/f1":                       {},
		"d2/":                         {},
		"d2/" + WhiteoutOpaqueDir:     {},
		"d2/f1":                       {},
		"d3/":                         {},
		"d3/" + WhiteoutPrefix + "f1": {},
	})

	err = Untar(bytes.NewReader(archive), dst, options)
	assert.NilError(t, err)

	checkFileMode(t, filepath.Join(dst, "d1"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d2"), 0o750|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d3"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", "f1"), 0o600)
	checkFileMode(t, filepath.Join(dst, "d2", "f1"), 0o660)
	checkFileMode(t, filepath.Join(dst, "d3", "f1"), os.ModeCharDevice|os.ModeDevice)

	checkOpaqueness(t, filepath.Join(dst, "d1"), "y")
	checkOpaqueness(t, filepath.Join(dst, "d2"), "y")
	checkOpaqueness(t, filepath.Join(dst, "d3"), "")
	checkOverlayWhiteout(t, filepath.Join(dst, "d3", "f1"))
}

func TestOverlayTarAUFSUntar(t *testing.T) {
	restore := overrideUmask(0)
	defer restore()

	src, err := os.MkdirTemp("", "docker-test-overlay-tar-src")
	assert.NilError(t, err)
	defer os.RemoveAll(src)

	setupOverlayTestDir(t, src)

	dst, err := os.MkdirTemp("", "docker-test-overlay-tar-dst")
	assert.NilError(t, err)
	defer os.RemoveAll(dst)

	archive, err := TarWithOptions(src, &TarOptions{
		Compression:    Uncompressed,
		WhiteoutFormat: OverlayWhiteoutFormat,
	})
	assert.NilError(t, err)
	defer archive.Close()

	err = Untar(archive, dst, &TarOptions{
		Compression:    Uncompressed,
		WhiteoutFormat: AUFSWhiteoutFormat,
	})
	assert.NilError(t, err)

	checkFileMode(t, filepath.Join(dst, "d1"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", WhiteoutOpaqueDir), 0o700)
	checkFileMode(t, filepath.Join(dst, "d2"), 0o750|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d2", WhiteoutOpaqueDir), 0o750)
	checkFileMode(t, filepath.Join(dst, "d3"), 0o700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", "f1"), 0o600)
	checkFileMode(t, filepath.Join(dst, "d2", "f1"), 0o660)
	checkFileMode(t, filepath.Join(dst, "d3", WhiteoutPrefix+"f1"), 0o600)
}
