package safepath

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestJoinEscapingSymlink(t *testing.T) {
	type testCase struct {
		name   string
		target string
	}
	var cases []testCase

	if runtime.GOOS == "windows" {
		cases = []testCase{
			{name: "root", target: `C:\`},
			{name: "absolute file", target: `C:\Windows\System32\cmd.exe`},
		}
	} else {
		cases = []testCase{
			{name: "root", target: "/"},
			{name: "absolute file", target: "/etc/passwd"},
		}
	}
	cases = append(cases, testCase{name: "relative", target: "../../"})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dir, err := filepath.EvalSymlinks(tempDir)
			assert.NilError(t, err, "filepath.EvalSymlinks failed for temporary directory %s", tempDir)

			err = os.Symlink(tc.target, filepath.Join(dir, "link"))
			assert.NilError(t, err, "failed to create symlink to %s", tc.target)

			safe, err := Join(context.Background(), dir, "link")
			if err == nil {
				safe.Close(context.Background())
			}
			assert.ErrorType(t, err, &ErrEscapesBase{})
		})
	}
}

func TestJoinGoodSymlink(t *testing.T) {
	tempDir := t.TempDir()
	dir, err := filepath.EvalSymlinks(tempDir)
	assert.NilError(t, err, "filepath.EvalSymlinks failed for temporary directory %s", tempDir)

	assert.Assert(t, os.WriteFile(filepath.Join(dir, "foo"), []byte("bar"), 0o744), "failed to create file 'foo'")
	assert.Assert(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o744), "failed to create directory 'subdir'")
	assert.Assert(t, os.WriteFile(filepath.Join(dir, "subdir/hello.txt"), []byte("world"), 0o744), "failed to create file 'subdir/hello.txt'")

	assert.Assert(t, os.Symlink(filepath.Join(dir, "subdir"), filepath.Join(dir, "subdir_link_absolute")), "failed to create absolute symlink to directory 'subdir'")
	assert.Assert(t, os.Symlink("subdir", filepath.Join(dir, "subdir_link_relative")), "failed to create relative symlink to directory 'subdir'")

	assert.Assert(t, os.Symlink(filepath.Join(dir, "foo"), filepath.Join(dir, "foo_link_absolute")), "failed to create absolute symlink to file 'foo'")
	assert.Assert(t, os.Symlink("foo", filepath.Join(dir, "foo_link_relative")), "failed to create relative symlink to file 'foo'")

	for _, target := range []string{
		"foo", "subdir",
		"subdir_link_absolute", "foo_link_absolute",
		"subdir_link_relative", "foo_link_relative",
	} {
		t.Run(target, func(t *testing.T) {
			safe, err := Join(context.Background(), dir, target)
			assert.NilError(t, err)

			defer safe.Close(context.Background())
			if strings.HasPrefix(target, "subdir") {
				data, err := os.ReadFile(filepath.Join(safe.Path(), "hello.txt"))
				assert.NilError(t, err)
				assert.Assert(t, is.Equal(string(data), "world"))
			}
		})
	}
}

func TestJoinWithSymlinkReplace(t *testing.T) {
	tempDir := t.TempDir()
	dir, err := filepath.EvalSymlinks(tempDir)
	assert.NilError(t, err, "filepath.EvalSymlinks failed for temporary directory %s", tempDir)

	link := filepath.Join(dir, "link")
	target := filepath.Join(dir, "foo")

	err = os.WriteFile(target, []byte("bar"), 0o744)
	assert.NilError(t, err, "failed to create test file")

	err = os.Symlink(target, link)
	assert.Check(t, err, "failed to create symlink to foo")

	safe, err := Join(context.Background(), dir, "link")
	assert.NilError(t, err)

	defer safe.Close(context.Background())

	// Delete the link target.
	err = os.Remove(target)
	if runtime.GOOS == "windows" {
		// On Windows it shouldn't be possible.
		assert.Assert(t, is.ErrorType(err, &os.PathError{}), "link shouldn't be deletable before cleanup")
	} else {
		// On Linux we can delete it just fine.
		assert.NilError(t, err, "failed to remove symlink")

		// Replace target with a symlink to /etc/paswd
		err = os.Symlink("/etc/passwd", target)
		assert.NilError(t, err, "failed to create symlink")
	}

	// The returned safe path should still point to the old file.
	data, err := os.ReadFile(safe.Path())
	assert.NilError(t, err, "failed to read file")

	assert.Check(t, is.Equal(string(data), "bar"))

}

func TestJoinCloseInvalidates(t *testing.T) {
	tempDir := t.TempDir()
	dir, err := filepath.EvalSymlinks(tempDir)
	assert.NilError(t, err)

	foo := filepath.Join(dir, "foo")
	err = os.WriteFile(foo, []byte("bar"), 0o744)
	assert.NilError(t, err, "failed to create test file")

	safe, err := Join(context.Background(), dir, "foo")
	assert.NilError(t, err)

	assert.Check(t, safe.IsValid())

	assert.NilError(t, safe.Close(context.Background()))

	assert.Check(t, !safe.IsValid())
}
