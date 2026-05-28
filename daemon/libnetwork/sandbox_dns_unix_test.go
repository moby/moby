//go:build !windows

package libnetwork

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/resolvconf"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func getResolvConfOptions(t *testing.T, rcPath string) []string {
	t.Helper()
	resolv, err := os.ReadFile(rcPath)
	assert.NilError(t, err)
	rc, err := resolvconf.Parse(bytes.NewBuffer(resolv), "")
	assert.NilError(t, err)
	return rc.Options()
}

func TestDNSOptions(t *testing.T) {
	c, err := New(context.Background(), config.OptionDataDir(t.TempDir()))
	assert.NilError(t, err)

	sb, err := c.NewSandbox(context.Background(), "cnt1", nil)
	assert.NilError(t, err)

	cleanup := func(s *Sandbox) {
		if err := s.Delete(context.Background()); err != nil {
			t.Error(err)
		}
	}

	defer cleanup(sb)
	sb.startResolver(false)

	err = sb.setupDNS()
	assert.NilError(t, err)
	err = sb.rebuildDNS()
	assert.NilError(t, err)
	dnsOptionsList := getResolvConfOptions(t, sb.config.resolvConfPath)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:0", dnsOptionsList[0]))

	sb.config.dnsOptionsList = []string{"ndots:5"}
	err = sb.setupDNS()
	assert.NilError(t, err)
	dnsOptionsList = getResolvConfOptions(t, sb.config.resolvConfPath)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:5", dnsOptionsList[0]))

	err = sb.rebuildDNS()
	assert.NilError(t, err)
	dnsOptionsList = getResolvConfOptions(t, sb.config.resolvConfPath)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:5", dnsOptionsList[0]))

	sb2, err := c.NewSandbox(context.Background(), "cnt2", nil)
	assert.NilError(t, err)
	defer cleanup(sb2)
	sb2.startResolver(false)

	sb2.config.dnsOptionsList = []string{"ndots:0"}
	err = sb2.setupDNS()
	assert.NilError(t, err)
	err = sb2.rebuildDNS()
	assert.NilError(t, err)
	dnsOptionsList = getResolvConfOptions(t, sb2.config.resolvConfPath)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:0", dnsOptionsList[0]))

	sb2.config.dnsOptionsList = []string{"ndots:foobar"}
	err = sb2.setupDNS()
	assert.NilError(t, err)
	err = sb2.rebuildDNS()
	assert.NilError(t, err)
	dnsOptionsList = getResolvConfOptions(t, sb2.config.resolvConfPath)
	assert.Check(t, is.DeepEqual([]string{"ndots:0"}, dnsOptionsList))

	sb2.config.dnsOptionsList = []string{"ndots:-1"}
	err = sb2.setupDNS()
	assert.NilError(t, err)
	err = sb2.rebuildDNS()
	assert.NilError(t, err)
	dnsOptionsList = getResolvConfOptions(t, sb2.config.resolvConfPath)
	assert.Check(t, is.DeepEqual([]string{"ndots:0"}, dnsOptionsList))
}
