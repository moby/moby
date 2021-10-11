package archive // import "github.com/docker/docker/pkg/archive"

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/system"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func max(x, y int) int {
	if x >= y {
		return x
	}
	return y
}

func copyDir(src, dst string) error {
	if runtime.GOOS != "windows" {
		return exec.Command("cp", "-a", src, dst).Run()
	}

	// Could have used xcopy src dst /E /I /H /Y /B. However, xcopy has the
	// unfortunate side effect of not preserving timestamps of newly created
	// directories in the target directory, so we don't get accurate changes.
	// Use robocopy instead. Note this isn't available in microsoft/nanoserver.
	// But it has gotchas. See https://weblogs.sqlteam.com/robv/archive/2010/02/17/61106.aspx
	err := exec.Command("robocopy", filepath.FromSlash(src), filepath.FromSlash(dst), "/SL", "/COPYALL", "/MIR").Run()
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			if status.ExitStatus()&24 == 0 {
				return nil
			}
		}
	}
	return err
}

type FileType uint32

const (
	Regular FileType = iota
	Dir
	Symlink
)

type FileData struct {
	filetype    FileType
	path        string
	contents    string
	permissions os.FileMode
}

func createSampleDir(t *testing.T, root string) {
	files := []FileData{
		{filetype: Regular, path: "file1", contents: "file1\n", permissions: 0600},
		{filetype: Regular, path: "file2", contents: "file2\n", permissions: 0666},
		{filetype: Regular, path: "file3", contents: "file3\n", permissions: 0404},
		{filetype: Regular, path: "file4", contents: "file4\n", permissions: 0600},
		{filetype: Regular, path: "file5", contents: "file5\n", permissions: 0600},
		{filetype: Regular, path: "file6", contents: "file6\n", permissions: 0600},
		{filetype: Regular, path: "file7", contents: "file7\n", permissions: 0600},
		{filetype: Dir, path: "dir1", contents: "", permissions: 0740},
		{filetype: Regular, path: "dir1/file1-1", contents: "file1-1\n", permissions: 01444},
		{filetype: Regular, path: "dir1/file1-2", contents: "file1-2\n", permissions: 0666},
		{filetype: Dir, path: "dir2", contents: "", permissions: 0700},
		{filetype: Regular, path: "dir2/file2-1", contents: "file2-1\n", permissions: 0666},
		{filetype: Regular, path: "dir2/file2-2", contents: "file2-2\n", permissions: 0666},
		{filetype: Dir, path: "dir3", contents: "", permissions: 0700},
		{filetype: Regular, path: "dir3/file3-1", contents: "file3-1\n", permissions: 0666},
		{filetype: Regular, path: "dir3/file3-2", contents: "file3-2\n", permissions: 0666},
		{filetype: Dir, path: "dir4", contents: "", permissions: 0700},
		{filetype: Regular, path: "dir4/file3-1", contents: "file4-1\n", permissions: 0666},
		{filetype: Regular, path: "dir4/file3-2", contents: "file4-2\n", permissions: 0666},
		{filetype: Symlink, path: "symlink1", contents: "target1", permissions: 0666},
		{filetype: Symlink, path: "symlink2", contents: "target2", permissions: 0666},
		{filetype: Symlink, path: "symlink3", contents: root + "/file1", permissions: 0666},
		{filetype: Symlink, path: "symlink4", contents: root + "/symlink3", permissions: 0666},
		{filetype: Symlink, path: "dirSymlink", contents: root + "/dir1", permissions: 0740},
	}
	provisionSampleDir(t, root, files)
}

func provisionSampleDir(t *testing.T, root string, files []FileData) {
	now := time.Now()
	for _, info := range files {
		p := path.Join(root, info.path)
		if info.filetype == Dir {
			err := os.MkdirAll(p, info.permissions)
			assert.NilError(t, err)
		} else if info.filetype == Regular {
			err := os.WriteFile(p, []byte(info.contents), info.permissions)
			assert.NilError(t, err)
		} else if info.filetype == Symlink {
			err := os.Symlink(info.contents, p)
			assert.NilError(t, err)
		}

		if info.filetype != Symlink {
			// Set a consistent ctime, atime for all files and dirs
			err := system.Chtimes(p, now, now)
			assert.NilError(t, err)
		}
	}
}

func TestChangeString(t *testing.T) {
	modifyChange := Change{"change", ChangeModify}
	toString := modifyChange.String()
	if toString != "C change" {
		t.Fatalf("String() of a change with ChangeModify Kind should have been %s but was %s", "C change", toString)
	}
	addChange := Change{"change", ChangeAdd}
	toString = addChange.String()
	if toString != "A change" {
		t.Fatalf("String() of a change with ChangeAdd Kind should have been %s but was %s", "A change", toString)
	}
	deleteChange := Change{"change", ChangeDelete}
	toString = deleteChange.String()
	if toString != "D change" {
		t.Fatalf("String() of a change with ChangeDelete Kind should have been %s but was %s", "D change", toString)
	}
}

func TestChangesWithNoChanges(t *testing.T) {
	rwLayer, err := os.MkdirTemp("", "docker-changes-test")
	assert.NilError(t, err)
	defer os.RemoveAll(rwLayer)
	layer, err := os.MkdirTemp("", "docker-changes-test-layer")
	assert.NilError(t, err)
	defer os.RemoveAll(layer)
	createSampleDir(t, layer)
	changes, err := Changes([]string{layer}, rwLayer)
	assert.NilError(t, err)
	if len(changes) != 0 {
		t.Fatalf("Changes with no difference should have detect no changes, but detected %d", len(changes))
	}
}

func TestChangesWithChanges(t *testing.T) {
	// Mock the readonly layer
	layer, err := os.MkdirTemp("", "docker-changes-test-layer")
	assert.NilError(t, err)
	defer os.RemoveAll(layer)
	createSampleDir(t, layer)
	os.MkdirAll(path.Join(layer, "dir1/subfolder"), 0740)

	// Mock the RW layer
	rwLayer, err := os.MkdirTemp("", "docker-changes-test")
	assert.NilError(t, err)
	defer os.RemoveAll(rwLayer)

	// Create a folder in RW layer
	dir1 := path.Join(rwLayer, "dir1")
	os.MkdirAll(dir1, 0740)
	deletedFile := path.Join(dir1, ".wh.file1-2")
	os.WriteFile(deletedFile, []byte{}, 0600)
	modifiedFile := path.Join(dir1, "file1-1")
	os.WriteFile(modifiedFile, []byte{0x00}, 01444)
	// Let's add a subfolder for a newFile
	subfolder := path.Join(dir1, "subfolder")
	os.MkdirAll(subfolder, 0740)
	newFile := path.Join(subfolder, "newFile")
	os.WriteFile(newFile, []byte{}, 0740)

	changes, err := Changes([]string{layer}, rwLayer)
	assert.NilError(t, err)

	expectedChanges := []Change{
		{filepath.FromSlash("/dir1"), ChangeModify},
		{filepath.FromSlash("/dir1/file1-1"), ChangeModify},
		{filepath.FromSlash("/dir1/file1-2"), ChangeDelete},
		{filepath.FromSlash("/dir1/subfolder"), ChangeModify},
		{filepath.FromSlash("/dir1/subfolder/newFile"), ChangeAdd},
	}
	checkChanges(expectedChanges, changes, t)
}

// See https://github.com/docker/docker/pull/13590
func TestChangesWithChangesGH13590(t *testing.T) {
	// TODO Windows. Needs further investigation to identify the failure
	if runtime.GOOS == "windows" {
		t.Skip("needs more investigation")
	}
	baseLayer, err := os.MkdirTemp("", "docker-changes-test.")
	assert.NilError(t, err)
	defer os.RemoveAll(baseLayer)

	dir3 := path.Join(baseLayer, "dir1/dir2/dir3")
	os.MkdirAll(dir3, 07400)

	file := path.Join(dir3, "file.txt")
	os.WriteFile(file, []byte("hello"), 0666)

	layer, err := os.MkdirTemp("", "docker-changes-test2.")
	assert.NilError(t, err)
	defer os.RemoveAll(layer)

	// Test creating a new file
	if err := copyDir(baseLayer+"/dir1", layer+"/"); err != nil {
		t.Fatalf("Cmd failed: %q", err)
	}

	os.Remove(path.Join(layer, "dir1/dir2/dir3/file.txt"))
	file = path.Join(layer, "dir1/dir2/dir3/file1.txt")
	os.WriteFile(file, []byte("bye"), 0666)

	changes, err := Changes([]string{baseLayer}, layer)
	assert.NilError(t, err)

	expectedChanges := []Change{
		{"/dir1/dir2/dir3", ChangeModify},
		{"/dir1/dir2/dir3/file1.txt", ChangeAdd},
	}
	checkChanges(expectedChanges, changes, t)

	// Now test changing a file
	layer, err = os.MkdirTemp("", "docker-changes-test3.")
	assert.NilError(t, err)
	defer os.RemoveAll(layer)

	if err := copyDir(baseLayer+"/dir1", layer+"/"); err != nil {
		t.Fatalf("Cmd failed: %q", err)
	}

	file = path.Join(layer, "dir1/dir2/dir3/file.txt")
	os.WriteFile(file, []byte("bye"), 0666)

	changes, err = Changes([]string{baseLayer}, layer)
	assert.NilError(t, err)

	expectedChanges = []Change{
		{"/dir1/dir2/dir3/file.txt", ChangeModify},
	}
	checkChanges(expectedChanges, changes, t)
}

// Create a directory, copy it, make sure we report no changes between the two
func TestChangesDirsEmpty(t *testing.T) {
	// Note we parse kernel.GetKernelVersion rather than system.GetOSVersion
	// as test binaries aren't manifested, so would otherwise report the wrong
	// build number.
	if runtime.GOOS == "windows" {
		v, err := kernel.GetKernelVersion()
		assert.NilError(t, err)
		build, _ := strconv.Atoi(strings.Split(strings.SplitN(v.String(), " ", 3)[2][1:], ".")[0])
		if build >= osversion.V19H1 {
			t.Skip("FIXME: broken on Windows 1903 and up; see #39846")
		}
	}

	src, err := os.MkdirTemp("", "docker-changes-test")
	assert.NilError(t, err)
	defer os.RemoveAll(src)
	createSampleDir(t, src)
	dst := src + "-copy"
	err = copyDir(src, dst)
	assert.NilError(t, err)
	defer os.RemoveAll(dst)
	changes, err := ChangesDirs(dst, src)
	assert.NilError(t, err)

	if len(changes) != 0 {
		t.Fatalf("Reported changes for identical dirs: %v", changes)
	}
	os.RemoveAll(src)
	os.RemoveAll(dst)
}

func mutateSampleDir(t *testing.T, root string) {
	// Remove a regular file
	err := os.RemoveAll(path.Join(root, "file1"))
	assert.NilError(t, err)

	// Remove a directory
	err = os.RemoveAll(path.Join(root, "dir1"))
	assert.NilError(t, err)

	// Remove a symlink
	err = os.RemoveAll(path.Join(root, "symlink1"))
	assert.NilError(t, err)

	// Rewrite a file
	err = os.WriteFile(path.Join(root, "file2"), []byte("fileNN\n"), 0777)
	assert.NilError(t, err)

	// Replace a file
	err = os.RemoveAll(path.Join(root, "file3"))
	assert.NilError(t, err)
	err = os.WriteFile(path.Join(root, "file3"), []byte("fileMM\n"), 0404)
	assert.NilError(t, err)

	// Touch file
	err = system.Chtimes(path.Join(root, "file4"), time.Now().Add(time.Second), time.Now().Add(time.Second))
	assert.NilError(t, err)

	// Replace file with dir
	err = os.RemoveAll(path.Join(root, "file5"))
	assert.NilError(t, err)
	err = os.MkdirAll(path.Join(root, "file5"), 0666)
	assert.NilError(t, err)

	// Create new file
	err = os.WriteFile(path.Join(root, "filenew"), []byte("filenew\n"), 0777)
	assert.NilError(t, err)

	// Create new dir
	err = os.MkdirAll(path.Join(root, "dirnew"), 0766)
	assert.NilError(t, err)

	// Create a new symlink
	err = os.Symlink("targetnew", path.Join(root, "symlinknew"))
	assert.NilError(t, err)

	// Change a symlink
	err = os.RemoveAll(path.Join(root, "symlink2"))
	assert.NilError(t, err)

	err = os.Symlink("target2change", path.Join(root, "symlink2"))
	assert.NilError(t, err)

	// Replace dir with file
	err = os.RemoveAll(path.Join(root, "dir2"))
	assert.NilError(t, err)
	err = os.WriteFile(path.Join(root, "dir2"), []byte("dir2\n"), 0777)
	assert.NilError(t, err)

	// Touch dir
	err = system.Chtimes(path.Join(root, "dir3"), time.Now().Add(time.Second), time.Now().Add(time.Second))
	assert.NilError(t, err)
}

func TestChangesDirsMutated(t *testing.T) {
	// Note we parse kernel.GetKernelVersion rather than system.GetOSVersion
	// as test binaries aren't manifested, so would otherwise report the wrong
	// build number.
	if runtime.GOOS == "windows" {
		v, err := kernel.GetKernelVersion()
		assert.NilError(t, err)
		build, _ := strconv.Atoi(strings.Split(strings.SplitN(v.String(), " ", 3)[2][1:], ".")[0])
		if build >= osversion.V19H1 {
			t.Skip("FIXME: broken on Windows 1903 and up; see #39846")
		}
	}

	src, err := os.MkdirTemp("", "docker-changes-test")
	assert.NilError(t, err)
	createSampleDir(t, src)
	dst := src + "-copy"
	err = copyDir(src, dst)
	assert.NilError(t, err)
	defer os.RemoveAll(src)
	defer os.RemoveAll(dst)

	mutateSampleDir(t, dst)

	changes, err := ChangesDirs(dst, src)
	assert.NilError(t, err)

	sort.Sort(changesByPath(changes))

	expectedChanges := []Change{
		{filepath.FromSlash("/dir1"), ChangeDelete},
		{filepath.FromSlash("/dir2"), ChangeModify},
	}

	// Note there is slight difference between the Linux and Windows
	// implementations here. Due to https://github.com/moby/moby/issues/9874,
	// and the fix at https://github.com/moby/moby/pull/11422, Linux does not
	// consider a change to the directory time as a change. Windows on NTFS
	// does. See https://github.com/moby/moby/pull/37982 for more information.
	//
	// Note also: https://github.com/moby/moby/pull/37982#discussion_r223523114
	// that differences are ordered in the way the test is currently written, hence
	// this is in the middle of the list of changes rather than at the start or
	// end. Potentially can be addressed later.
	if runtime.GOOS == "windows" {
		expectedChanges = append(expectedChanges, Change{filepath.FromSlash("/dir3"), ChangeModify})
	}

	expectedChanges = append(expectedChanges, []Change{
		{filepath.FromSlash("/dirnew"), ChangeAdd},
		{filepath.FromSlash("/file1"), ChangeDelete},
		{filepath.FromSlash("/file2"), ChangeModify},
		{filepath.FromSlash("/file3"), ChangeModify},
		{filepath.FromSlash("/file4"), ChangeModify},
		{filepath.FromSlash("/file5"), ChangeModify},
		{filepath.FromSlash("/filenew"), ChangeAdd},
		{filepath.FromSlash("/symlink1"), ChangeDelete},
		{filepath.FromSlash("/symlink2"), ChangeModify},
		{filepath.FromSlash("/symlinknew"), ChangeAdd},
	}...)

	for i := 0; i < max(len(changes), len(expectedChanges)); i++ {
		if i >= len(expectedChanges) {
			t.Fatalf("unexpected change %s\n", changes[i].String())
		}
		if i >= len(changes) {
			t.Fatalf("no change for expected change %s\n", expectedChanges[i].String())
		}
		if changes[i].Path == expectedChanges[i].Path {
			if changes[i] != expectedChanges[i] {
				t.Fatalf("Wrong change for %s, expected %s, got %s\n", changes[i].Path, changes[i].String(), expectedChanges[i].String())
			}
		} else if changes[i].Path < expectedChanges[i].Path {
			t.Fatalf("unexpected change %q %q\n", changes[i].String(), expectedChanges[i].Path)
		} else {
			t.Fatalf("no change for expected change %s != %s\n", expectedChanges[i].String(), changes[i].String())
		}
	}
}

func TestApplyLayer(t *testing.T) {
	// TODO Windows. This is very close to working, but it fails with changes
	// to \symlinknew and \symlink2. The destination has an updated
	// Access/Modify/Change/Birth date to the source (~3/100th sec different).
	// Needs further investigation as to why, but I currently believe this is
	// just the way NTFS works. I don't think it's a bug in this test or archive.
	if runtime.GOOS == "windows" {
		t.Skip("needs further investigation")
	}
	src, err := os.MkdirTemp("", "docker-changes-test")
	assert.NilError(t, err)
	createSampleDir(t, src)
	defer os.RemoveAll(src)
	dst := src + "-copy"
	err = copyDir(src, dst)
	assert.NilError(t, err)
	mutateSampleDir(t, dst)
	defer os.RemoveAll(dst)

	changes, err := ChangesDirs(dst, src)
	assert.NilError(t, err)

	layer, err := ExportChanges(dst, changes, nil, nil)
	assert.NilError(t, err)

	layerCopy, err := NewTempArchive(layer, "")
	assert.NilError(t, err)

	_, err = ApplyLayer(src, layerCopy)
	assert.NilError(t, err)

	changes2, err := ChangesDirs(src, dst)
	assert.NilError(t, err)

	if len(changes2) != 0 {
		t.Fatalf("Unexpected differences after reapplying mutation: %v", changes2)
	}
}

func TestChangesSizeWithHardlinks(t *testing.T) {
	// TODO Windows. Needs further investigation. Likely in ChangeSizes not
	// coping correctly with hardlinks on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("needs further investigation")
	}
	srcDir, err := os.MkdirTemp("", "docker-test-srcDir")
	assert.NilError(t, err)
	defer os.RemoveAll(srcDir)

	destDir, err := os.MkdirTemp("", "docker-test-destDir")
	assert.NilError(t, err)
	defer os.RemoveAll(destDir)

	creationSize, err := prepareUntarSourceDirectory(100, destDir, true)
	assert.NilError(t, err)

	changes, err := ChangesDirs(destDir, srcDir)
	assert.NilError(t, err)

	got := ChangesSize(destDir, changes)
	if got != int64(creationSize) {
		t.Errorf("Expected %d bytes of changes, got %d", creationSize, got)
	}
}

func TestChangesSizeWithNoChanges(t *testing.T) {
	size := ChangesSize("/tmp", nil)
	if size != 0 {
		t.Fatalf("ChangesSizes with no changes should be 0, was %d", size)
	}
}

func TestChangesSizeWithOnlyDeleteChanges(t *testing.T) {
	changes := []Change{
		{Path: "deletedPath", Kind: ChangeDelete},
	}
	size := ChangesSize("/tmp", changes)
	if size != 0 {
		t.Fatalf("ChangesSizes with only delete changes should be 0, was %d", size)
	}
}

func TestChangesSize(t *testing.T) {
	parentPath, err := os.MkdirTemp("", "docker-changes-test")
	assert.NilError(t, err)
	defer os.RemoveAll(parentPath)
	addition := path.Join(parentPath, "addition")
	err = os.WriteFile(addition, []byte{0x01, 0x01, 0x01}, 0744)
	assert.NilError(t, err)
	modification := path.Join(parentPath, "modification")
	err = os.WriteFile(modification, []byte{0x01, 0x01, 0x01}, 0744)
	assert.NilError(t, err)

	changes := []Change{
		{Path: "addition", Kind: ChangeAdd},
		{Path: "modification", Kind: ChangeModify},
	}
	size := ChangesSize(parentPath, changes)
	if size != 6 {
		t.Fatalf("Expected 6 bytes of changes, got %d", size)
	}
}

func checkChanges(expectedChanges, changes []Change, t *testing.T) {
	skip.If(t, runtime.GOOS != "windows" && os.Getuid() != 0, "skipping test that requires root")
	sort.Sort(changesByPath(expectedChanges))
	sort.Sort(changesByPath(changes))
	for i := 0; i < max(len(changes), len(expectedChanges)); i++ {
		if i >= len(expectedChanges) {
			t.Fatalf("unexpected change %s\n", changes[i].String())
		}
		if i >= len(changes) {
			t.Fatalf("no change for expected change %s\n", expectedChanges[i].String())
		}
		if changes[i].Path == expectedChanges[i].Path {
			if changes[i] != expectedChanges[i] {
				t.Fatalf("Wrong change for %s, expected %s, got %s\n", changes[i].Path, changes[i].String(), expectedChanges[i].String())
			}
		} else if changes[i].Path < expectedChanges[i].Path {
			t.Fatalf("unexpected change %s\n", changes[i].String())
		} else {
			t.Fatalf("no change for expected change %s != %s\n", expectedChanges[i].String(), changes[i].String())
		}
	}
}
