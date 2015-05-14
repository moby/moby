package bridge

import (
	"os"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
)

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func TestPortMappingConfig(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	binding1 := netutils.PortBinding{Proto: netutils.UDP, Port: uint16(400), HostPort: uint16(54000)}
	binding2 := netutils.PortBinding{Proto: netutils.TCP, Port: uint16(500), HostPort: uint16(65000)}
	portBindings := []netutils.PortBinding{binding1, binding2}

	epOptions := make(map[string]interface{})
	epOptions[netlabel.PortMap] = portBindings

	netConfig := &NetworkConfiguration{
		BridgeName:     DefaultBridgeName,
		EnableIPTables: true,
	}
	netOptions := make(map[string]interface{})
	netOptions[netlabel.GenericData] = netConfig

	err := d.CreateNetwork("dummy", netOptions)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep1", te, epOptions)
	if err != nil {
		t.Fatalf("Failed to create the endpoint: %s", err.Error())
	}

	dd := d.(*driver)
	ep, _ := dd.network.endpoints["ep1"]
	if len(ep.portMapping) != 2 {
		t.Fatalf("Failed to store the port bindings into the sandbox info. Found: %v", ep.portMapping)
	}
	if ep.portMapping[0].Proto != binding1.Proto || ep.portMapping[0].Port != binding1.Port ||
		ep.portMapping[1].Proto != binding2.Proto || ep.portMapping[1].Port != binding2.Port {
		t.Fatalf("bridgeEndpoint has incorrect port mapping values")
	}
	if ep.portMapping[0].HostIP == nil || ep.portMapping[0].HostPort == 0 ||
		ep.portMapping[1].HostIP == nil || ep.portMapping[1].HostPort == 0 {
		t.Fatalf("operational port mapping data not found on bridgeEndpoint")
	}

	err = releasePorts(ep)
	if err != nil {
		t.Fatalf("Failed to release mapped ports: %v", err)
	}
}
