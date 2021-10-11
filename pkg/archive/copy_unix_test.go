//go:build !windows
// +build !windows

// TODO Windows: Some of these tests may be salvageable and portable to Windows.

package archive // import "github.com/docker/docker/pkg/archive"

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func removeAllPaths(paths ...string) {
	for _, path := range paths {
		os.RemoveAll(path)
	}
}

func getTestTempDirs(t *testing.T) (tmpDirA, tmpDirB string) {
	var err error

	tmpDirA, err = os.MkdirTemp("", "archive-copy-test")
	assert.NilError(t, err)

	tmpDirB, err = os.MkdirTemp("", "archive-copy-test")
	assert.NilError(t, err)

	return
}

func isNotDir(err error) bool {
	return strings.Contains(err.Error(), "not a directory")
}

func joinTrailingSep(pathElements ...string) string {
	joined := filepath.Join(pathElements...)

	return fmt.Sprintf("%s%c", joined, filepath.Separator)
}

func fileContentsEqual(t *testing.T, filenameA, filenameB string) (err error) {
	t.Logf("checking for equal file contents: %q and %q\n", filenameA, filenameB)

	fileA, err := os.Open(filenameA)
	if err != nil {
		return
	}
	defer fileA.Close()

	fileB, err := os.Open(filenameB)
	if err != nil {
		return
	}
	defer fileB.Close()

	hasher := sha256.New()

	if _, err = io.Copy(hasher, fileA); err != nil {
		return
	}

	hashA := hasher.Sum(nil)
	hasher.Reset()

	if _, err = io.Copy(hasher, fileB); err != nil {
		return
	}

	hashB := hasher.Sum(nil)

	if !bytes.Equal(hashA, hashB) {
		err = fmt.Errorf("file content hashes not equal - expected %s, got %s", hex.EncodeToString(hashA), hex.EncodeToString(hashB))
	}

	return
}

func dirContentsEqual(t *testing.T, newDir, oldDir string) (err error) {
	t.Logf("checking for equal directory contents: %q and %q\n", newDir, oldDir)

	var changes []Change

	if changes, err = ChangesDirs(newDir, oldDir); err != nil {
		return
	}

	if len(changes) != 0 {
		err = fmt.Errorf("expected no changes between directories, but got: %v", changes)
	}

	return
}

func logDirContents(t *testing.T, dirPath string) {
	logWalkedPaths := filepath.WalkFunc(func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Errorf("stat error for path %q: %s", path, err)
			return nil
		}

		if info.IsDir() {
			path = joinTrailingSep(path)
		}

		t.Logf("\t%s", path)

		return nil
	})

	t.Logf("logging directory contents: %q", dirPath)

	err := filepath.Walk(dirPath, logWalkedPaths)
	assert.NilError(t, err)
}

func testCopyHelper(t *testing.T, srcPath, dstPath string) (err error) {
	t.Logf("copying from %q to %q (not follow symbol link)", srcPath, dstPath)

	return CopyResource(srcPath, dstPath, false)
}

func testCopyHelperFSym(t *testing.T, srcPath, dstPath string) (err error) {
	t.Logf("copying from %q to %q (follow symbol link)", srcPath, dstPath)

	return CopyResource(srcPath, dstPath, true)
}

// Basic assumptions about SRC and DST:
// 1. SRC must exist.
// 2. If SRC ends with a trailing separator, it must be a directory.
// 3. DST parent directory must exist.
// 4. If DST exists as a file, it must not end with a trailing separator.

// First get these easy error cases out of the way.

// Test for error when SRC does not exist.
func TestCopyErrSrcNotExists(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	if _, err := CopyInfoSourcePath(filepath.Join(tmpDirA, "file1"), false); !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}
}

// Test for error when SRC ends in a trailing
// path separator but it exists as a file.
func TestCopyErrSrcNotDir(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	if _, err := CopyInfoSourcePath(joinTrailingSep(tmpDirA, "file1"), false); !isNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}
}

// Test for error when SRC is a valid file or directory,
// but the DST parent directory does not exist.
func TestCopyErrDstParentNotExists(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcInfo := CopyInfo{Path: filepath.Join(tmpDirA, "file1"), Exists: true, IsDir: false}

	// Try with a file source.
	content, err := TarResource(srcInfo)
	if err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	// Copy to a file whose parent does not exist.
	if err = CopyTo(content, srcInfo, filepath.Join(tmpDirB, "fakeParentDir", "file1")); err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcInfo = CopyInfo{Path: filepath.Join(tmpDirA, "dir1"), Exists: true, IsDir: true}

	content, err = TarResource(srcInfo)
	if err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	// Copy to a directory whose parent does not exist.
	if err = CopyTo(content, srcInfo, joinTrailingSep(tmpDirB, "fakeParentDir", "fakeDstDir")); err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}
}

// Test for error when DST ends in a trailing
// path separator but exists as a file.
func TestCopyErrDstNotDir(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	// Try with a file source.
	srcInfo := CopyInfo{Path: filepath.Join(tmpDirA, "file1"), Exists: true, IsDir: false}

	content, err := TarResource(srcInfo)
	if err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	if err = CopyTo(content, srcInfo, joinTrailingSep(tmpDirB, "file1")); err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcInfo = CopyInfo{Path: filepath.Join(tmpDirA, "dir1"), Exists: true, IsDir: true}

	content, err = TarResource(srcInfo)
	if err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	if err = CopyTo(content, srcInfo, joinTrailingSep(tmpDirB, "file1")); err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}
}

// Test to check if CopyTo works with a long (>100 characters) destination file name.
// This is a regression (see https://github.com/docker/for-linux/issues/484).
func TestCopyLongDstFilename(t *testing.T) {
	const longName = "a_very_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx_long_filename_that_is_101_characters"
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcInfo := CopyInfo{Path: filepath.Join(tmpDirA, "file1"), Exists: true, IsDir: false}

	content, err := TarResource(srcInfo)
	if err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	err = CopyTo(content, srcInfo, filepath.Join(tmpDirB, longName))
	if err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}
}

// Possibilities are reduced to the remaining 10 cases:
//
//  case | srcIsDir | onlyDirContents | dstExists | dstIsDir | dstTrSep | action
// ===================================================================================================
//   A   |  no      |  -              |  no       |  -       |  no      |  create file
//   B   |  no      |  -              |  no       |  -       |  yes     |  error
//   C   |  no      |  -              |  yes      |  no      |  -       |  overwrite file
//   D   |  no      |  -              |  yes      |  yes     |  -       |  create file in dst dir
//   E   |  yes     |  no             |  no       |  -       |  -       |  create dir, copy contents
//   F   |  yes     |  no             |  yes      |  no      |  -       |  error
//   G   |  yes     |  no             |  yes      |  yes     |  -       |  copy dir and contents
//   H   |  yes     |  yes            |  no       |  -       |  -       |  create dir, copy contents
//   I   |  yes     |  yes            |  yes      |  no      |  -       |  error
//   J   |  yes     |  yes            |  yes      |  yes     |  -       |  copy dir contents
//

// A. SRC specifies a file and DST (no trailing path separator) doesn't
//    exist. This should create a file with the name DST and copy the
//    contents of the source file into it.
func TestCopyCaseA(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstPath := filepath.Join(tmpDirB, "itWorks.txt")

	var err error

	if err = testCopyHelper(t, srcPath, dstPath); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, srcPath, dstPath)
	assert.NilError(t, err)
	os.Remove(dstPath)

	symlinkPath := filepath.Join(tmpDirA, "symlink3")
	symlinkPath1 := filepath.Join(tmpDirA, "symlink4")
	linkTarget := filepath.Join(tmpDirA, "file1")

	if err = testCopyHelperFSym(t, symlinkPath, dstPath); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, linkTarget, dstPath)
	assert.NilError(t, err)
	os.Remove(dstPath)
	if err = testCopyHelperFSym(t, symlinkPath1, dstPath); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, linkTarget, dstPath)
	assert.NilError(t, err)
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func TestCopyCaseB(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstDir := joinTrailingSep(tmpDirB, "testDir")

	var err error

	if err = testCopyHelper(t, srcPath, dstDir); err == nil {
		t.Fatal("expected ErrDirNotExists error, but got nil instead")
	}

	if err != ErrDirNotExists {
		t.Fatalf("expected ErrDirNotExists error, but got %T: %s", err, err)
	}

	symlinkPath := filepath.Join(tmpDirA, "symlink3")

	if err = testCopyHelperFSym(t, symlinkPath, dstDir); err == nil {
		t.Fatal("expected ErrDirNotExists error, but got nil instead")
	}
	if err != ErrDirNotExists {
		t.Fatalf("expected ErrDirNotExists error, but got %T: %s", err, err)
	}

}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func TestCopyCaseC(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstPath := filepath.Join(tmpDirB, "file2")

	var err error

	// Ensure they start out different.
	if err = fileContentsEqual(t, srcPath, dstPath); err == nil {
		t.Fatal("expected different file contents")
	}

	if err = testCopyHelper(t, srcPath, dstPath); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, srcPath, dstPath)
	assert.NilError(t, err)
}

// C. Symbol link following version:
//    SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func TestCopyCaseCFSym(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	symlinkPathBad := filepath.Join(tmpDirA, "symlink1")
	symlinkPath := filepath.Join(tmpDirA, "symlink3")
	linkTarget := filepath.Join(tmpDirA, "file1")
	dstPath := filepath.Join(tmpDirB, "file2")

	var err error

	// first to test broken link
	if err = testCopyHelperFSym(t, symlinkPathBad, dstPath); err == nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	// test symbol link -> symbol link -> target
	// Ensure they start out different.
	if err = fileContentsEqual(t, linkTarget, dstPath); err == nil {
		t.Fatal("expected different file contents")
	}

	if err = testCopyHelperFSym(t, symlinkPath, dstPath); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, linkTarget, dstPath)
	assert.NilError(t, err)
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCopyCaseD(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstDir := filepath.Join(tmpDirB, "dir1")
	dstPath := filepath.Join(dstDir, "file1")

	var err error

	// Ensure that dstPath doesn't exist.
	if _, err = os.Stat(dstPath); !os.IsNotExist(err) {
		t.Fatalf("did not expect dstPath %q to exist", dstPath)
	}

	if err = testCopyHelper(t, srcPath, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, srcPath, dstPath)
	assert.NilError(t, err)

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir1")

	if err = testCopyHelper(t, srcPath, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, srcPath, dstPath)
	assert.NilError(t, err)
}

// D. Symbol link following version:
//    SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCopyCaseDFSym(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcPath := filepath.Join(tmpDirA, "symlink4")
	linkTarget := filepath.Join(tmpDirA, "file1")
	dstDir := filepath.Join(tmpDirB, "dir1")
	dstPath := filepath.Join(dstDir, "symlink4")

	var err error

	// Ensure that dstPath doesn't exist.
	if _, err = os.Stat(dstPath); !os.IsNotExist(err) {
		t.Fatalf("did not expect dstPath %q to exist", dstPath)
	}

	if err = testCopyHelperFSym(t, srcPath, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, linkTarget, dstPath)
	assert.NilError(t, err)

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir1")

	if err = testCopyHelperFSym(t, srcPath, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = fileContentsEqual(t, linkTarget, dstPath)
	assert.NilError(t, err)
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func TestCopyCaseE(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcDir := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(t, dstDir, srcDir); err != nil {
		t.Log("dir contents not equal")
		logDirContents(t, tmpDirA)
		logDirContents(t, tmpDirB)
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, dstDir, srcDir)
	assert.NilError(t, err)
}

// E. Symbol link following version:
//    SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func TestCopyCaseEFSym(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcDir := filepath.Join(tmpDirA, "dirSymlink")
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(t, dstDir, linkTarget); err != nil {
		t.Log("dir contents not equal")
		logDirContents(t, tmpDirA)
		logDirContents(t, tmpDirB)
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, dstDir, linkTarget)
	assert.NilError(t, err)
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func TestCopyCaseF(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcDir := filepath.Join(tmpDirA, "dir1")
	symSrcDir := filepath.Join(tmpDirA, "dirSymlink")
	dstFile := filepath.Join(tmpDirB, "file1")

	var err error

	if err = testCopyHelper(t, srcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	// now test with symbol link
	if err = testCopyHelperFSym(t, symSrcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func TestCopyCaseG(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcDir := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "dir2")
	resultDir := filepath.Join(dstDir, "dir1")

	var err error

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, resultDir, srcDir)
	assert.NilError(t, err)

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir2")

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, resultDir, srcDir)
	assert.NilError(t, err)
}

// G. Symbol link version:
//    SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func TestCopyCaseGFSym(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcDir := filepath.Join(tmpDirA, "dirSymlink")
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "dir2")
	resultDir := filepath.Join(dstDir, "dirSymlink")

	var err error

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, resultDir, linkTarget)
	assert.NilError(t, err)

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir2")

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, resultDir, linkTarget)
	assert.NilError(t, err)
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCopyCaseH(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcDir := joinTrailingSep(tmpDirA, "dir1") + "."
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(t, dstDir, srcDir); err != nil {
		t.Log("dir contents not equal")
		logDirContents(t, tmpDirA)
		logDirContents(t, tmpDirB)
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(t, dstDir, srcDir); err != nil {
		t.Log("dir contents not equal")
		logDirContents(t, tmpDirA)
		logDirContents(t, tmpDirB)
		t.Fatal(err)
	}
}

// H. Symbol link following version:
//    SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCopyCaseHFSym(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(t, tmpDirA)

	srcDir := joinTrailingSep(tmpDirA, "dirSymlink") + "."
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(t, dstDir, linkTarget); err != nil {
		t.Log("dir contents not equal")
		logDirContents(t, tmpDirA)
		logDirContents(t, tmpDirB)
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(t, dstDir, linkTarget); err != nil {
		t.Log("dir contents not equal")
		logDirContents(t, tmpDirA)
		logDirContents(t, tmpDirB)
		t.Fatal(err)
	}
}

// I. SRC specifies a directory's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func TestCopyCaseI(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcDir := joinTrailingSep(tmpDirA, "dir1") + "."
	symSrcDir := filepath.Join(tmpDirB, "dirSymlink")
	dstFile := filepath.Join(tmpDirB, "file1")

	var err error

	if err = testCopyHelper(t, srcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	// now try with symbol link of dir
	if err = testCopyHelperFSym(t, symSrcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func TestCopyCaseJ(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcDir := joinTrailingSep(tmpDirA, "dir1") + "."
	dstDir := filepath.Join(tmpDirB, "dir5")

	var err error

	// first to create an empty dir
	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, dstDir, srcDir)
	assert.NilError(t, err)

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir5")

	if err = testCopyHelper(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, dstDir, srcDir)
	assert.NilError(t, err)
}

// J. Symbol link following version:
//    SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func TestCopyCaseJFSym(t *testing.T) {
	tmpDirA, tmpDirB := getTestTempDirs(t)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(t, tmpDirA)
	createSampleDir(t, tmpDirB)

	srcDir := joinTrailingSep(tmpDirA, "dirSymlink") + "."
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "dir5")

	var err error

	// first to create an empty dir
	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, dstDir, linkTarget)
	assert.NilError(t, err)

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir5")

	if err = testCopyHelperFSym(t, srcDir, dstDir); err != nil {
		t.Fatalf("unexpected error %T: %s", err, err)
	}

	err = dirContentsEqual(t, dstDir, linkTarget)
	assert.NilError(t, err)
}
