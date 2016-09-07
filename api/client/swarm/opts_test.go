package swarm

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestNodeAddrOptionSetHostAndPort(c *check.C) {
	opt := NewNodeAddrOption("old:123")
	addr := "newhost:5555"
	c.Assert(opt.Set(addr), check.IsNil)
	c.Assert(opt.Value(), check.Equals, addr)
}

func (s *DockerSuite) TestNodeAddrOptionSetHostOnly(c *check.C) {
	opt := NewListenAddrOption()
	c.Assert(opt.Set("newhost"), check.IsNil)
	c.Assert(opt.Value(), check.Equals, "newhost:2377")
}

func (s *DockerSuite) TestNodeAddrOptionSetHostOnlyIPv6(c *check.C) {
	opt := NewListenAddrOption()
	c.Assert(opt.Set("::1"), check.IsNil)
	c.Assert(opt.Value(), check.Equals, "[::1]:2377")
}

func (s *DockerSuite) TestNodeAddrOptionSetPortOnly(c *check.C) {
	opt := NewListenAddrOption()
	c.Assert(opt.Set(":4545"), check.IsNil)
	c.Assert(opt.Value(), check.Equals, "0.0.0.0:4545")
}

func (s *DockerSuite) TestNodeAddrOptionSetInvalidFormat(c *check.C) {
	opt := NewListenAddrOption()
	c.Assert(opt.Set("http://localhost:4545"), check.ErrorMatches, ".*Invalid.*")
}
