// +build linux

package zfs

import (
	"testing"

	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestZfsSetup and TestZfsTeardown
func (s *DockerSuite) TestZfsSetup(c *check.C) {
	graphtest.GetDriver(c, "zfs")
}

func (s *DockerSuite) TestZfsCreateEmpty(c *check.C) {
	graphtest.DriverTestCreateEmpty(c, "zfs")
}

func (s *DockerSuite) TestZfsCreateBase(c *check.C) {
	graphtest.DriverTestCreateBase(c, "zfs")
}

func (s *DockerSuite) TestZfsCreateSnap(c *check.C) {
	graphtest.DriverTestCreateSnap(c, "zfs")
}

func (s *DockerSuite) TestZfsSetQuota(c *check.C) {
	graphtest.DriverTestSetQuota(c, "zfs")
}

func (s *DockerSuite) TestZfsTeardown(c *check.C) {
	graphtest.PutDriver(c)
}
