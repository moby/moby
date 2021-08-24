package file // import "github.com/docker/docker/pkg/discovery/file"

import (
	"os"
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
	d.Initialize("/path/to/file", 1000, 0, nil)
	assert.Equal(c, d.path, "/path/to/file")
}

func (s *DiscoverySuite) TestNew(c *testing.T) {
	d, err := discovery.New("file:///path/to/file", 0, 0, nil)
	assert.Assert(c, err == nil)
	assert.Equal(c, d.(*Discovery).path, "/path/to/file")
}

func (s *DiscoverySuite) TestContent(c *testing.T) {
	data := `
1.1.1.[1:2]:1111
2.2.2.[2:4]:2222
`
	ips := parseFileContent([]byte(data))
	assert.Equal(c, len(ips), 5)
	assert.Equal(c, ips[0], "1.1.1.1:1111")
	assert.Equal(c, ips[1], "1.1.1.2:1111")
	assert.Equal(c, ips[2], "2.2.2.2:2222")
	assert.Equal(c, ips[3], "2.2.2.3:2222")
	assert.Equal(c, ips[4], "2.2.2.4:2222")
}

func (s *DiscoverySuite) TestRegister(c *testing.T) {
	discovery := &Discovery{path: "/path/to/file"}
	assert.Assert(c, discovery.Register("0.0.0.0") != nil)
}

func (s *DiscoverySuite) TestParsingContentsWithComments(c *testing.T) {
	data := `
### test ###
1.1.1.1:1111 # inline comment
# 2.2.2.2:2222
      ### empty line with comment
    3.3.3.3:3333
### test ###
`
	ips := parseFileContent([]byte(data))
	assert.Equal(c, len(ips), 2)
	assert.Equal(c, "1.1.1.1:1111", ips[0])
	assert.Equal(c, "3.3.3.3:3333", ips[1])
}

func (s *DiscoverySuite) TestWatch(c *testing.T) {
	data := `
1.1.1.1:1111
2.2.2.2:2222
`
	expected := discovery.Entries{
		&discovery.Entry{Host: "1.1.1.1", Port: "1111"},
		&discovery.Entry{Host: "2.2.2.2", Port: "2222"},
	}

	// Create a temporary file and remove it.
	tmp, err := os.CreateTemp(os.TempDir(), "discovery-file-test")
	assert.Assert(c, err == nil)
	assert.Assert(c, tmp.Close() == nil)
	assert.Assert(c, os.Remove(tmp.Name()) == nil)

	// Set up file discovery.
	d := &Discovery{}
	d.Initialize(tmp.Name(), 1000, 0, nil)
	stopCh := make(chan struct{})
	ch, errCh := d.Watch(stopCh)

	// Make sure it fires errors since the file doesn't exist.
	assert.Assert(c, <-errCh != nil)
	// We have to drain the error channel otherwise Watch will get stuck.
	go func() {
		for range errCh {
		}
	}()

	// Write the file and make sure we get the expected value back.
	assert.Assert(c, os.WriteFile(tmp.Name(), []byte(data), 0600) == nil)
	assert.DeepEqual(c, <-ch, expected)

	// Add a new entry and look it up.
	expected = append(expected, &discovery.Entry{Host: "3.3.3.3", Port: "3333"})
	f, err := os.OpenFile(tmp.Name(), os.O_APPEND|os.O_WRONLY, 0600)
	assert.Assert(c, err == nil)
	assert.Assert(c, f != nil)
	_, err = f.WriteString("\n3.3.3.3:3333\n")
	assert.Assert(c, err == nil)
	f.Close()
	assert.DeepEqual(c, <-ch, expected)

	// Stop and make sure it closes all channels.
	close(stopCh)
	assert.Assert(c, <-ch == nil)
	assert.Assert(c, <-errCh == nil)
}
