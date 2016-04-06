// +build daemon,!windows,!experimental

package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-check/check"
)

// os.Kill should kill daemon ungracefully, leaving behind container mounts.
// A subsequent daemon restart shoud clean up said mounts.
func (s *DockerDaemonSuite) TestCleanupMountsAfterDaemonKill(c *check.C) {
	c.Assert(s.d.StartWithBusybox(), check.IsNil)

	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", out))
	id := strings.TrimSpace(out)
	c.Assert(s.d.cmd.Process.Signal(os.Kill), check.IsNil)
	mountOut, err := ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))

	// container mounts should exist even after daemon has crashed.
	comment := check.Commentf("%s should stay mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, true, comment)

	// restart daemon.
	if err := s.d.Restart(); err != nil {
		c.Fatal(err)
	}

	// Now, container mounts should be gone.
	mountOut, err = ioutil.ReadFile("/proc/self/mountinfo")
	c.Assert(err, check.IsNil, check.Commentf("Output: %s", mountOut))
	comment = check.Commentf("%s is still mounted from older daemon start:\nDaemon root repository %s\n%s", id, s.d.folder, mountOut)
	c.Assert(strings.Contains(string(mountOut), id), check.Equals, false, comment)
}
