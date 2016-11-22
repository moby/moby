// +build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/system"
	"github.com/go-check/check"
)

// Check ownership is root, both in non-userns and userns enabled modes
func (s *DockerSuite) TestCpCheckDestOwnership(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)
	tmpVolDir := getTestDir(c, "test-cp-tmpvol")
	containerID := makeTestContainer(c,
		testContainerOptions{volumes: []string{fmt.Sprintf("%s:/tmpvol", tmpVolDir)}})

	tmpDir := getTestDir(c, "test-cp-to-check-ownership")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := cpPath(tmpDir, "file1")
	dstPath := containerCpPath(containerID, "/tmpvol", "file1")

	err := runDockerCp(c, srcPath, dstPath)
	c.Assert(err, checker.IsNil)

	stat, err := system.Stat(filepath.Join(tmpVolDir, "file1"))
	c.Assert(err, checker.IsNil)
	uid, gid, err := getRootUIDGID()
	c.Assert(err, checker.IsNil)
	c.Assert(stat.UID(), checker.Equals, uint32(uid), check.Commentf("Copied file not owned by container root UID"))
	c.Assert(stat.GID(), checker.Equals, uint32(gid), check.Commentf("Copied file not owned by container root GID"))
}
