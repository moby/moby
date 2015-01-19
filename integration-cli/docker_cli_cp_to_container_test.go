package main

import (
	"os"
	"testing"
)

// docker cp LOCALPATH CONTAINER:PATH

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
func TestCpToErrSrcNotExists(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-err-src-not-exists")
	defer os.RemoveAll(tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(cID, "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error source not exists")
}

// Test for error when SRC ends in a trailing
// path separator but it exists as a file.
func TestCpToErrSrcNotDir(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-err-src-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := cpPathTrailingSep(tmpDir, "file1")
	dstPath := containerCpPath(cID, "testDir")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error source not directory")
}

// Test for error when SRC is a valid file or directory,
// bu the DST parent directory does not exist.
func TestCpToErrDstParentNotExists(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-err-dst-parent-not-exists")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	// Try with a file source.
	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(cID, "/notExists", "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcPath = cpPath(tmpDir, "dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotExist error, but got nil instead")
	}

	if !isCpNotExist(err) {
		t.Fatalf("expected IsNotExist error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error destination parent not exists")
}

// Test for error when DST ends in a trailing
// path separator but exists as a file.
func TestCpToErrDstNotDir(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{addContent: true})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-err-dst-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	// Try with a file source.
	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPathTrailingSep(cID, "file1")

	err := runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	// Try with a directory source.
	srcPath = cpPath(tmpDir, "dir1")

	err = runDockerCp(t, srcPath, dstPath)
	if err == nil {
		t.Fatal("expected IsNotDir error, but got nil instead")
	}

	if !isCpNotDir(err) {
		t.Fatalf("expected IsNotDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error destination not directory")
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
func TestCpToCaseA(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		workDir: "/root", command: makeCatFileCommand("itWorks.txt"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-a")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(cID, "itWorks.txt")

	var err error

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	if err = containerStartOutputEquals(t, cID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container create file")
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func TestCpToCaseB(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		command: makeCatFileCommand("testDir/file1"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-b")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstDir := containerCpPathTrailingSep(cID, "testDir")

	var err error

	if err = runDockerCp(t, srcPath, dstDir); err == nil {
		t.Fatal("expected DirNotExists error, but got nil instead")
	}

	if !isCpDirNotExist(err) {
		t.Fatalf("expected DirNotExists error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error file copy can't create dir")
}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func TestCpToCaseC(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
		command: makeCatFileCommand("file2"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-c")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(cID, "file2")

	var err error

	// Ensure the container's file starts with the original content.
	if err = containerStartOutputEquals(t, cID, "file2\n"); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstPath); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, cID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container overwrite file")
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpToCaseD(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true,
		command:    makeCatFileCommand("/dir1/file1"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-d")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstDir := containerCpPath(cID, "dir1")

	var err error

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, cID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	cID = makeTestContainer(t, testContainerOptions{
		addContent: true,
		command:    makeCatFileCommand("/dir1/file1"),
	})
	defer deleteContainer(cID)

	dstDir = containerCpPathTrailingSep(cID, "dir1")

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcPath, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1's contents.
	if err = containerStartOutputEquals(t, cID, "file1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container file to directory")
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func TestCpToCaseE(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-e")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := cpPath(tmpDir, "dir1")
	dstDir := containerCpPath(cID, "testDir")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	cID = makeTestContainer(t, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})
	defer deleteContainer(cID)

	dstDir = containerCpPathTrailingSep(cID, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container create directory")
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func TestCpToCaseF(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-f")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := cpPath(tmpDir, "dir1")
	dstFile := containerCpPath(cID, "file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if !isCpCannotCopyDir(err) {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error directory can't replace file")
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func TestCpToCaseG(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
		command: makeCatFileCommand("dir2/dir1/file1-1"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-g")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := cpPath(tmpDir, "dir1")
	dstDir := containerCpPath(cID, "/root/dir2")

	var err error

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	cID = makeTestContainer(t, testContainerOptions{
		addContent: true,
		command:    makeCatFileCommand("/dir2/dir1/file1-1"),
	})
	defer deleteContainer(cID)

	dstDir = containerCpPathTrailingSep(cID, "/dir2")

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container directory into directory")
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func TestCpToCaseH(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-h")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := cpPathTrailingSep(tmpDir, "dir1") + "."
	dstDir := containerCpPath(cID, "testDir")

	var err error

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	cID = makeTestContainer(t, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})
	defer deleteContainer(cID)

	dstDir = containerCpPathTrailingSep(cID, "testDir")

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container create directory contents")
}

// I. SRC specifies a direcotry's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func TestCpToCaseI(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-i")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := cpPathTrailingSep(tmpDir, "dir1") + "."
	dstFile := containerCpPath(cID, "file1")

	var err error

	if err = runDockerCp(t, srcDir, dstFile); err == nil {
		t.Fatal("expected ErrCannotCopyDir error, but got nil instead")
	}

	if !isCpCannotCopyDir(err) {
		t.Fatalf("expected ErrCannotCopyDir error, but got %T: %s", err, err)
	}

	logDone("cp - copy to container error directory contents can't replace file")
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func TestCpToCaseJ(t *testing.T) {
	cID := makeTestContainer(t, testContainerOptions{
		addContent: true, workDir: "/root",
		command: makeCatFileCommand("/dir2/file1-1"),
	})
	defer deleteContainer(cID)

	tmpDir := getTestDir(t, "test-cp-to-case-j")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	srcDir := cpPathTrailingSep(tmpDir, "dir1") + "."
	dstDir := containerCpPath(cID, "/dir2")

	var err error

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	cID = makeTestContainer(t, testContainerOptions{
		command: makeCatFileCommand("/dir2/file1-1"),
	})
	defer deleteContainer(cID)

	dstDir = containerCpPathTrailingSep(cID, "/dir2")

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	if err = runDockerCp(t, srcDir, dstDir); err != nil {
		t.Fatal("unexpected error %T: %s", err, err)
	}

	// Should now contain file1-1's contents.
	if err = containerStartOutputEquals(t, cID, "file1-1\n"); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container directory contents into directory")
}

// The `docker cp` command should also ensure that you cannot
// write to a container rootfs that is marked as read-only.
func TestCpToErrReadOnlyRootfs(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-to-err-read-only-rootfs")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	cID := makeTestContainer(t, testContainerOptions{
		readOnly: true, workDir: "/root",
		command: makeCatFileCommand("shouldNotExist"),
	})
	defer deleteContainer(cID)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(cID, "shouldNotExist")

	var err error

	if err = runDockerCp(t, srcPath, dstPath); err == nil {
		t.Fatal("expected ErrContainerRootfsReadonly error, but got nil instead")
	}

	if !isCpCannotCopyReadOnly(err) {
		t.Fatalf("expected ErrContainerRootfsReadonly error, but got %T: %s", err, err)
	}

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container error read-only rootfs")
}

// The `docker cp` command should also ensure that you
// cannot write to a volume that is mounted as read-only.
func TestCpToErrReadOnlyVolume(t *testing.T) {
	tmpDir := getTestDir(t, "test-cp-to-err-read-only-volume")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(t, tmpDir)

	cID := makeTestContainer(t, testContainerOptions{
		volumes: defaultVolumes(tmpDir), workDir: "/root",
		command: makeCatFileCommand("/vol_ro/shouldNotExist"),
	})
	defer deleteContainer(cID)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(cID, "/vol_ro/shouldNotExist")

	var err error

	if err = runDockerCp(t, srcPath, dstPath); err == nil {
		t.Fatal("expected ErrMountReadonly error, but got nil instead")
	}

	if !isCpCannotCopyReadOnly(err) {
		t.Fatalf("expected ErrMountReadonly error, but got %T: %s", err, err)
	}

	// Ensure that dstPath doesn't exist.
	if err = containerStartOutputEquals(t, cID, ""); err != nil {
		t.Fatal(err)
	}

	logDone("cp - copy to container error read-only volume")
}
