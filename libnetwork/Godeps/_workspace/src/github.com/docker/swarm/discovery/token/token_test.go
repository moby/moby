package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	discovery := &Discovery{}
	err := discovery.Initialize("token", 0)
	assert.NoError(t, err)
	assert.Equal(t, discovery.token, "token")
	assert.Equal(t, discovery.url, DiscoveryURL)

	err = discovery.Initialize("custom/path/token", 0)
	assert.NoError(t, err)
	assert.Equal(t, discovery.token, "token")
	assert.Equal(t, discovery.url, "https://custom/path")

	err = discovery.Initialize("", 0)
	assert.Error(t, err)
}

func TestRegister(t *testing.T) {
	discovery := &Discovery{token: "TEST_TOKEN", url: DiscoveryURL}
	expected := "127.0.0.1:2675"
	assert.NoError(t, discovery.Register(expected))

	addrs, err := discovery.Fetch()
	assert.NoError(t, err)
	assert.Equal(t, len(addrs), 1)
	assert.Equal(t, addrs[0].String(), expected)

	assert.NoError(t, discovery.Register(expected))
}
