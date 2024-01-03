//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/docker/docker/integration-cli/cli"
	"gotest.tools/v3/assert"
)

func (s *DockerCLICpSuite) TestCpToContainerWithPermissions(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	tmpDir := getTestDir(c, "test-cp-to-host-with-permissions")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	containerName := "permtest"

	exc := cli.DockerCmd(c, "create", "--name", containerName, "busybox", "/bin/sh", "-c", "stat -c '%u %g %a' /permdirtest /permdirtest/permtest").ExitCode
	assert.Equal(c, exc, 0)
	defer cli.DockerCmd(c, "rm", "-f", containerName)

	srcPath := cpPath(tmpDir, "permdirtest")
	dstPath := containerCpPath(containerName, "/")

	args := []string{"cp", "-a", srcPath, dstPath}
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, args...))
	assert.NilError(c, err, "output: %v", out)

	out, err = startContainerGetOutput(c, containerName)
	assert.NilError(c, err, "output: %v", out)
	assert.Equal(c, strings.TrimSpace(out), "2 2 700\n65534 65534 400", "output: %v", out)
}

// Check ownership is root, both in non-userns and userns enabled modes
func (s *DockerCLICpSuite) TestCpCheckDestOwnership(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	tmpVolDir := getTestDir(c, "test-cp-tmpvol")
	containerID := makeTestContainer(c,
		testContainerOptions{volumes: []string{fmt.Sprintf("%s:/tmpvol", tmpVolDir)}})

	tmpDir := getTestDir(c, "test-cp-to-check-ownership")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(containerID, "/tmpvol", "file1")

	assert.NilError(c, runDockerCp(c, srcPath, dstPath))

	stat, err := os.Stat(filepath.Join(tmpVolDir, "file1"))
	assert.NilError(c, err)
	uid, gid, err := getRootUIDGID()
	assert.NilError(c, err)
	fi := stat.Sys().(*syscall.Stat_t)
	assert.Equal(c, fi.Uid, uint32(uid), "Copied file not owned by container root UID")
	assert.Equal(c, fi.Gid, uint32(gid), "Copied file not owned by container root GID")
}

func getRootUIDGID() (int, int, error) {
	uidgid := strings.Split(filepath.Base(testEnv.DaemonInfo.DockerRootDir), ".")
	if len(uidgid) == 1 {
		// user namespace remapping is not turned on; return 0
		return 0, 0, nil
	}
	uid, err := strconv.Atoi(uidgid[0])
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(uidgid[1])
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}
