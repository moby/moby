//go:build windows

package container

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAssertsDirectory(t *testing.T) {
	sep := string(filepath.Separator)
	for _, tc := range []struct {
		path     string
		expected bool
	}{
		{path: "", expected: false},
		{path: `\foo`, expected: false},
		{path: `\foo\bar`, expected: false},
		{path: `\foo\`, expected: true},
		{path: sep, expected: true},
		{path: `\foo\.`, expected: true},
		{path: `.`, expected: true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assert.Check(t, is.Equal(assertsDirectory(tc.path), tc.expected))
		})
	}
}

// TestStatPathNotADirectory verifies that StatPath rejects a path that asserts a
// directory (trailing separator or "." component) but resolves to a file, and
// accepts it otherwise. This is the Windows-specific invariant added for
// moby/moby#47107 (the containerd snapshotter mounts the rootfs at a regular
// path where os.Lstat does not reject a trailing separator on a file).
//
// The resolvedPath argument is passed without a trailing separator so that
// os.Lstat succeeds regardless of the underlying filesystem, isolating the
// assertsDirectory check from mount-specific os.Lstat behavior.
func TestStatPathNotADirectory(t *testing.T) {
	baseFS := t.TempDir()
	filePath := filepath.Join(baseFS, "afile")
	assert.NilError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	subDir := filepath.Join(baseFS, "adir")
	assert.NilError(t, os.Mkdir(subDir, 0o755))

	ctr := &Container{ID: "test", BaseFS: baseFS}

	t.Run("file with trailing separator errors", func(t *testing.T) {
		_, err := ctr.StatPath(filePath, `\afile\`)
		assert.Check(t, is.ErrorContains(err, "not a directory"))
	})

	t.Run("file with trailing dot errors", func(t *testing.T) {
		_, err := ctr.StatPath(filePath, `\afile\.`)
		assert.Check(t, is.ErrorContains(err, "not a directory"))
	})

	t.Run("file without trailing separator is ok", func(t *testing.T) {
		st, err := ctr.StatPath(filePath, `\afile`)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(st.Mode.IsDir(), false))
	})

	t.Run("directory with trailing separator is ok", func(t *testing.T) {
		st, err := ctr.StatPath(subDir, `\adir\`)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(st.Mode.IsDir(), true))
	})

	t.Run("empty BaseFS errors", func(t *testing.T) {
		emptyCtr := &Container{ID: "test"}
		_, err := emptyCtr.StatPath(filePath, `\afile\`)
		assert.Check(t, is.ErrorContains(err, "unexpectedly empty"))
	})
}
