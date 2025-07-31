//go:build !windows

package libnetwork

import (
	"context"
	"net"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/daemon/libnetwork/ipamutils"
	"github.com/moby/moby/v2/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
)

func TestCleanupServiceDiscovery(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	c, err := New(context.Background(), config.OptionDataDir(t.TempDir()),
		config.OptionDefaultAddressPoolConfig(ipamutils.GetLocalScopeDefaultNetworks()))
	assert.NilError(t, err)
	defer c.Stop()

	cleanup := func(n *Network) {
		if err := n.Delete(); err != nil {
			t.Error(err)
		}
	}
	n1, err := c.NewNetwork(context.Background(), "bridge", "net1", "", NetworkOptionEnableIPv4(true))
	assert.NilError(t, err)
	defer cleanup(n1)

	n2, err := c.NewNetwork(context.Background(), "bridge", "net2", "", NetworkOptionEnableIPv4(true))
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
