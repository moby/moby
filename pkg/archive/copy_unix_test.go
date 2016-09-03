// +build !windows

// TODO Windows: Some of these tests may be salvagable and portable to Windows.

package archive

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-check/check"
)

func removeAllPaths(paths ...string) {
	for _, path := range paths {
		os.RemoveAll(path)
	}
}

func getTestTempDirs(c *check.C) (tmpDirA, tmpDirB string) {
	var err error

	if tmpDirA, err = ioutil.TempDir("", "archive-copy-test"); err != nil {
		c.Fatal(err)
	}

	if tmpDirB, err = ioutil.TempDir("", "archive-copy-test"); err != nil {
		c.Fatal(err)
	}

	return
}

func isNotDir(err error) bool {
	return strings.Contains(err.Error(), "not a directory")
}

func joinTrailingSep(pathElements ...string) string {
	joined := filepath.Join(pathElements...)

	return fmt.Sprintf("%s%c", joined, filepath.Separator)
}

func fileContentsEqual(c *check.C, filenameA, filenameB string) (err error) {
	c.Logf("checking for equal file contents: %q and %q\n", filenameA, filenameB)

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

func dirContentsEqual(c *check.C, newDir, oldDir string) (err error) {
	c.Logf("checking for equal directory contents: %q and %q\n", newDir, oldDir)

	var changes []Change

	if changes, err = ChangesDirs(newDir, oldDir); err != nil {
		return
	}

	if len(changes) != 0 {
		err = fmt.Errorf("expected no changes between directories, but got: %v", changes)
	}

	return
}

func logDirContents(c *check.C, dirPath string) {
	logWalkedPaths := filepath.WalkFunc(func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.Errorf("stat error for path %q: %s", path, err)
			return nil
		}

		if info.IsDir() {
			path = joinTrailingSep(path)
		}

		c.Logf("\t%s", path)

		return nil
	})

	c.Logf("logging directory contents: %q", dirPath)

	if err := filepath.Walk(dirPath, logWalkedPaths); err != nil {
		c.Fatal(err)
	}
}

func testCopyHelper(c *check.C, srcPath, dstPath string) (err error) {
	c.Logf("copying from %q to %q (not follow symbol link)", srcPath, dstPath)

	return CopyResource(srcPath, dstPath, false)
}

func testCopyHelperFSym(c *check.C, srcPath, dstPath string) (err error) {
	c.Logf("copying from %q to %q (follow symbol link)", srcPath, dstPath)

	return CopyResource(srcPath, dstPath, true)
}

// Basic assumptions about SRC and DST:
// 1. SRC must exist.
// 2. If SRC ends with a trailing separator, it must be a directory.
// 3. DST parent directory must exist.
// 4. If DST exists as a file, it must not end with a trailing separator.

// First get these easy error cases out of the way.

// Test for error when SRC does not exist.
func (s *DockerSuite) TestCopyErrSrcNotExists(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	if _, err := CopyInfoSourcePath(filepath.Join(tmpDirA, "file1"), false); !os.IsNotExist(err) {
		c.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}
}

// Test for error when SRC ends in a trailing
// path separator but it exists as a file.
func (s *DockerSuite) TestCopyErrSrcNotDir(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	if _, err := CopyInfoSourcePath(joinTrailingSep(tmpDirA, "file1"), false); !isNotDir(err) {
		c.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}
}

// Test for error when SRC is a valid file or directory,
// but the DST parent directory does not exist.
func (s *DockerSuite) TestCopyErrDstParentNotExists(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcInfo := CopyInfo{Path: filepath.Join(tmpDirA, "file1"), Exists: true, IsDir: false}

	// Try with a file source.
	content, err := TarResource(srcInfo)
	if err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	// Copy to a file whose parent does not exist.
	if err = CopyTo(content, srcInfo, filepath.Join(tmpDirB, "fakeParentDir", "file1")); err == nil {
		c.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !os.IsNotExist(err) {
		c.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcInfo = CopyInfo{Path: filepath.Join(tmpDirA, "dir1"), Exists: true, IsDir: true}

	content, err = TarResource(srcInfo)
	if err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	// Copy to a directory whose parent does not exist.
	if err = CopyTo(content, srcInfo, joinTrailingSep(tmpDirB, "fakeParentDir", "fakeDstDir")); err == nil {
		c.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !os.IsNotExist(err) {
		c.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}
}

// Test for error when DST ends in a trailing
// path separator but exists as a file.
func (s *DockerSuite) TestCopyErrDstNotDir(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	// Try with a file source.
	srcInfo := CopyInfo{Path: filepath.Join(tmpDirA, "file1"), Exists: true, IsDir: false}

	content, err := TarResource(srcInfo)
	if err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	if err = CopyTo(content, srcInfo, joinTrailingSep(tmpDirB, "file1")); err == nil {
		c.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isNotDir(err) {
		c.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcInfo = CopyInfo{Path: filepath.Join(tmpDirA, "dir1"), Exists: true, IsDir: true}

	content, err = TarResource(srcInfo)
	if err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}
	defer content.Close()

	if err = CopyTo(content, srcInfo, joinTrailingSep(tmpDirB, "file1")); err == nil {
		c.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isNotDir(err) {
		c.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
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
func (s *DockerSuite) TestCopyCaseA(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstPath := filepath.Join(tmpDirB, "itWorks.txt")

	var err error

	if err = testCopyHelper(c, srcPath, dstPath); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, srcPath, dstPath); err != nil {
		c.Fatal(err)
	}
	os.Remove(dstPath)

	symlinkPath := filepath.Join(tmpDirA, "symlink3")
	symlinkPath1 := filepath.Join(tmpDirA, "symlink4")
	linkTarget := filepath.Join(tmpDirA, "file1")

	if err = testCopyHelperFSym(c, symlinkPath, dstPath); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, linkTarget, dstPath); err != nil {
		c.Fatal(err)
	}
	os.Remove(dstPath)
	if err = testCopyHelperFSym(c, symlinkPath1, dstPath); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, linkTarget, dstPath); err != nil {
		c.Fatal(err)
	}
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func (s *DockerSuite) TestCopyCaseB(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstDir := joinTrailingSep(tmpDirB, "testDir")

	var err error

	if err = testCopyHelper(c, srcPath, dstDir); err == nil {
		c.Fatal("expected ErrDirNotExists error, but got nil instead")
	}

	if err != ErrDirNotExists {
		c.Fatalf("expected ErrDirNotExists error, but got %T: %s", err, err)
	}

	symlinkPath := filepath.Join(tmpDirA, "symlink3")

	if err = testCopyHelperFSym(c, symlinkPath, dstDir); err == nil {
		c.Fatal("expected ErrDirNotExists error, but got nil instead")
	}
	if err != ErrDirNotExists {
		c.Fatalf("expected ErrDirNotExists error, but got %T: %s", err, err)
	}

}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func (s *DockerSuite) TestCopyCaseC(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstPath := filepath.Join(tmpDirB, "file2")

	var err error

	// Ensure they start out different.
	if err = fileContentsEqual(c, srcPath, dstPath); err == nil {
		c.Fatal("expected different file contents")
	}

	if err = testCopyHelper(c, srcPath, dstPath); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, srcPath, dstPath); err != nil {
		c.Fatal(err)
	}
}

// C. Symbol link following version:
//    SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func (s *DockerSuite) TestCopyCaseCFSym(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	symlinkPathBad := filepath.Join(tmpDirA, "symlink1")
	symlinkPath := filepath.Join(tmpDirA, "symlink3")
	linkTarget := filepath.Join(tmpDirA, "file1")
	dstPath := filepath.Join(tmpDirB, "file2")

	var err error

	// first to test broken link
	if err = testCopyHelperFSym(c, symlinkPathBad, dstPath); err == nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	// test symbol link -> symbol link -> target
	// Ensure they start out different.
	if err = fileContentsEqual(c, linkTarget, dstPath); err == nil {
		c.Fatal("expected different file contents")
	}

	if err = testCopyHelperFSym(c, symlinkPath, dstPath); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, linkTarget, dstPath); err != nil {
		c.Fatal(err)
	}
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCopyCaseD(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcPath := filepath.Join(tmpDirA, "file1")
	dstDir := filepath.Join(tmpDirB, "dir1")
	dstPath := filepath.Join(dstDir, "file1")

	var err error

	// Ensure that dstPath doesn't exist.
	if _, err = os.Stat(dstPath); !os.IsNotExist(err) {
		c.Fatalf("did not expect dstPath %q to exist", dstPath)
	}

	if err = testCopyHelper(c, srcPath, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, srcPath, dstPath); err != nil {
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir1")

	if err = testCopyHelper(c, srcPath, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, srcPath, dstPath); err != nil {
		c.Fatal(err)
	}
}

// D. Symbol link following version:
//    SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCopyCaseDFSym(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcPath := filepath.Join(tmpDirA, "symlink4")
	linkTarget := filepath.Join(tmpDirA, "file1")
	dstDir := filepath.Join(tmpDirB, "dir1")
	dstPath := filepath.Join(dstDir, "symlink4")

	var err error

	// Ensure that dstPath doesn't exist.
	if _, err = os.Stat(dstPath); !os.IsNotExist(err) {
		c.Fatalf("did not expect dstPath %q to exist", dstPath)
	}

	if err = testCopyHelperFSym(c, srcPath, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, linkTarget, dstPath); err != nil {
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir1")

	if err = testCopyHelperFSym(c, srcPath, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = fileContentsEqual(c, linkTarget, dstPath); err != nil {
		c.Fatal(err)
	}
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func (s *DockerSuite) TestCopyCaseE(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcDir := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, srcDir); err != nil {
		c.Log("dir contents not equal")
		logDirContents(c, tmpDirA)
		logDirContents(c, tmpDirB)
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, srcDir); err != nil {
		c.Fatal(err)
	}
}

// E. Symbol link following version:
//    SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func (s *DockerSuite) TestCopyCaseEFSym(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcDir := filepath.Join(tmpDirA, "dirSymlink")
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, linkTarget); err != nil {
		c.Log("dir contents not equal")
		logDirContents(c, tmpDirA)
		logDirContents(c, tmpDirB)
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, linkTarget); err != nil {
		c.Fatal(err)
	}
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func (s *DockerSuite) TestCopyCaseF(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcDir := filepath.Join(tmpDirA, "dir1")
	symSrcDir := filepath.Join(tmpDirA, "dirSymlink")
	dstFile := filepath.Join(tmpDirB, "file1")

	var err error

	if err = testCopyHelper(c, srcDir, dstFile); err == nil {
		c.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		c.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	// now test with symbol link
	if err = testCopyHelperFSym(c, symSrcDir, dstFile); err == nil {
		c.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		c.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCopyCaseG(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcDir := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "dir2")
	resultDir := filepath.Join(dstDir, "dir1")

	var err error

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, resultDir, srcDir); err != nil {
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir2")

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, resultDir, srcDir); err != nil {
		c.Fatal(err)
	}
}

// G. Symbol link version:
//    SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCopyCaseGFSym(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcDir := filepath.Join(tmpDirA, "dirSymlink")
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "dir2")
	resultDir := filepath.Join(dstDir, "dirSymlink")

	var err error

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, resultDir, linkTarget); err != nil {
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir2")

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, resultDir, linkTarget); err != nil {
		c.Fatal(err)
	}
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCopyCaseH(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcDir := joinTrailingSep(tmpDirA, "dir1") + "."
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, srcDir); err != nil {
		c.Log("dir contents not equal")
		logDirContents(c, tmpDirA)
		logDirContents(c, tmpDirB)
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, srcDir); err != nil {
		c.Log("dir contents not equal")
		logDirContents(c, tmpDirA)
		logDirContents(c, tmpDirB)
		c.Fatal(err)
	}
}

// H. Symbol link following version:
//    SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCopyCaseHFSym(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A with some sample files and directories.
	createSampleDir(c, tmpDirA)

	srcDir := joinTrailingSep(tmpDirA, "dirSymlink") + "."
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "testDir")

	var err error

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, linkTarget); err != nil {
		c.Log("dir contents not equal")
		logDirContents(c, tmpDirA)
		logDirContents(c, tmpDirB)
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "testDir")

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, linkTarget); err != nil {
		c.Log("dir contents not equal")
		logDirContents(c, tmpDirA)
		logDirContents(c, tmpDirB)
		c.Fatal(err)
	}
}

// I. SRC specifies a directory's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func (s *DockerSuite) TestCopyCaseI(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcDir := joinTrailingSep(tmpDirA, "dir1") + "."
	symSrcDir := filepath.Join(tmpDirB, "dirSymlink")
	dstFile := filepath.Join(tmpDirB, "file1")

	var err error

	if err = testCopyHelper(c, srcDir, dstFile); err == nil {
		c.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		c.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	// now try with symbol link of dir
	if err = testCopyHelperFSym(c, symSrcDir, dstFile); err == nil {
		c.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if err != ErrCannotCopyDir {
		c.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func (s *DockerSuite) TestCopyCaseJ(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcDir := joinTrailingSep(tmpDirA, "dir1") + "."
	dstDir := filepath.Join(tmpDirB, "dir5")

	var err error

	// first to create an empty dir
	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, srcDir); err != nil {
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir5")

	if err = testCopyHelper(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, srcDir); err != nil {
		c.Fatal(err)
	}
}

// J. Symbol link following version:
//    SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func (s *DockerSuite) TestCopyCaseJFSym(c *check.C) {
	tmpDirA, tmpDirB := getTestTempDirs(c)
	defer removeAllPaths(tmpDirA, tmpDirB)

	// Load A and B with some sample files and directories.
	createSampleDir(c, tmpDirA)
	createSampleDir(c, tmpDirB)

	srcDir := joinTrailingSep(tmpDirA, "dirSymlink") + "."
	linkTarget := filepath.Join(tmpDirA, "dir1")
	dstDir := filepath.Join(tmpDirB, "dir5")

	var err error

	// first to create an empty dir
	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, linkTarget); err != nil {
		c.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		c.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		c.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = joinTrailingSep(tmpDirB, "dir5")

	if err = testCopyHelperFSym(c, srcDir, dstDir); err != nil {
		c.Fatalf("unexpected error %T: %s", err, err)
	}

	if err = dirContentsEqual(c, dstDir, linkTarget); err != nil {
		c.Fatal(err)
	}
}
