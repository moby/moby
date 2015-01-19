package main

import (
	"os"
	"testing"
)

// docker cp CONTAINER:PATH CONTAINER:PATH

// Try all of the test cases from the archive package which implements the
// internals of `docker cp` and ensure that the behavior matches when actually
// copying to and from containers. Each of the tests in this file are cases
// with volumes. All tests copy between containers to cover copy from volumes
// and copy to volumes cases together.

// Basic assumptions about SRC and DST:
// 1. SRC must exist.
// 2. If SRC ends with a trailing separator, it must be a directory.
// 3. DST parent directory must exist.
// 4. If DST exists as a file, it must not end with a trailing separator.

// First get these easy error cases out of the way.

// Test for error when SRC does not exist.
func TestCpWithVolErrSrcNotExists(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-err-src-not-exist")
	defer os.RemoveAll(tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "/vol2/file1")
	dstPath := containerCpPath(dstID, "/vol1/file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error source not exists")
}

// Test for error when SRC ends in a trailing
// path separator but it exists as a file.
func TestCpWithVolErrSrcNotDir(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-err-src-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(dstID)

	srcPath := containerCpPathTrailingSep(srcID, "/vol2/file1")
	dstPath := containerCpPath(dstID, "/vol1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error source not directory")
}

// Test for error when SRC is a valid file or directory,
// bu the DST parent directory does not exist.
func TestCpWithVolErrDstParentNotExists(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-err-dst-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(dstID)

	// Try with a file source.
	srcPath := containerCpPath(srcID, "/vol2/file1")
	dstPath := containerCpPath(dstID, "/vol2/notExists/file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcID = makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	srcPath = containerCpPath(srcID, "/vol2/dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error destination parent not exists")
}

// Test for error when DST ends in a trailing
// path separator but exists as a file.
func TestCpWithVolErrDstNotDir(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-err-dst-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "/vol2/file1")
	dstPath := containerCpPathTrailingSep(dstID, "/vol2/file2")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcID = makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error destination not directory")
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
func TestCpWithVolCaseA(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-a")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/vol3", "itWorks.txt")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "/vol2/file1")
	dstPath := containerCpPath(dstID, "/vol2/vol3/itWorks.txt")

	var err error

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across volumes create file")
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func TestCpWithVolCaseB(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-b")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "file1")
	dstDir := containerCpPathTrailingSep(dstID, "vol3/testDir")

	var err error

	if err = runDockerCp(t, srcPath, dstDir); err == nil {
		t.Fatal("expected DirNotExists error, but got nil instead")
	}

	if !isCpDirNotExist(err) {
		t.Fatalf("expected DirNotExists error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error file copy can't create dir")
}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func TestCpWithVolCaseC(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-c")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/vol2/vol3", "/vol2/file2")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "file1")
	dstPath := containerCpPath(dstID, "../file2")

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

	logDone("cp - copy across volumes overwrite file")
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpWithVolCaseD(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-d")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2/vol3")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "", "/vol1/file1")
	defer deleteContainer(dstID)

	srcPath := containerCpPath(srcID, "../file1")
	dstDir := containerCpPath(dstID, "vol1")

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
	dstID = makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "", "/vol1/file1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "vol1")

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

	logDone("cp - copy across volumes file to directory")
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func TestCpWithVolCaseE(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-e")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2/vol3")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/vol1", "/vol1/testDir/file1")
	defer deleteContainer(dstID)

	srcDir := containerCpPath(srcID, "..")
	dstDir := containerCpPath(dstID, "testDir")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	dstID = makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/vol1", "/vol1/testDir/file1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across volumes create directory")
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func TestCpWithVolCaseF(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-f")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2")
	defer deleteContainer(dstID)

	srcDir := containerCpPath(srcID, "dir1")
	dstFile := containerCpPath(dstID, "vol3/../file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error directory can't replace file")
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func TestCpWithVolCaseG(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-g")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2/vol3")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/vol2/vol3", "/vol1/vol2/dir1/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPath(srcID, "..")
	dstDir := containerCpPath(dstID, "../../vol1")

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
	dstID = makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/vol2/vol3", "/vol1/vol2/dir1/file1-1")
	defer deleteContainer(dstID)

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, dstID, ""); err != nil {
		t.Fatal(err)
	}

	dstDir = containerCpPathTrailingSep(dstID, "../../vol1")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across volumes directory into directory")
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpWithVolCaseH(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-h")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2/vol3")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "", "/vol1/testDir/dir1/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPathTrailingSep(srcID, "..") + "."
	dstDir := containerCpPath(dstID, "/vol1/testDir")

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
	dstID = makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "", "/vol1/testDir/dir1/file1-1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "/vol1/testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, dstID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy across volumes create directory contents")
}

// I. SRC specifies a direcotry's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func TestCpWithVolCaseI(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-i")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "")
	defer deleteContainer(srcID)

	dstID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/root")
	defer deleteContainer(dstID)

	srcDir := containerCpPathTrailingSep(srcID, "/vol2/dir2") + "."
	dstFile := containerCpPath(dstID, "/vol2/file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy across volumes error directory contents can't replace file")
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func TestCpWithVolCaseJ(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-with-vol-case-j")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcID := makeTestContainer(t, false, defaultVolumes(tmpDir), "/vol2/vol3")
	defer deleteContainer(srcID)

	dstID := makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/root", "/vol1/file1-1")
	defer deleteContainer(dstID)

	srcDir := containerCpPathTrailingSep(srcID, "../dir1") + "."
	dstDir := containerCpPath(dstID, "../vol1")

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
	dstID = makeTestContainerCatFile(t, false, defaultVolumes(tmpDir), "/root", "/vol1/file1-1")
	defer deleteContainer(dstID)

	dstDir = containerCpPathTrailingSep(dstID, "../vol1")

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

	logDone("cp - copy across volumes directory contents into directory")
}
