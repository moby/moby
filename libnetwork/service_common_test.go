package libnetwork

import (
	"net"
	"testing"

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
