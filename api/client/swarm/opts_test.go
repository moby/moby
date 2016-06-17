package swarm

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestNodeAddrOptionSetHostAndPort(t *testing.T) {
	opt := NewNodeAddrOption("old", 123)
	addr := "newhost:5555"
	assert.NilError(t, opt.Set(addr))
	assert.Equal(t, opt.addr, "newhost")
	assert.Equal(t, opt.port, uint16(5555))
	assert.Equal(t, opt.Value(), addr)
}

func TestNodeAddrOptionSetHostOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set("newhost"))
	assert.Equal(t, opt.addr, "newhost")
	assert.Equal(t, opt.port, defaultListenPort)
}

func TestNodeAddrOptionSetPortOnly(t *testing.T) {
	opt := NewListenAddrOption()
	assert.NilError(t, opt.Set(":4545"))
	assert.Equal(t, opt.addr, defaultListenAddr)
	assert.Equal(t, opt.port, uint16(4545))
}

func TestNodeAddrOptionSetInvalidFormat(t *testing.T) {
	opt := NewListenAddrOption()
	assert.Error(t, opt.Set("http://localhost:4545"), "Invalid url")
}
