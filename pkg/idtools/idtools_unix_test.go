// +build !windows

package idtools // import "github.com/docker/docker/pkg/idtools"

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/skip"
	"golang.org/x/sys/unix"
)

const (
	tempUser = "tempuser"
)

type node struct {
	uid int
	gid int
}

func TestMkdirAllAndChown(t *testing.T) {
	RequiresRoot(t)
	dirName, err := ioutil.TempDir("", "mkdirall")
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
	if err := MkdirAllAndChown(filepath.Join(dirName, "usr", "share"), 0755, IDPair{UID: 99, GID: 99}); err != nil {
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
	if err := MkdirAllAndChown(filepath.Join(dirName, "lib", "some", "other"), 0755, IDPair{UID: 101, GID: 101}); err != nil {
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
	if err := MkdirAllAndChown(filepath.Join(dirName, "usr"), 0755, IDPair{UID: 102, GID: 102}); err != nil {
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
	dirName, err := ioutil.TempDir("", "mkdirnew")
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
	err = MkdirAllAndChownNew(filepath.Join(dirName, "usr", "share"), 0755, IDPair{UID: 99, GID: 99})
	assert.NilError(t, err)

	testTree["usr/share"] = node{99, 99}
	verifyTree, err := readTree(dirName, "")
	assert.NilError(t, err)
	assert.NilError(t, compareTrees(testTree, verifyTree))

	// test 2-deep new directories--both should be owned by the uid/gid pair
	err = MkdirAllAndChownNew(filepath.Join(dirName, "lib", "some", "other"), 0755, IDPair{UID: 101, GID: 101})
	assert.NilError(t, err)
	testTree["lib/some"] = node{101, 101}
	testTree["lib/some/other"] = node{101, 101}
	verifyTree, err = readTree(dirName, "")
	assert.NilError(t, err)
	assert.NilError(t, compareTrees(testTree, verifyTree))

	// test a directory that already exists; should NOT be chowned
	err = MkdirAllAndChownNew(filepath.Join(dirName, "usr"), 0755, IDPair{UID: 102, GID: 102})
	assert.NilError(t, err)
	verifyTree, err = readTree(dirName, "")
	assert.NilError(t, err)
	assert.NilError(t, compareTrees(testTree, verifyTree))
}

func TestMkdirAndChown(t *testing.T) {
	RequiresRoot(t)
	dirName, err := ioutil.TempDir("", "mkdir")
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
	if err := MkdirAndChown(filepath.Join(dirName, "usr"), 0755, IDPair{UID: 99, GID: 99}); err != nil {
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
	if err := MkdirAndChown(filepath.Join(dirName, "usr", "bin", "subdir"), 0755, IDPair{UID: 102, GID: 102}); err == nil {
		t.Fatalf("Trying to create a directory with Mkdir where the parent doesn't exist should have failed")
	}

	// create a subdir under an existing dir; should only change the ownership of the new subdir
	if err := MkdirAndChown(filepath.Join(dirName, "usr", "bin"), 0755, IDPair{UID: 102, GID: 102}); err != nil {
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
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("Couldn't create path: %s; error: %v", fullPath, err)
		}
		if err := os.Chown(fullPath, node.uid, node.gid); err != nil {
			return fmt.Errorf("Couldn't chown path: %s; error: %v", fullPath, err)
		}
	}
	return nil
}

func readTree(base, root string) (map[string]node, error) {
	tree := make(map[string]node)

	dirInfos, err := ioutil.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read directory entries for %q: %v", base, err)
	}

	for _, info := range dirInfos {
		s := &unix.Stat_t{}
		if err := unix.Stat(filepath.Join(base, info.Name()), s); err != nil {
			return nil, fmt.Errorf("Can't stat file %q: %v", filepath.Join(base, info.Name()), err)
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
		return fmt.Errorf("Trees aren't the same size")
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

func delUser(t *testing.T, name string) {
	_, err := execCmd("userdel", name)
	assert.Check(t, err)
}

func TestParseSubidFileWithNewlinesAndComments(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "parsesubid")
	if err != nil {
		t.Fatal(err)
	}
	fnamePath := filepath.Join(tmpDir, "testsubuid")
	fcontent := `tss:100000:65536
# empty default subuid/subgid file

dockremap:231072:65536`
	if err := ioutil.WriteFile(fnamePath, []byte(fcontent), 0644); err != nil {
		t.Fatal(err)
	}
	ranges, err := parseSubidFile(fnamePath, "dockremap")
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 {
		t.Fatalf("wanted 1 element in ranges, got %d instead", len(ranges))
	}
	if ranges[0].Start != 231072 {
		t.Fatalf("wanted 231072, got %d instead", ranges[0].Start)
	}
	if ranges[0].Length != 65536 {
		t.Fatalf("wanted 65536, got %d instead", ranges[0].Length)
	}
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
	assert.Check(t, is.Equal(os.Getegid(), uid))
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

func TestNewIDMappings(t *testing.T) {
	RequiresRoot(t)
	_, _, err := AddNamespaceRangesUser(tempUser)
	assert.Check(t, err)
	defer delUser(t, tempUser)

	tempUser, err := user.Lookup(tempUser)
	assert.Check(t, err)

	gids, err := tempUser.GroupIds()
	assert.Check(t, err)
	group, err := user.LookupGroupId(string(gids[0]))
	assert.Check(t, err)

	idMappings, err := NewIDMappings(tempUser.Username, group.Name)
	assert.Check(t, err)

	rootUID, rootGID, err := GetRootUIDGID(idMappings.UIDs(), idMappings.GIDs())
	assert.Check(t, err)

	dirName, err := ioutil.TempDir("", "mkdirall")
	assert.Check(t, err, "Couldn't create temp directory")
	defer os.RemoveAll(dirName)

	err = MkdirAllAndChown(dirName, 0700, IDPair{UID: rootUID, GID: rootGID})
	assert.Check(t, err, "Couldn't change ownership of file path. Got error")
	assert.Check(t, CanAccess(dirName, idMappings.RootPair()), fmt.Sprintf("Unable to access %s directory with user UID:%d and GID:%d", dirName, rootUID, rootGID))
}

func TestLookupUserAndGroup(t *testing.T) {
	RequiresRoot(t)
	uid, gid, err := AddNamespaceRangesUser(tempUser)
	assert.Check(t, err)
	defer delUser(t, tempUser)

	fetchedUser, err := LookupUser(tempUser)
	assert.Check(t, err)

	fetchedUserByID, err := LookupUID(uid)
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(fetchedUserByID, fetchedUser))

	fetchedGroup, err := LookupGroup(tempUser)
	assert.Check(t, err)

	fetchedGroupByID, err := LookupGID(gid)
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(fetchedGroupByID, fetchedGroup))
}

func TestLookupUserAndGroupThatDoesNotExist(t *testing.T) {
	fakeUser := "fakeuser"
	_, err := LookupUser(fakeUser)
	assert.Check(t, is.Error(err, "getent unable to find entry \""+fakeUser+"\" in passwd database"))

	_, err = LookupUID(-1)
	assert.Check(t, is.ErrorContains(err, ""))

	fakeGroup := "fakegroup"
	_, err = LookupGroup(fakeGroup)
	assert.Check(t, is.Error(err, "getent unable to find entry \""+fakeGroup+"\" in group database"))

	_, err = LookupGID(-1)
	assert.Check(t, is.ErrorContains(err, ""))
}

// TestMkdirIsNotDir checks that mkdirAs() function (used by MkdirAll...)
// returns a correct error in case a directory which it is about to create
// already exists but is a file (rather than a directory).
func TestMkdirIsNotDir(t *testing.T) {
	file, err := ioutil.TempFile("", t.Name())
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}
	defer os.Remove(file.Name())

	err = mkdirAs(file.Name(), 0755, 0, 0, false, false)
	assert.Check(t, is.Error(err, "mkdir "+file.Name()+": not a directory"))
}

func RequiresRoot(t *testing.T) {
	skip.IfCondition(t, os.Getuid() != 0, "skipping test that requires root")
}
