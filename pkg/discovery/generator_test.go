package discovery // import "github.com/docker/docker/pkg/discovery"
import (
	"testing"

	"gotest.tools/v3/assert"
)

func (s *DiscoverySuite) TestGeneratorNotGenerate(c *testing.T) {
	ips := Generate("127.0.0.1")
	assert.Equal(c, len(ips), 1)
	assert.Equal(c, ips[0], "127.0.0.1")
}

func (s *DiscoverySuite) TestGeneratorWithPortNotGenerate(c *testing.T) {
	ips := Generate("127.0.0.1:8080")
	assert.Equal(c, len(ips), 1)
	assert.Equal(c, ips[0], "127.0.0.1:8080")
}

func (s *DiscoverySuite) TestGeneratorMatchFailedNotGenerate(c *testing.T) {
	ips := Generate("127.0.0.[1]")
	assert.Equal(c, len(ips), 1)
	assert.Equal(c, ips[0], "127.0.0.[1]")
}

func (s *DiscoverySuite) TestGeneratorWithPort(c *testing.T) {
	ips := Generate("127.0.0.[1:11]:2375")
	assert.Equal(c, len(ips), 11)
	assert.Equal(c, ips[0], "127.0.0.1:2375")
	assert.Equal(c, ips[1], "127.0.0.2:2375")
	assert.Equal(c, ips[2], "127.0.0.3:2375")
	assert.Equal(c, ips[3], "127.0.0.4:2375")
	assert.Equal(c, ips[4], "127.0.0.5:2375")
	assert.Equal(c, ips[5], "127.0.0.6:2375")
	assert.Equal(c, ips[6], "127.0.0.7:2375")
	assert.Equal(c, ips[7], "127.0.0.8:2375")
	assert.Equal(c, ips[8], "127.0.0.9:2375")
	assert.Equal(c, ips[9], "127.0.0.10:2375")
	assert.Equal(c, ips[10], "127.0.0.11:2375")
}

func (s *DiscoverySuite) TestGenerateWithMalformedInputAtRangeStart(c *testing.T) {
	malformedInput := "127.0.0.[x:11]:2375"
	ips := Generate(malformedInput)
	assert.Equal(c, len(ips), 1)
	assert.Equal(c, ips[0], malformedInput)
}

func (s *DiscoverySuite) TestGenerateWithMalformedInputAtRangeEnd(c *testing.T) {
	malformedInput := "127.0.0.[1:x]:2375"
	ips := Generate(malformedInput)
	assert.Equal(c, len(ips), 1)
	assert.Equal(c, ips[0], malformedInput)
}
