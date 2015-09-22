// +build experimental

package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestRunVolumeBindMountUserChowned(c *check.C) {
	testRequires(c, DaemonIsLinux)

	tmpDir, err := ioutil.TempDir("", "tmpchown")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	out, _ := dockerCmd(c, "run", "-v", tmpDir+":/tmp:u", "-u", "1", "busybox", "ls", "-la", "/tmp")
	if !strings.Contains(out, "daemon") {
		c.Fatalf("expected dir owned by user 1 (daemon), got %s", out)
	}
}
