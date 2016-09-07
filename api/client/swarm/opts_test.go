package swarm

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestNodeAddrOptionSetHostAndPort(c *check.C) {
	opt := NewNodeAddrOption("old:123")
	addr := "newhost:5555"
	assert.NilError(c, opt.Set(addr))
	assert.Equal(c, opt.Value(), addr)
}

func (s *DockerSuite) TestNodeAddrOptionSetHostOnly(c *check.C) {
	opt := NewListenAddrOption()
	assert.NilError(c, opt.Set("newhost"))
	assert.Equal(c, opt.Value(), "newhost:2377")
}

func (s *DockerSuite) TestNodeAddrOptionSetHostOnlyIPv6(c *check.C) {
	opt := NewListenAddrOption()
	assert.NilError(c, opt.Set("::1"))
	assert.Equal(c, opt.Value(), "[::1]:2377")
}

func (s *DockerSuite) TestNodeAddrOptionSetPortOnly(c *check.C) {
	opt := NewListenAddrOption()
	assert.NilError(c, opt.Set(":4545"))
	assert.Equal(c, opt.Value(), "0.0.0.0:4545")
}

func (s *DockerSuite) TestNodeAddrOptionSetInvalidFormat(c *check.C) {
	opt := NewListenAddrOption()
	assert.Error(c, opt.Set("http://localhost:4545"), "Invalid")
}
