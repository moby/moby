//go:build !windows

package libnetwork

import (
	"net"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/ipamutils"
	"gotest.tools/v3/assert"
)

func TestCleanupServiceDiscovery(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	c, err := New(config.OptionDataDir(t.TempDir()),
		config.OptionDefaultAddressPoolConfig(ipamutils.GetLocalScopeDefaultNetworks()))
	assert.NilError(t, err)
	defer c.Stop()

	cleanup := func(n *Network) {
		if err := n.Delete(); err != nil {
			t.Error(err)
		}
	}
	n1, err := c.NewNetwork("bridge", "net1", "", NetworkOptionEnableIPv4(true))
	assert.NilError(t, err)
	defer cleanup(n1)

	n2, err := c.NewNetwork("bridge", "net2", "", NetworkOptionEnableIPv4(true))
	assert.NilError(t, err)
	defer cleanup(n2)

	n1.addSvcRecords("N1ep1", "service_test", "serviceID1", net.ParseIP("192.168.0.1"), net.IP{}, true, "test")
	n1.addSvcRecords("N2ep2", "service_test", "serviceID2", net.ParseIP("192.168.0.2"), net.IP{}, true, "test")

	n2.addSvcRecords("N2ep1", "service_test", "serviceID1", net.ParseIP("192.168.1.1"), net.IP{}, true, "test")
	n2.addSvcRecords("N2ep2", "service_test", "serviceID2", net.ParseIP("192.168.1.2"), net.IP{}, true, "test")

	if len(c.svcRecords) != 2 {
		t.Fatalf("Service record not added correctly:%v", c.svcRecords)
	}

	// cleanup net1
	c.cleanupServiceDiscovery(n1.ID())

	if len(c.svcRecords) != 1 {
		t.Fatalf("Service record not cleaned correctly:%v", c.svcRecords)
	}

	c.cleanupServiceDiscovery("")

	if len(c.svcRecords) != 0 {
		t.Fatalf("Service record not cleaned correctly:%v", c.svcRecords)
	}
}
