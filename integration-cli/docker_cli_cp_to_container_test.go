package main

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

// Try all of the test cases from the archive package which implements the
// internals of `docker cp` and ensure that the behavior matches when actually
// copying to and from containers.

// Basic assumptions about SRC and DST:
// 1. SRC must exist.
// 2. If SRC ends with a trailing separator, it must be a directory.
// 3. DST parent directory must exist.
// 4. If DST exists as a file, it must not end with a trailing separator.

// Check that copying from a local path to a symlink in a container copies to
// the symlink target and does not overwrite the container symlink itself.
func (s *DockerCLICpSuite) TestCpToSymlinkDestination(c *testing.T) {
	//  stat /tmp/test-cp-to-symlink-destination-262430901/vol3 gets permission denied for the user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	testRequires(c, testEnv.IsLocalDaemon) // Requires local volume mount bind.

	testVol := getTestDir(c, "test-cp-to-symlink-destination-")
	defer os.RemoveAll(testVol)

	makeTestContentInDir(c, testVol)

	containerID := makeTestContainer(c, testContainerOptions{
		volumes: defaultVolumes(testVol), // Our bind mount is at /vol2
	})

	// First, copy a local file to a symlink to a file in the container. This
	// should overwrite the symlink target contents with the source contents.
	srcPath := cpPath(testVol, "file2")
	dstPath := containerCpPath(containerID, "/vol2/symlinkToFile1")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, symlinkTargetEquals(c, cpPath(testVol, "symlinkToFile1"), "file1"), "The symlink should not have been modified")
	assert.NilError(c, fileContentEquals(c, cpPath(testVol, "file1"), "file2\n"), `The file should have the contents of "file2" now`)

	// Next, copy a local file to a symlink to a directory in the container.
	// This should copy the file into the symlink target directory.
	dstPath = containerCpPath(containerID, "/vol2/symlinkToDir1")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, symlinkTargetEquals(c, cpPath(testVol, "symlinkToDir1"), "dir1"), "The symlink should not have been modified")
	assert.NilError(c, fileContentEquals(c, cpPath(testVol, "file2"), "file2\n"), `The file should have the contents of "file2"" now`)

	// Next, copy a file to a symlink to a file that does not exist (a broken
	// symlink) in the container. This should create the target file with the
	// contents of the source file.
	dstPath = containerCpPath(containerID, "/vol2/brokenSymlinkToFileX")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, symlinkTargetEquals(c, cpPath(testVol, "brokenSymlinkToFileX"), "fileX"), "The symlink should not have been modified")
	assert.NilError(c, fileContentEquals(c, cpPath(testVol, "fileX"), "file2\n"), `The file should have the contents of "file2"" now`)

	// Next, copy a local directory to a symlink to a directory in the
	// container. This should copy the directory into the symlink target
	// directory and not modify the symlink.
	srcPath = cpPath(testVol, "/dir2")
	dstPath = containerCpPath(containerID, "/vol2/symlinkToDir1")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, symlinkTargetEquals(c, cpPath(testVol, "symlinkToDir1"), "dir1"), "The symlink should not have been modified")
	assert.NilError(c, fileContentEquals(c, cpPath(testVol, "dir1/dir2/file2-1"), "file2-1\n"), `The directory should now contain a copy of "dir2"`)

	// Next, copy a local directory to a symlink to a local directory that does
	// not exist (a broken symlink) in the container. This should create the
	// target as a directory with the contents of the source directory. It
	// should not modify the symlink.
	dstPath = containerCpPath(containerID, "/vol2/brokenSymlinkToDirX")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, symlinkTargetEquals(c, cpPath(testVol, "brokenSymlinkToDirX"), "dirX"), "The symlink should not have been modified")
	assert.NilError(c, fileContentEquals(c, cpPath(testVol, "dirX/file2-1"), "file2-1\n"), `The "dirX" directory should now be a copy of "dir2"`)
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
func (s *DockerCLICpSuite) TestCpToCaseA(c *testing.T) {
	containerID := makeTestContainer(c, testContainerOptions{
		workDir: "/root", command: makeCatFileCommand("itWorks.txt"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-a")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(containerID, "/root/itWorks.txt")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1\n"))
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func (s *DockerCLICpSuite) TestCpToCaseB(c *testing.T) {
	containerID := makeTestContainer(c, testContainerOptions{
		command: makeCatFileCommand("testDir/file1"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-b")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstDir := containerCpPathTrailingSep(containerID, "testDir")

	err := runDockerCp(c, srcPath, dstDir)
	assert.ErrorContains(c, err, "")
	assert.Assert(c, isCpDirNotExist(err), "expected DirNotExists error, but got %T: %s", err, err)
}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func (s *DockerCLICpSuite) TestCpToCaseC(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
		command: makeCatFileCommand("file2"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-c")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(containerID, "/root/file2")

	// Ensure the container's file starts with the original content.
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file2\n"))
	assert.NilError(c, runDockerCp(c, srcPath, dstPath))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1\n"), "Should now contain file1's contents")
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerCLICpSuite) TestCpToCaseD(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true,
		command:    makeCatFileCommand("/dir1/file1"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-d")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstDir := containerCpPath(containerID, "dir1")

	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
	assert.NilError(c, runDockerCp(c, srcPath, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1\n"), "Should now contain file1's contents")

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	containerID = makeTestContainer(c, testContainerOptions{
		addContent: true,
		command:    makeCatFileCommand("/dir1/file1"),
	})

	dstDir = containerCpPathTrailingSep(containerID, "dir1")

	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
	assert.NilError(c, runDockerCp(c, srcPath, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1\n"), "Should now contain file1's contents")
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func (s *DockerCLICpSuite) TestCpToCaseE(c *testing.T) {
	containerID := makeTestContainer(c, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-e")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := cpPath(tmpDir, "dir1")
	dstDir := containerCpPath(containerID, "testDir")

	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should now contain file1-1's contents")

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	containerID = makeTestContainer(c, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})

	dstDir = containerCpPathTrailingSep(containerID, "testDir")

	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should now contain file1-1's contents")
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func (s *DockerCLICpSuite) TestCpToCaseF(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-to-case-f")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := cpPath(tmpDir, "dir1")
	dstFile := containerCpPath(containerID, "/root/file1")

	err := runDockerCp(c, srcDir, dstFile)
	assert.ErrorContains(c, err, "")
	assert.Assert(c, isCpCannotCopyDir(err), "expected ErrCannotCopyDir error, but got %T: %s", err, err)
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func (s *DockerCLICpSuite) TestCpToCaseG(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
		command: makeCatFileCommand("dir2/dir1/file1-1"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-g")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := cpPath(tmpDir, "dir1")
	dstDir := containerCpPath(containerID, "/root/dir2")

	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should now contain file1-1's contents")

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	containerID = makeTestContainer(c, testContainerOptions{
		addContent: true,
		command:    makeCatFileCommand("/dir2/dir1/file1-1"),
	})

	dstDir = containerCpPathTrailingSep(containerID, "/dir2")

	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should now contain file1-1's contents")
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerCLICpSuite) TestCpToCaseH(c *testing.T) {
	containerID := makeTestContainer(c, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-h")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := cpPathTrailingSep(tmpDir, "dir1") + "."
	dstDir := containerCpPath(containerID, "testDir")

	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should now contain file1-1's contents")

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	containerID = makeTestContainer(c, testContainerOptions{
		command: makeCatFileCommand("/testDir/file1-1"),
	})

	dstDir = containerCpPathTrailingSep(containerID, "testDir")

	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should now contain file1-1's contents")
}

// I. SRC specifies a directory's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func (s *DockerCLICpSuite) TestCpToCaseI(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-to-case-i")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := cpPathTrailingSep(tmpDir, "dir1") + "."
	dstFile := containerCpPath(containerID, "/root/file1")

	err := runDockerCp(c, srcDir, dstFile)
	assert.ErrorContains(c, err, "")
	assert.Assert(c, isCpCannotCopyDir(err), "expected ErrCannotCopyDir error, but got %T: %s", err, err)
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func (s *DockerCLICpSuite) TestCpToCaseJ(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
		command: makeCatFileCommand("/dir2/file1-1"),
	})

	tmpDir := getTestDir(c, "test-cp-to-case-j")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := cpPathTrailingSep(tmpDir, "dir1") + "."
	dstDir := containerCpPath(containerID, "/dir2")

	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should've contained file1-1's contents")

	// Now try again but using a trailing path separator for dstDir.

	// Make new destination container.
	containerID = makeTestContainer(c, testContainerOptions{
		command: makeCatFileCommand("/dir2/file1-1"),
	})

	dstDir = containerCpPathTrailingSep(containerID, "/dir2")

	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
	assert.NilError(c, runDockerCp(c, srcDir, dstDir))
	assert.NilError(c, containerStartOutputEquals(c, containerID, "file1-1\n"), "Should've contained file1-1's contents")
}

// The `docker cp` command should also ensure that you cannot
// write to a container rootfs that is marked as read-only.
func (s *DockerCLICpSuite) TestCpToErrReadOnlyRootfs(c *testing.T) {
	// --read-only + userns has remount issues
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	tmpDir := getTestDir(c, "test-cp-to-err-read-only-rootfs")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	containerID := makeTestContainer(c, testContainerOptions{
		readOnly: true, workDir: "/root",
		command: makeCatFileCommand("shouldNotExist"),
	})

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(containerID, "/root/shouldNotExist")

	err := runDockerCp(c, srcPath, dstPath)
	assert.ErrorContains(c, err, "")

	assert.Assert(c, isCpCannotCopyReadOnly(err), "expected ErrContainerRootfsReadonly error, but got %T: %s", err, err)
	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
}

// The `docker cp` command should also ensure that you
// cannot write to a volume that is mounted as read-only.
func (s *DockerCLICpSuite) TestCpToErrReadOnlyVolume(c *testing.T) {
	// --read-only + userns has remount issues
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	tmpDir := getTestDir(c, "test-cp-to-err-read-only-volume")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	containerID := makeTestContainer(c, testContainerOptions{
		volumes: defaultVolumes(tmpDir), workDir: "/root",
		command: makeCatFileCommand("/vol_ro/shouldNotExist"),
	})

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(containerID, "/vol_ro/shouldNotExist")

	err := runDockerCp(c, srcPath, dstPath)
	assert.ErrorContains(c, err, "")

	assert.Assert(c, isCpCannotCopyReadOnly(err), "expected ErrVolumeReadonly error, but got %T: %s", err, err)
	assert.NilError(c, containerStartOutputEquals(c, containerID, ""), "dstPath should not have existed")
}
