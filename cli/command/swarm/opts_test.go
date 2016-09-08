package swarm

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestNodeAddrOptionSetHostAndPort(t *testing.T) {
	opt := NewNodeAddrOption("old:123")
	addr := "newhost:5555"
	assert.NilError(t, opt.Set(addr))
	assert.Equal(t, opt.Value(), addr)
}

func TestNodeAddrOptionSetHostOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("newhost"))
	assert.Equal(t, opt.Value(), "newhost:2377")
}

func TestNodeAddrOptionSetHostOnlyIPv6(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("::1"))
	assert.Equal(t, opt.Value(), "[::1]:2377")
}

func TestNodeAddrOptionSetPortOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set(":4545"))
	assert.Equal(t, opt.Value(), "0.0.0.0:4545")
}

func TestNodeAddrOptionSetInvalidFormat(t *testing.T) {
	opt := NewListenAddrOption()
	assert.Error(t, opt.Set("http://localhost:4545"), "Invalid")
}
