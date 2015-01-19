package main

import (
	"testing"
)

// docker cp CONTAINER:PATH CONTAINER:PATH

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
func TestCpAcrossErrSrcNotExists(t *testing.T) {
	srcID := makeTestContainer(t, false, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, nil, "")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "file1")
	dstPath := containerCpPath(dstID, "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error source not exists")
}

// Test for error when SRC ends in a trailing
// path separator but it exists as a file.
func TestCpAcrossErrSrcNotDir(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, nil, "")
	defer deleteContainer(dstID)

	srcPath := containerCpPathTrailingSep(srcID, "file1")
	dstPath := containerCpPath(dstID, "testDir")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error source not directory")
}

// Test for error when SRC is a valid file or directory,
// bu the DST parent directory does not exist.
func TestCpAcrossErrDstParentNotExists(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, nil, "")
	defer deleteContainer(dstID)

	// Try with a file source.
	srcPath := containerCpPath(srcID, "file1")
	dstPath := containerCpPath(dstID, "notExists", "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcID = makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	srcPath = containerCpPath(srcID, "dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error destination parent not exists")
}

// Test for error when DST ends in a trailing
// path separator but exists as a file.
func TestCpAcrossErrDstNotDir(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(dstID)

	// Try with a file source.
	srcPath := containerCpPath(srcID, "file1")
	dstPath := containerCpPathTrailingSep(dstID, "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcID = makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	srcPath = containerCpPath(srcID, "dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error destination not directory")
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
func TestCpAcrossCaseA(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, nil, "/root", "itWorks.txt")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "file1")
	dstPath := containerCpPath(dstID, "itWorks.txt")

	var err error

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers create file")
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func TestCpAcrossCaseB(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, nil, "")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "file1")
	dstDir := containerCpPathTrailingSep(dstID, "testDir")

	var err error

	if err = runDockerCp(t, srcPath, dstDir); err == nil {
		t.Fatal("expected DirNotExists error, but got nil instead")
	}

	if !isCpDirNotExist(err) {
		t.Fatalf("expected DirNotExists error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error file copy can't create dir")
}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func TestCpAcrossCaseC(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, true, nil, "/root", "file2")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "file1")
	dstPath := containerCpPath(dstID, "file2")

	var err error

	// Ensure the container's file starts with the original content.
	if err = containerStartOutputEquals(t, dstID, "file2\n"); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers overwrite file")
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpAcrossCaseD(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "/root")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, true, nil, "", "/dir1/file1")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "/root/file1")
	dstDir := containerCpPath(dstID, "dir1")

	var err error

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	dstID = makeTestContainerCatFile(t, true, nil, "", "/dir1/file1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "dir1")

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers file to directory")
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func TestCpAcrossCaseE(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, nil, "", "/testDir/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPath(srcID, "dir1")
	dstDir := containerCpPath(dstID, "testDir")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	dstID = makeTestContainerCatFile(t, false, nil, "", "/testDir/file1-1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers create directory")
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func TestCpAcrossCaseF(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, true, nil, "/root")
	defer deleteContainer(dstID)

	srcDir := containerCpPath(srcID, "/dir1")
	dstFile := containerCpPath(dstID, "/root/file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error directory can't replace file")
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func TestCpAcrossCaseG(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, true, nil, "/root", "dir2/dir1/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPath(srcID, "dir1")
	dstDir := containerCpPath(dstID, "/root/dir2")

	var err error

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	dstID = makeTestContainerCatFile(t, true, nil, "", "/dir2/dir1/file1-1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "/dir2")

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers directory into directory")
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpAcrossCaseH(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, nil, "", "/testDir/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPathTrailingSep(srcID, "/dir1") + "."
	dstDir := containerCpPath(dstID, "testDir")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	dstID = makeTestContainerCatFile(t, false, nil, "", "/testDir/file1-1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers create directory contents")
}

// I. SRC specifies a direcotry's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func TestCpAcrossCaseI(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, true, nil, "/root")
	defer deleteContainer(dstID)

	srcDir := containerCpPathTrailingSep(srcID, "dir1") + "."
	dstFile := containerCpPath(dstID, "file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across containers error directory contents can't replace file")
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func TestCpAcrossCaseJ(t *testing.T) {
	srcID := makeTestContainer(t, true, nil, "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, true, nil, "/root", "/dir2/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPathTrailingSep(srcID, "dir1") + "."
	dstDir := containerCpPath(dstID, "/dir2")

	var err error

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	dstID = makeTestContainerCatFile(t, true, nil, "", "/dir2/file1-1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "/dir2")

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across containers directory contents into directory")
}
