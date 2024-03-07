//go:build !windows

package libnetwork

import (
	"runtime"
	"testing"

	"github.com/docker/docker/libnetwork/resolvconf"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDNSOptions(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "test only works on linux")

	c, err := New(OptionBoltdbWithRandomDBFile(t))
	assert.NilError(t, err)

	sb, err := c.NewSandbox("cnt1", nil)
	assert.NilError(t, err)

	cleanup := func(s *Sandbox) {
		if err := s.Delete(); err != nil {
			t.Error(err)
		}
	}

	defer cleanup(sb)
	sb.startResolver(false)

	err = sb.setupDNS()
	assert.NilError(t, err)
	err = sb.rebuildDNS()
	assert.NilError(t, err)
	currRC, err := resolvconf.GetSpecific(sb.config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList := resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:0", dnsOptionsList[0]))

	sb.config.dnsOptionsList = []string{"ndots:5"}
	err = sb.setupDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb.config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:5", dnsOptionsList[0]))

	err = sb.rebuildDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb.config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:5", dnsOptionsList[0]))

	sb2, err := c.NewSandbox("cnt2", nil)
	assert.NilError(t, err)
	defer cleanup(sb2)
	sb2.startResolver(false)

	sb2.config.dnsOptionsList = []string{"ndots:0"}
	err = sb2.setupDNS()
	assert.NilError(t, err)
	err = sb2.rebuildDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb2.config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:0", dnsOptionsList[0]))

	sb2.config.dnsOptionsList = []string{"ndots:foobar"}
	err = sb2.setupDNS()
	assert.NilError(t, err)
	err = sb2.rebuildDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb2.config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.DeepEqual([]string{"ndots:0"}, dnsOptionsList))

	sb2.config.dnsOptionsList = []string{"ndots:-1"}
	err = sb2.setupDNS()
	assert.NilError(t, err)
	err = sb2.rebuildDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb2.config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.DeepEqual([]string{"ndots:0"}, dnsOptionsList))
}
