package libvirt

import (
	"testing"
	"time"
)

func buildTestNetwork(netName string) (VirNetwork, VirConnection) {
	conn := buildTestConnection()
	var name string
	if netName == "" {
		name = time.Now().String()
	} else {
		name = netName
	}
	net, err := conn.NetworkDefineXML(`<network>
    <name>` + name + `</name>
    <bridge name="testbr0"/>
    <forward/>
    <ip address="192.168.0.1" netmask="255.255.255.0">
    </ip>
    </network>`)
	if err != nil {
		panic(err)
	}
	return net, conn
}

func TestGetNetworkName(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	if _, err := net.GetName(); err != nil {
		t.Fatal(err)
		return
	}
}

func TestGetNetworkUUID(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	_, err := net.GetUUID()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestGetNetworkUUIDString(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	_, err := net.GetUUIDString()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestGetNetworkXMLDesc(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	if _, err := net.GetXMLDesc(0); err != nil {
		t.Error(err)
		return
	}
}

func TestCreateDestroyNetwork(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	if err := net.Create(); err != nil {
		t.Error(err)
		return
	}

	if err := net.Destroy(); err != nil {
		t.Error(err)
		return
	}
}

func TestNetworkAutostart(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	as, err := net.GetAutostart()
	if err != nil {
		t.Error(err)
		return
	}
	if as {
		t.Fatal("autostart should be false")
		return
	}
	if err := net.SetAutostart(true); err != nil {
		t.Error(err)
		return
	}
	as, err = net.GetAutostart()
	if err != nil {
		t.Error(err)
		return
	}
	if !as {
		t.Fatal("autostart should be true")
		return
	}
}

func TestNetworkIsActive(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	if err := net.Create(); err != nil {
		t.Log(err)
		return
	}
	active, err := net.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if !active {
		t.Fatal("Network should be active")
		return
	}
	if err := net.Destroy(); err != nil {
		t.Error(err)
		return
	}
	active, err = net.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if active {
		t.Fatal("Network should be inactive")
		return
	}
}

func TestNetworkGetBridgeName(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer func() {
		net.Free()
		conn.CloseConnection()
	}()
	if err := net.Create(); err != nil {
		t.Error(err)
		return
	}
	brName := "testbr0"
	br, err := net.GetBridgeName()
	if err != nil {
		t.Errorf("got %s but expected %s", br, brName)
	}
}

func TestNetworkFree(t *testing.T) {
	net, conn := buildTestNetwork("")
	defer conn.CloseConnection()
	if err := net.Free(); err != nil {
		t.Error(err)
		return
	}
}
