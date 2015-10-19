// +build experimental

package main

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestInspectNamedMountPoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "-v", "data:/data", "busybox", "cat")

	vol, err := inspectFieldJSON("test", "Mounts")
	c.Assert(err, checker.IsNil)

	var mp []types.MountPoint
	err = unmarshalJSON([]byte(vol), &mp)
	c.Assert(err, checker.IsNil)

	c.Assert(mp, checker.HasLen, 1, check.Commentf("Expected 1 mount point"))

	m := mp[0]
	c.Assert(m.Name, checker.Equals, "data", check.Commentf("Expected name data"))

	c.Assert(m.Driver, checker.Equals, "local", check.Commentf("Expected driver local"))

	c.Assert(m.Source, checker.Not(checker.Equals), "", check.Commentf("Expected source to not be empty"))

	c.Assert(m.RW, checker.Equals, true)

	c.Assert(m.Destination, checker.Equals, "/data", check.Commentf("Expected destination /data"))
}
