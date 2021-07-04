package libnetwork

import (
	"net"
	"runtime"
	"testing"

	"github.com/docker/docker/libnetwork/resolvconf"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestCleanupServiceDiscovery(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "test only works on linux")

	c, err := New()
	assert.NilError(t, err)
	defer c.Stop()

	cleanup := func(n Network) {
		if err := n.Delete(); err != nil {
			t.Error(err)
		}
	}
	n1, err := c.NewNetwork("bridge", "net1", "", nil)
	assert.NilError(t, err)
	defer cleanup(n1)

	n2, err := c.NewNetwork("bridge", "net2", "", nil)
	assert.NilError(t, err)
	defer cleanup(n2)

	n1.(*network).addSvcRecords("N1ep1", "service_test", "serviceID1", net.ParseIP("192.168.0.1"), net.IP{}, true, "test")
	n1.(*network).addSvcRecords("N2ep2", "service_test", "serviceID2", net.ParseIP("192.168.0.2"), net.IP{}, true, "test")

	n2.(*network).addSvcRecords("N2ep1", "service_test", "serviceID1", net.ParseIP("192.168.1.1"), net.IP{}, true, "test")
	n2.(*network).addSvcRecords("N2ep2", "service_test", "serviceID2", net.ParseIP("192.168.1.2"), net.IP{}, true, "test")

	if len(c.(*controller).svcRecords) != 2 {
		t.Fatalf("Service record not added correctly:%v", c.(*controller).svcRecords)
	}

	// cleanup net1
	c.(*controller).cleanupServiceDiscovery(n1.ID())

	if len(c.(*controller).svcRecords) != 1 {
		t.Fatalf("Service record not cleaned correctly:%v", c.(*controller).svcRecords)
	}

	c.(*controller).cleanupServiceDiscovery("")

	if len(c.(*controller).svcRecords) != 0 {
		t.Fatalf("Service record not cleaned correctly:%v", c.(*controller).svcRecords)
	}
}

func TestDNSOptions(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "test only works on linux")

	c, err := New()
	assert.NilError(t, err)

	sb, err := c.(*controller).NewSandbox("cnt1", nil)
	assert.NilError(t, err)

	cleanup := func(s Sandbox) {
		if err := s.Delete(); err != nil {
			t.Error(err)
		}
	}

	defer cleanup(sb)
	sb.(*sandbox).startResolver(false)

	err = sb.(*sandbox).setupDNS()
	assert.NilError(t, err)
	err = sb.(*sandbox).rebuildDNS()
	assert.NilError(t, err)
	currRC, err := resolvconf.GetSpecific(sb.(*sandbox).config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList := resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:0", dnsOptionsList[0]))

	sb.(*sandbox).config.dnsOptionsList = []string{"ndots:5"}
	err = sb.(*sandbox).setupDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb.(*sandbox).config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:5", dnsOptionsList[0]))

	err = sb.(*sandbox).rebuildDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb.(*sandbox).config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:5", dnsOptionsList[0]))

	sb2, err := c.(*controller).NewSandbox("cnt2", nil)
	assert.NilError(t, err)
	defer cleanup(sb2)
	sb2.(*sandbox).startResolver(false)

	sb2.(*sandbox).config.dnsOptionsList = []string{"ndots:0"}
	err = sb2.(*sandbox).setupDNS()
	assert.NilError(t, err)
	err = sb2.(*sandbox).rebuildDNS()
	assert.NilError(t, err)
	currRC, err = resolvconf.GetSpecific(sb2.(*sandbox).config.resolvConfPath)
	assert.NilError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Check(t, is.Len(dnsOptionsList, 1))
	assert.Check(t, is.Equal("ndots:0", dnsOptionsList[0]))

	sb2.(*sandbox).config.dnsOptionsList = []string{"ndots:foobar"}
	err = sb2.(*sandbox).setupDNS()
	assert.NilError(t, err)
	err = sb2.(*sandbox).rebuildDNS()
	assert.Error(t, err, "invalid number for ndots option: foobar")

	sb2.(*sandbox).config.dnsOptionsList = []string{"ndots:-1"}
	err = sb2.(*sandbox).setupDNS()
	assert.NilError(t, err)
	err = sb2.(*sandbox).rebuildDNS()
	assert.Error(t, err, "invalid number for ndots option: -1")
}
