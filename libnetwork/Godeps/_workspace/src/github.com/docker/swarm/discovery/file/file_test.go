package file

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/swarm/discovery"
	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	d := &Discovery{}
	d.Initialize("/path/to/file", 1000, 0)
	assert.Equal(t, d.path, "/path/to/file")
}

func TestNew(t *testing.T) {
	d, err := discovery.New("file:///path/to/file", 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, d.(*Discovery).path, "/path/to/file")
}

func TestContent(t *testing.T) {
	data := `
1.1.1.[1:2]:1111
2.2.2.[2:4]:2222
`
	ips := parseFileContent([]byte(data))
	assert.Len(t, ips, 5)
	assert.Equal(t, ips[0], "1.1.1.1:1111")
	assert.Equal(t, ips[1], "1.1.1.2:1111")
	assert.Equal(t, ips[2], "2.2.2.2:2222")
	assert.Equal(t, ips[3], "2.2.2.3:2222")
	assert.Equal(t, ips[4], "2.2.2.4:2222")
}

func TestRegister(t *testing.T) {
	discovery := &Discovery{path: "/path/to/file"}
	assert.Error(t, discovery.Register("0.0.0.0"))
}

func TestParsingContentsWithComments(t *testing.T) {
	data := `
### test ###
1.1.1.1:1111 # inline comment
# 2.2.2.2:2222
      ### empty line with comment
    3.3.3.3:3333
### test ###
`
	ips := parseFileContent([]byte(data))
	assert.Len(t, ips, 2)
	assert.Equal(t, "1.1.1.1:1111", ips[0])
	assert.Equal(t, "3.3.3.3:3333", ips[1])
}

func TestWatch(t *testing.T) {
	data := `
1.1.1.1:1111
2.2.2.2:2222
`
	expected := discovery.Entries{
		&discovery.Entry{Host: "1.1.1.1", Port: "1111"},
		&discovery.Entry{Host: "2.2.2.2", Port: "2222"},
	}

	// Create a temporary file and remove it.
	tmp, err := ioutil.TempFile(os.TempDir(), "discovery-file-test")
	assert.NoError(t, err)
	assert.NoError(t, tmp.Close())
	assert.NoError(t, os.Remove(tmp.Name()))

	// Set up file discovery.
	d := &Discovery{}
	d.Initialize(tmp.Name(), 1000, 0)
	stopCh := make(chan struct{})
	ch, errCh := d.Watch(stopCh)

	// Make sure it fires errors since the file doesn't exist.
	assert.Error(t, <-errCh)
	// We have to drain the error channel otherwise Watch will get stuck.
	go func() {
		for _ = range errCh {
		}
	}()

	// Write the file and make sure we get the expected value back.
	assert.NoError(t, ioutil.WriteFile(tmp.Name(), []byte(data), 0600))
	assert.Equal(t, expected, <-ch)

	// Add a new entry and look it up.
	expected = append(expected, &discovery.Entry{Host: "3.3.3.3", Port: "3333"})
	f, err := os.OpenFile(tmp.Name(), os.O_APPEND|os.O_WRONLY, 0600)
	assert.NoError(t, err)
	assert.NotNil(t, f)
	_, err = f.WriteString("\n3.3.3.3:3333\n")
	assert.NoError(t, err)
	f.Close()
	assert.Equal(t, expected, <-ch)

	// Stop and make sure it closes all channels.
	close(stopCh)
	assert.Nil(t, <-ch)
	assert.Nil(t, <-errCh)
}
