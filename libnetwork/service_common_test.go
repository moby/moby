package libnetwork

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/resolvconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupServiceDiscovery(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Stop()

	n1, err := c.NewNetwork("bridge", "net1", "", nil)
	require.NoError(t, err)
	defer n1.Delete()

	n2, err := c.NewNetwork("bridge", "net2", "", nil)
	require.NoError(t, err)
	defer n2.Delete()

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
	c, err := New()
	require.NoError(t, err)

	sb, err := c.(*controller).NewSandbox("cnt1", nil)
	require.NoError(t, err)
	defer sb.Delete()
	sb.(*sandbox).startResolver(false)

	err = sb.(*sandbox).setupDNS()
	require.NoError(t, err)
	err = sb.(*sandbox).rebuildDNS()
	require.NoError(t, err)
	currRC, err := resolvconf.GetSpecific(sb.(*sandbox).config.resolvConfPath)
	require.NoError(t, err)
	dnsOptionsList := resolvconf.GetOptions(currRC.Content)
	assert.Equal(t, 1, len(dnsOptionsList), "There should be only 1 option instead:", dnsOptionsList)
	assert.Equal(t, "ndots:0", dnsOptionsList[0], "The option must be ndots:0 instead:", dnsOptionsList[0])

	sb.(*sandbox).config.dnsOptionsList = []string{"ndots:5"}
	err = sb.(*sandbox).setupDNS()
	require.NoError(t, err)
	currRC, err = resolvconf.GetSpecific(sb.(*sandbox).config.resolvConfPath)
	require.NoError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Equal(t, 1, len(dnsOptionsList), "There should be only 1 option instead:", dnsOptionsList)
	assert.Equal(t, "ndots:5", dnsOptionsList[0], "The option must be ndots:5 instead:", dnsOptionsList[0])

	err = sb.(*sandbox).rebuildDNS()
	require.NoError(t, err)
	currRC, err = resolvconf.GetSpecific(sb.(*sandbox).config.resolvConfPath)
	require.NoError(t, err)
	dnsOptionsList = resolvconf.GetOptions(currRC.Content)
	assert.Equal(t, 1, len(dnsOptionsList), "There should be only 1 option instead:", dnsOptionsList)
	assert.Equal(t, "ndots:5", dnsOptionsList[0], "The option must be ndots:5 instead:", dnsOptionsList[0])
}
