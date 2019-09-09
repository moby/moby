package discovery // import "github.com/docker/docker/pkg/discovery"

import (
	"github.com/go-check/check"
)

func (s *DiscoverySuite) TestGeneratorNotGenerate(c *testing.T) {
	ips := Generate("127.0.0.1")
	assert.Assert(c, len(ips), checker.Equals, 1)
	assert.Assert(c, ips[0], checker.Equals, "127.0.0.1")
}

func (s *DiscoverySuite) TestGeneratorWithPortNotGenerate(c *testing.T) {
	ips := Generate("127.0.0.1:8080")
	assert.Assert(c, len(ips), checker.Equals, 1)
	assert.Assert(c, ips[0], checker.Equals, "127.0.0.1:8080")
}

func (s *DiscoverySuite) TestGeneratorMatchFailedNotGenerate(c *testing.T) {
	ips := Generate("127.0.0.[1]")
	assert.Assert(c, len(ips), checker.Equals, 1)
	assert.Assert(c, ips[0], checker.Equals, "127.0.0.[1]")
}

func (s *DiscoverySuite) TestGeneratorWithPort(c *testing.T) {
	ips := Generate("127.0.0.[1:11]:2375")
	assert.Assert(c, len(ips), checker.Equals, 11)
	assert.Assert(c, ips[0], checker.Equals, "127.0.0.1:2375")
	assert.Assert(c, ips[1], checker.Equals, "127.0.0.2:2375")
	assert.Assert(c, ips[2], checker.Equals, "127.0.0.3:2375")
	assert.Assert(c, ips[3], checker.Equals, "127.0.0.4:2375")
	assert.Assert(c, ips[4], checker.Equals, "127.0.0.5:2375")
	assert.Assert(c, ips[5], checker.Equals, "127.0.0.6:2375")
	assert.Assert(c, ips[6], checker.Equals, "127.0.0.7:2375")
	assert.Assert(c, ips[7], checker.Equals, "127.0.0.8:2375")
	assert.Assert(c, ips[8], checker.Equals, "127.0.0.9:2375")
	assert.Assert(c, ips[9], checker.Equals, "127.0.0.10:2375")
	assert.Assert(c, ips[10], checker.Equals, "127.0.0.11:2375")
}

func (s *DiscoverySuite) TestGenerateWithMalformedInputAtRangeStart(c *testing.T) {
	malformedInput := "127.0.0.[x:11]:2375"
	ips := Generate(malformedInput)
	assert.Assert(c, len(ips), checker.Equals, 1)
	assert.Assert(c, ips[0], checker.Equals, malformedInput)
}

func (s *DiscoverySuite) TestGenerateWithMalformedInputAtRangeEnd(c *testing.T) {
	malformedInput := "127.0.0.[1:x]:2375"
	ips := Generate(malformedInput)
	assert.Assert(c, len(ips), checker.Equals, 1)
	assert.Assert(c, ips[0], checker.Equals, malformedInput)
}
