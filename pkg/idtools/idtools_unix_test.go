//go:build !windows

package idtools

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

type node struct {
	uid int
	gid int
}

func TestMkdirAllAndChown(t *testing.T) {
	RequiresRoot(t)
	dirName, err := os.MkdirTemp("", "mkdirall")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(dirName)

	testTree := map[string]node{
		"usr":              {0, 0},
		"usr/bin":          {0, 0},
		"lib":              {33, 33},
		"lib/x86_64":       {45, 45},
		"lib/x86_64/share": {1, 1},
	}

	if err := buildTree(dirName, testTree); err != nil {
		t.Fatal(err)
	}

	// test adding a directory to a pre-existing dir; only the new dir is owned by the uid/gid
	if err := MkdirAllAndChown(filepath.Join(dirName, "usr", "share"), 0o755, Identity{UID: 99, GID: 99}); err != nil {
		t.Fatal(err)
	}
	testTree["usr/share"] = node{99, 99}
	verifyTree, err := readTree(dirName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := compareTrees(testTree, verifyTree); err != nil {
		t.Fatal(err)
	}

	// test 2-deep new directories--both should be owned by the uid/gid pair
	if err := MkdirAllAndChown(filepath.Join(dirName, "lib", "some", "other"), 0o755, Identity{UID: 101, GID: 101}); err != nil {
		t.Fatal(err)
	}
	testTree["lib/some"] = node{101, 101}
	testTree["lib/some/other"] = node{101, 101}
	verifyTree, err = readTree(dirName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := compareTrees(testTree, verifyTree); err != nil {
		t.Fatal(err)
	}

	// test a directory that already exists; should be chowned, but nothing else
	if err := MkdirAllAndChown(filepath.Join(dirName, "usr"), 0o755, Identity{UID: 102, GID: 102}); err != nil {
		t.Fatal(err)
	}
	testTree["usr"] = node{102, 102}
	verifyTree, err = readTree(dirName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := compareTrees(testTree, verifyTree); err != nil {
		t.Fatal(err)
	}
}

func TestMkdirAllAndChownNew(t *testing.T) {
	RequiresRoot(t)
	dirName, err := os.MkdirTemp("", "mkdirnew")
	assert.NilError(t, err)
	defer os.RemoveAll(dirName)

	testTree := map[string]node{
		"usr":              {0, 0},
		"usr/bin":          {0, 0},
		"lib":              {33, 33},
		"lib/x86_64":       {45, 45},
		"lib/x86_64/share": {1, 1},
	}
	assert.NilError(t, buildTree(dirName, testTree))

	// test adding a directory to a pre-existing dir; only the new dir is owned by the uid/gid
	err = MkdirAllAndChownNew(filepath.Join(dirName, "usr", "share"), 0o755, Identity{UID: 99, GID: 99})
	assert.NilError(t, err)

	testTree["usr/share"] = node{99, 99}
	verifyTree, err := readTree(dirName, "")
	assert.NilError(t, err)
	assert.NilError(t, compareTrees(testTree, verifyTree))

	// test 2-deep new directories--both should be owned by the uid/gid pair
	err = MkdirAllAndChownNew(filepath.Join(dirName, "lib", "some", "other"), 0o755, Identity{UID: 101, GID: 101})
	assert.NilError(t, err)
	testTree["lib/some"] = node{101, 101}
	testTree["lib/some/other"] = node{101, 101}
	verifyTree, err = readTree(dirName, "")
	assert.NilError(t, err)
	assert.NilError(t, compareTrees(testTree, verifyTree))

	// test a directory that already exists; should NOT be chowned
	err = MkdirAllAndChownNew(filepath.Join(dirName, "usr"), 0o755, Identity{UID: 102, GID: 102})
	assert.NilError(t, err)
	verifyTree, err = readTree(dirName, "")
	assert.NilError(t, err)
	assert.NilError(t, compareTrees(testTree, verifyTree))
}

func TestMkdirAllAndChownNewRelative(t *testing.T) {
	RequiresRoot(t)

	tests := []struct {
		in  string
		out []string
	}{
		{
			in:  "dir1",
			out: []string{"dir1"},
		},
		{
			in:  "dir2/subdir2",
			out: []string{"dir2", "dir2/subdir2"},
		},
		{
			in:  "dir3/subdir3/",
			out: []string{"dir3", "dir3/subdir3"},
		},
		{
			in:  "dir4/subdir4/.",
			out: []string{"dir4", "dir4/subdir4"},
		},
		{
			in:  "dir5/././subdir5/",
			out: []string{"dir5", "dir5/subdir5"},
		},
		{
			in:  "./dir6",
			out: []string{"dir6"},
		},
		{
			in:  "./dir7/subdir7",
			out: []string{"dir7", "dir7/subdir7"},
		},
		{
			in:  "./dir8/subdir8/",
			out: []string{"dir8", "dir8/subdir8"},
		},
		{
			in:  "./dir9/subdir9/.",
			out: []string{"dir9", "dir9/subdir9"},
		},
		{
			in:  "./dir10/././subdir10/",
			out: []string{"dir10", "dir10/subdir10"},
		},
	}

	// Set the current working directory to the temp-dir, as we're
	// testing relative paths.
	tmpDir := t.TempDir()
	setWorkingDirectory(t, tmpDir)

	const expectedUIDGID = 101

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			for _, p := range tc.out {
				_, err := os.Stat(p)
				assert.ErrorIs(t, err, os.ErrNotExist)
			}

			err := MkdirAllAndChownNew(tc.in, 0o755, Identity{UID: expectedUIDGID, GID: expectedUIDGID})
			assert.Check(t, err)

			for _, p := range tc.out {
				s := &unix.Stat_t{}
				err = unix.Stat(p, s)
				if assert.Check(t, err) {
					assert.Check(t, is.Equal(uint64(s.Uid), uint64(expectedUIDGID)))
					assert.Check(t, is.Equal(uint64(s.Gid), uint64(expectedUIDGID)))
				}
			}
		})
	}
}

// Change the current working directory for the duration of the test. This may
// break if tests are run in parallel.
func setWorkingDirectory(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	assert.NilError(t, err)
	t.Cleanup(func() {
		assert.NilError(t, os.Chdir(cwd))
	})
	err = os.Chdir(dir)
	assert.NilError(t, err)
}

func TestMkdirAndChown(t *testing.T) {
	RequiresRoot(t)
	dirName, err := os.MkdirTemp("", "mkdir")
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.RemoveAll(dirName)

	testTree := map[string]node{
		"usr": {0, 0},
	}
	if err := buildTree(dirName, testTree); err != nil {
		t.Fatal(err)
	}

	// test a directory that already exists; should just chown to the requested uid/gid
	if err := MkdirAndChown(filepath.Join(dirName, "usr"), 0o755, Identity{UID: 99, GID: 99}); err != nil {
		t.Fatal(err)
	}
	testTree["usr"] = node{99, 99}
	verifyTree, err := readTree(dirName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := compareTrees(testTree, verifyTree); err != nil {
		t.Fatal(err)
	}

	// create a subdir under a dir which doesn't exist--should fail
	if err := MkdirAndChown(filepath.Join(dirName, "usr", "bin", "subdir"), 0o755, Identity{UID: 102, GID: 102}); err == nil {
		t.Fatalf("Trying to create a directory with Mkdir where the parent doesn't exist should have failed")
	}

	// create a subdir under an existing dir; should only change the ownership of the new subdir
	if err := MkdirAndChown(filepath.Join(dirName, "usr", "bin"), 0o755, Identity{UID: 102, GID: 102}); err != nil {
		t.Fatal(err)
	}
	testTree["usr/bin"] = node{102, 102}
	verifyTree, err = readTree(dirName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := compareTrees(testTree, verifyTree); err != nil {
		t.Fatal(err)
	}
}

func buildTree(base string, tree map[string]node) error {
	for path, node := range tree {
		fullPath := filepath.Join(base, path)
		if err := os.MkdirAll(fullPath, 0o755); err != nil {
			return fmt.Errorf("couldn't create path: %s; error: %v", fullPath, err)
		}
		if err := os.Chown(fullPath, node.uid, node.gid); err != nil {
			return fmt.Errorf("couldn't chown path: %s; error: %v", fullPath, err)
		}
	}
	return nil
}

func readTree(base, root string) (map[string]node, error) {
	tree := make(map[string]node)

	dirInfos, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("couldn't read directory entries for %q: %v", base, err)
	}

	for _, info := range dirInfos {
		s := &unix.Stat_t{}
		if err := unix.Stat(filepath.Join(base, info.Name()), s); err != nil {
			return nil, fmt.Errorf("can't stat file %q: %v", filepath.Join(base, info.Name()), err)
		}
		tree[filepath.Join(root, info.Name())] = node{int(s.Uid), int(s.Gid)}
		if info.IsDir() {
			// read the subdirectory
			subtree, err := readTree(filepath.Join(base, info.Name()), filepath.Join(root, info.Name()))
			if err != nil {
				return nil, err
			}
			for path, nodeinfo := range subtree {
				tree[path] = nodeinfo
			}
		}
	}
	return tree, nil
}

func compareTrees(left, right map[string]node) error {
	if len(left) != len(right) {
		return fmt.Errorf("trees aren't the same size")
	}
	for path, nodeLeft := range left {
		if nodeRight, ok := right[path]; ok {
			if nodeRight.uid != nodeLeft.uid || nodeRight.gid != nodeLeft.gid {
				// mismatch
				return fmt.Errorf("mismatched ownership for %q: expected: %d:%d, got: %d:%d", path,
					nodeLeft.uid, nodeLeft.gid, nodeRight.uid, nodeRight.gid)
			}
			continue
		}
		return fmt.Errorf("right tree didn't contain path %q", path)
	}
	return nil
}

func TestGetRootUIDGID(t *testing.T) {
	uidMap := []IDMap{
		{
			ContainerID: 0,
			HostID:      os.Getuid(),
			Size:        1,
		},
	}
	gidMap := []IDMap{
		{
			ContainerID: 0,
			HostID:      os.Getgid(),
			Size:        1,
		},
	}

	uid, gid, err := GetRootUIDGID(uidMap, gidMap)
	assert.Check(t, err)
	assert.Check(t, is.Equal(os.Geteuid(), uid))
	assert.Check(t, is.Equal(os.Getegid(), gid))

	uidMapError := []IDMap{
		{
			ContainerID: 1,
			HostID:      os.Getuid(),
			Size:        1,
		},
	}
	_, _, err = GetRootUIDGID(uidMapError, gidMap)
	assert.Check(t, is.Error(err, "Container ID 0 cannot be mapped to a host ID"))
}

func TestToContainer(t *testing.T) {
	uidMap := []IDMap{
		{
			ContainerID: 2,
			HostID:      2,
			Size:        1,
		},
	}

	containerID, err := toContainer(2, uidMap)
	assert.Check(t, err)
	assert.Check(t, is.Equal(uidMap[0].ContainerID, containerID))
}

// TestMkdirIsNotDir checks that mkdirAs() function (used by MkdirAll...)
// returns a correct error in case a directory which it is about to create
// already exists but is a file (rather than a directory).
func TestMkdirIsNotDir(t *testing.T) {
	file, err := os.CreateTemp("", t.Name())
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.Remove(file.Name())

	err = mkdirAs(file.Name(), 0o755, Identity{UID: 0, GID: 0}, false, false)
	assert.Check(t, is.Error(err, "mkdir "+file.Name()+": not a directory"))
}

func RequiresRoot(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
}
