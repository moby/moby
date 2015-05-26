package nodes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialise(t *testing.T) {
	discovery := &Discovery{}
	discovery.Initialize("1.1.1.1:1111,2.2.2.2:2222", 0)
	assert.Equal(t, len(discovery.entries), 2)
	assert.Equal(t, discovery.entries[0].String(), "1.1.1.1:1111")
	assert.Equal(t, discovery.entries[1].String(), "2.2.2.2:2222")
}

func TestInitialiseWithPattern(t *testing.T) {
	discovery := &Discovery{}
	discovery.Initialize("1.1.1.[1:2]:1111,2.2.2.[2:4]:2222", 0)
	assert.Equal(t, len(discovery.entries), 5)
	assert.Equal(t, discovery.entries[0].String(), "1.1.1.1:1111")
	assert.Equal(t, discovery.entries[1].String(), "1.1.1.2:1111")
	assert.Equal(t, discovery.entries[2].String(), "2.2.2.2:2222")
	assert.Equal(t, discovery.entries[3].String(), "2.2.2.3:2222")
	assert.Equal(t, discovery.entries[4].String(), "2.2.2.4:2222")
}

func TestRegister(t *testing.T) {
	discovery := &Discovery{}
	assert.Error(t, discovery.Register("0.0.0.0"))
}
