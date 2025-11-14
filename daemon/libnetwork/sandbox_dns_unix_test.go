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

func getResolvConf(t *testing.T, rcPath string) resolvconf.ResolvConf {
	t.Helper()
	resolv, err := os.ReadFile(rcPath)
	assert.NilError(t, err)
	rc, err := resolvconf.Parse(bytes.NewBuffer(resolv), "")
	assert.NilError(t, err)
	return rc
}

func getResolvConfOptions(t *testing.T, rcPath string) []string {
	rc := getResolvConf(t, rcPath)
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

func TestNonHostNetDNSRestart(t *testing.T) {
	c, err := New(context.Background(), config.OptionDataDir(t.TempDir()))
	assert.NilError(t, err)

	// Step 1: Create initial sandbox (simulating first container start)
	sb, err := c.NewSandbox(context.Background(), "cnt1")
	assert.NilError(t, err)

	sb.startResolver(false)

	err = sb.setupDNS()
	assert.NilError(t, err)
	err = sb.rebuildDNS()
	assert.NilError(t, err)

	// Step 2: Simulate cri-dockerd modifying the resolv.conf for a Kubernetes pause container.
	// This mimics the behavior where external tools (like cri-dockerd) customize DNS
	// settings for K8s pods, which should be preserved during container restart/unpause.
	resolvConfPath := sb.config.resolvConfPath
	modifiedContent := []byte(`nameserver 10.96.0.10
search default.svc.cluster.local. svc.cluster.local. cluster.local.
options ndots:5
`)
	err = os.WriteFile(resolvConfPath, modifiedContent, 0644)
	assert.NilError(t, err)

	// Step 3: Delete the sandbox (simulating container stop)
	err = sb.Delete(context.Background())
	assert.NilError(t, err)

	// Step 4: Create a new sandbox with OptionRestartOperate (simulating container restart)
	sbRestart, err := c.NewSandbox(context.Background(), "cnt1",
		OptionCreateByRestart(),
		OptionResolvConfPath(resolvConfPath),
	)
	assert.NilError(t, err)
	defer func() {
		if err := sbRestart.Delete(context.Background()); err != nil {
			t.Error(err)
		}
	}()

	sbRestart.startResolver(false)

	// Step 5: Call setupDNS on restart - should preserve external modifications
	err = sbRestart.setupDNS()
	assert.NilError(t, err)

	// Verify that the DNS settings modified by cri-dockerd are preserved
	rc := getResolvConf(t, sbRestart.config.resolvConfPath)
	assert.Check(t, is.Len(rc.Options(), 1))
	assert.Check(t, is.Equal("10.96.0.10", rc.NameServers()[0].String()))
	assert.Check(t, is.DeepEqual([]string{"default.svc.cluster.local.", "svc.cluster.local.", "cluster.local."}, rc.Search()))
	assert.Check(t, is.Equal("ndots:5", rc.Options()[0]))

	err = sbRestart.rebuildDNS()
	assert.NilError(t, err)

	rc = getResolvConf(t, sbRestart.config.resolvConfPath)
	assert.Check(t, is.Len(rc.Options(), 1))
	assert.Check(t, is.Equal("10.96.0.10", rc.NameServers()[0].String()))
	assert.Check(t, is.DeepEqual([]string{"default.svc.cluster.local.", "svc.cluster.local.", "cluster.local."}, rc.Search()))
	assert.Check(t, is.Equal("ndots:5", rc.Options()[0]))

}
