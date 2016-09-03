// +build linux

package vfs

import (
	"testing"

	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/docker/docker/pkg/reexec"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func init() {
	reexec.Init()
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestVfsSetup and TestVfsTeardown
func (s *DockerSuite) TestVfsSetup(c *check.C) {
	graphtest.GetDriver(c, "vfs")
}

func (s *DockerSuite) TestVfsCreateEmpty(c *check.C) {
	graphtest.DriverTestCreateEmpty(c, "vfs")
}

func (s *DockerSuite) TestVfsCreateBase(c *check.C) {
	graphtest.DriverTestCreateBase(c, "vfs")
}

func (s *DockerSuite) TestVfsCreateSnap(c *check.C) {
	graphtest.DriverTestCreateSnap(c, "vfs")
}

func (s *DockerSuite) TestVfsTeardown(c *check.C) {
	graphtest.PutDriver(c)
}
