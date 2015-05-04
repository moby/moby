package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratorNotGenerate(t *testing.T) {
	ips := Generate("127.0.0.1")
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0], "127.0.0.1")
}

func TestGeneratorWithPortNotGenerate(t *testing.T) {
	ips := Generate("127.0.0.1:8080")
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0], "127.0.0.1:8080")
}

func TestGeneratorMatchFailedNotGenerate(t *testing.T) {
	ips := Generate("127.0.0.[1]")
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0], "127.0.0.[1]")
}

func TestGeneratorWithPort(t *testing.T) {
	ips := Generate("127.0.0.[1:11]:2375")
	assert.Equal(t, len(ips), 11)
	assert.Equal(t, ips[0], "127.0.0.1:2375")
	assert.Equal(t, ips[1], "127.0.0.2:2375")
	assert.Equal(t, ips[2], "127.0.0.3:2375")
	assert.Equal(t, ips[3], "127.0.0.4:2375")
	assert.Equal(t, ips[4], "127.0.0.5:2375")
	assert.Equal(t, ips[5], "127.0.0.6:2375")
	assert.Equal(t, ips[6], "127.0.0.7:2375")
	assert.Equal(t, ips[7], "127.0.0.8:2375")
	assert.Equal(t, ips[8], "127.0.0.9:2375")
	assert.Equal(t, ips[9], "127.0.0.10:2375")
	assert.Equal(t, ips[10], "127.0.0.11:2375")
}

func TestGenerateWithMalformedInputAtRangeStart(t *testing.T) {
	malformedInput := "127.0.0.[x:11]:2375"
	ips := Generate(malformedInput)
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0], malformedInput)
}

func TestGenerateWithMalformedInputAtRangeEnd(t *testing.T) {
	malformedInput := "127.0.0.[1:x]:2375"
	ips := Generate(malformedInput)
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0], malformedInput)
}
