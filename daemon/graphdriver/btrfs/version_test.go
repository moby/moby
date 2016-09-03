// +build linux,!btrfs_noversion

package btrfs

import "github.com/go-check/check"

func (s *DockerSuite) TestLibVersion(c *check.C) {
	if btrfsLibVersion() <= 0 {
		c.Errorf("expected output from btrfs lib version > 0")
	}
}
