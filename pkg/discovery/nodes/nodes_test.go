package nodes // import "github.com/docker/docker/pkg/discovery/nodes"

import (
	"testing"

	"github.com/docker/docker/internal/test/suite"
	"github.com/docker/docker/pkg/discovery"
	"gotest.tools/v3/assert"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	suite.Run(t, &DiscoverySuite{})
}

type DiscoverySuite struct{}

func (s *DiscoverySuite) TestInitialize(c *testing.T) {
	d := &Discovery{}
	d.Initialize("1.1.1.1:1111,2.2.2.2:2222", 0, 0, nil)
	assert.Equal(c, len(d.entries), 2)
	assert.Equal(c, d.entries[0].String(), "1.1.1.1:1111")
	assert.Equal(c, d.entries[1].String(), "2.2.2.2:2222")
}

func (s *DiscoverySuite) TestInitializeWithPattern(c *testing.T) {
	d := &Discovery{}
	d.Initialize("1.1.1.[1:2]:1111,2.2.2.[2:4]:2222", 0, 0, nil)
	assert.Equal(c, len(d.entries), 5)
	assert.Equal(c, d.entries[0].String(), "1.1.1.1:1111")
	assert.Equal(c, d.entries[1].String(), "1.1.1.2:1111")
	assert.Equal(c, d.entries[2].String(), "2.2.2.2:2222")
	assert.Equal(c, d.entries[3].String(), "2.2.2.3:2222")
	assert.Equal(c, d.entries[4].String(), "2.2.2.4:2222")
}

func (s *DiscoverySuite) TestWatch(c *testing.T) {
	d := &Discovery{}
	d.Initialize("1.1.1.1:1111,2.2.2.2:2222", 0, 0, nil)
	expected := discovery.Entries{
		&discovery.Entry{Host: "1.1.1.1", Port: "1111"},
		&discovery.Entry{Host: "2.2.2.2", Port: "2222"},
	}
	ch, _ := d.Watch(nil)
	assert.Equal(c, expected.Equals(<-ch), true)
}

func (s *DiscoverySuite) TestRegister(c *testing.T) {
	d := &Discovery{}
	assert.Assert(c, d.Register("0.0.0.0") != nil)
}
