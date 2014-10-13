package libvirt

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func buildTestInterface(mac string) (VirInterface, VirConnection) {
	conn := buildTestConnection()
	xml := `<interface type='ethernet' name='ethTest0'><mac address='` + mac + `'/></interface>`
	iface, err := conn.InterfaceDefineXML(xml, 0)
	if err != nil {
		panic(err)
	}
	return iface, conn
}

func generateRandomMac() string {
	macBuf := make([]byte, 3)
	if _, err := rand.Read(macBuf); err != nil {
		panic(err)
	}
	return fmt.Sprintf("aa:bb:cc:%02x:%02x:%02x", macBuf[0], macBuf[1], macBuf[2])
}

func TestCreateDestroyInterface(t *testing.T) {
	iface, conn := buildTestInterface(generateRandomMac())
	defer iface.Free()
	defer conn.CloseConnection()
	if err := iface.Create(0); err != nil {
		t.Error(err)
		return
	}
	if err := iface.Destroy(0); err != nil {
		t.Error(err)
	}
}

func TestUndefineInterface(t *testing.T) {
	iface, conn := buildTestInterface(generateRandomMac())
	defer iface.Free()
	defer conn.CloseConnection()
	name, err := iface.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if err := iface.Undefine(); err != nil {
		t.Error(err)
		return
	}
	if _, err := conn.LookupInterfaceByName(name); err == nil {
		t.Fatal("Shouldn't have been able to find interface")
	}
}

func TestGetInterfaceName(t *testing.T) {
	iface, conn := buildTestInterface(generateRandomMac())
	defer iface.Free()
	defer conn.CloseConnection()
	if _, err := iface.GetName(); err != nil {
		t.Fatal(err)
	}
}

func TestInterfaceIsActive(t *testing.T) {
	iface, conn := buildTestInterface(generateRandomMac())
	defer iface.Free()
	defer conn.CloseConnection()
	if err := iface.Create(0); err != nil {
		t.Log(err)
		return
	}
	active, err := iface.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if !active {
		t.Fatal("Interface should be active")
	}
	if err := iface.Destroy(0); err != nil {
		t.Error(err)
		return
	}
	active, err = iface.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if active {
		t.Fatal("Interface should be inactive")
	}
}

func TestGetMACString(t *testing.T) {
	origMac := generateRandomMac()
	iface, conn := buildTestInterface(origMac)
	defer iface.Free()
	defer conn.CloseConnection()
	mac, err := iface.GetMACString()
	if err != nil {
		t.Error(err)
		return
	}
	if mac != origMac {
		t.Fatalf("expected MAC: %s , got: %s", origMac, mac)
	}
}

func TestGetInterfaceXMLDesc(t *testing.T) {
	iface, conn := buildTestInterface(generateRandomMac())
	defer conn.CloseConnection()
	defer iface.Free()
	if _, err := iface.GetXMLDesc(0); err != nil {
		t.Error(err)
	}
}

func TestInterfaceFree(t *testing.T) {
	iface, conn := buildTestInterface(generateRandomMac())
	defer conn.CloseConnection()
	if err := iface.Free(); err != nil {
		t.Error(err)
		return
	}
}
