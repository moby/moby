package file

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	discovery := &Discovery{}
	discovery.Initialize("/path/to/file", 0)
	assert.Equal(t, discovery.path, "/path/to/file")
}

func TestContent(t *testing.T) {
	data := `
1.1.1.[1:2]:1111
2.2.2.[2:4]:2222
`
	ips := parseFileContent([]byte(data))
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
	assert.Equal(t, 2, len(ips))
	assert.Equal(t, "1.1.1.1:1111", ips[0])
	assert.Equal(t, "3.3.3.3:3333", ips[1])
}
