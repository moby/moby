package main

import (
	"os"
	"path/filepath"
	"testing"
)

// docker cp CONTAINER:PATH LOCALPATH

// Try all of the test cases from the archive package which implements the
// internals of `docker cp` and ensure that the behavior matches when actually
// copying to and from containers.

// Basic assumptions about SRC and DST:
// 1. SRC must exist.
// 2. If SRC ends with a trailing separator, it must be a directory.
// 3. DST parent directory must exist.
// 4. If DST exists as a file, it must not end with a trailing separator.

// First get these easy error cases out of the way.

// Test for error when SRC does not exist.
func TestCpFromErrSrcNotExists(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-err-src-not-exists")
	defer os.RemoveAll(tmpDir)

	err := runDockerCp(t, containerCpPath(cID, "file1"), tmpDir)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error source not exists")
}

// Test for error when SRC ends in a trailing
// path separator but it exists as a file.
func TestCpFromErrSrcNotDir(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-err-src-not-dir")
	defer os.RemoveAll(tmpDir)

	err := runDockerCp(t, containerCpPathTrailingSep(cID, "file1"), tmpDir)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error source not directory")
}

// Test for error when SRC is a valid file or directory,
// bu the DST parent directory does not exist.
func TestCpFromErrDstParentNotExists(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-err-dst-parent-not-exists")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	// Try with a file source.
	srcPath := containerCpPath(cID, "/file1")
	dstPath := cpPath(tmpDir, "notExists", "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcPath = containerCpPath(cID, "/dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error destination parent not exists")
}

// Test for error when DST ends in a trailing
// path separator but exists as a file.
func TestCpFromErrDstNotDir(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-err-dst-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	// Try with a file source.
	srcPath := containerCpPath(cID, "/file1")
	dstPath := cpPathTrailingSep(tmpDir, "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcPath = containerCpPath(cID, "/dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error destination not directory")
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
func TestCpFromCaseA(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-a")
	defer os.RemoveAll(tmpDir)

	srcPath := containerCpPath(cID, "file1")
	dstPath := cpPath(tmpDir, "itWorks.txt")

	var err error

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container create file")
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func TestCpFromCaseB(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-b")
	defer os.RemoveAll(tmpDir)

	srcPath := containerCpPath(cID, "/file1")
	dstDir := cpPathTrailingSep(tmpDir, "testDir")

	var err error

	if err = runDockerCp(t, srcPath, dstDir); err == nil {
		t.Fatal("expected DirNotExists error, but got nil instead")
	}

	if !isCpDirNotExist(err) {
		t.Fatalf("expected DirNotExists error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error file copy can't create dir")
}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func TestCpFromCaseC(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-c")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := containerCpPath(cID, "file1")
	dstPath := cpPath(tmpDir, "file2")

	var err error

	// Ensure the local file starts with different content.
	if err = fileContentEquals(t, dstPath, "file2\n"); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container overwrite file")
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpFromCaseD(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-d")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := containerCpPath(cID, "/file1")
	dstDir := cpPath(tmpDir, "dir1")
	dstPath := filepath.Join(dstDir, "file1")

	var err error

	// Ensure that dstPath doesn't exist.
	if _, err = os.Stat(dstPath); !os.IsNotExist(err) {
		t.Fatalf("did not expect dstPath %q to exist", dstPath)
	}

	if err = runDockerCp(t, srcPath, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = cpPathTrailingSep(tmpDir, "dir1")

	if err = runDockerCp(t, srcPath, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container file to directory")
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func TestCpFromCaseE(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-e")
	defer os.RemoveAll(tmpDir)

	srcDir := containerCpPath(cID, "dir1")
	dstDir := cpPath(tmpDir, "testDir")
	dstPath := filepath.Join(dstDir, "file1-1")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	dstDir = cpPathTrailingSep(tmpDir, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container create directory")
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func TestCpFromCaseF(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-f")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := containerCpPath(cID, "dir1")
	dstFile := cpPath(tmpDir, "file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if !isCpCannotCopyDir(err) {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error directory can't replace file")
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func TestCpFromCaseG(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-g")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := containerCpPath(cID, "dir1")
	dstDir := cpPath(tmpDir, "dir2")
	resultDir := filepath.Join(dstDir, "dir1")
	dstPath := filepath.Join(resultDir, "file1-1")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = cpPathTrailingSep(tmpDir, "dir2")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container directory into directory")
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpFromCaseH(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-h")
	defer os.RemoveAll(tmpDir)

	srcDir := containerCpPathTrailingSep(cID, "dir1") + "."
	dstDir := cpPath(tmpDir, "testDir")
	dstPath := filepath.Join(dstDir, "file1-1")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove resultDir: %s", err)
	}

	dstDir = cpPathTrailingSep(tmpDir, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container create directory contents")
}

// I. SRC specifies a direcotry's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func TestCpFromCaseI(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-i")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := containerCpPathTrailingSep(cID, "dir1") + "."
	dstFile := cpPath(tmpDir, "file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if !isCpCannotCopyDir(err) {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy from container error directory contents can't replace file")
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func TestCpFromCaseJ(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-from-case-j")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := containerCpPathTrailingSep(cID, "dir1") + "."
	dstDir := cpPath(tmpDir, "dir2")
	dstPath := filepath.Join(dstDir, "file1-1")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatalf("unable to remove dstDir: %s", err)
	}

	if err = os.MkdirAll(dstDir, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to make dstDir: %s", err)
	}

	dstDir = cpPathTrailingSep(tmpDir, "dir2")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = fileContentEquals(t, dstPath, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy from container directory contents into directory")
}
