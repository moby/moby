package libvirt

import (
	"testing"
	"time"
)

func buildTestQEMUConnection() VirConnection {
	conn, err := NewVirConnection("qemu:///system")
	if err != nil {
		panic(err)
	}
	return conn
}

func buildTestConnection() VirConnection {
	conn, err := NewVirConnection("test:///default")
	if err != nil {
		panic(err)
	}
	return conn
}

func TestConnection(t *testing.T) {
	conn, err := NewVirConnection("test:///default")
	if err != nil {
		t.Error(err)
		return
	}
	_, err = conn.CloseConnection()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestConnectionReadOnly(t *testing.T) {
	conn, err := NewVirConnectionReadOnly("test:///default")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	_, err = conn.NetworkDefineXML(`<network>
    <name>` + time.Now().String() + `</name>
    <bridge name="testbr0"/>
    <forward/>
    <ip address="192.168.0.1" netmask="255.255.255.0">
    </ip>
    </network>`)
	if err == nil {
		t.Fatal("writing on a read only connection")
	}
}

func TestInvalidConnection(t *testing.T) {
	_, err := NewVirConnection("invalid_transport:///default")
	if err == nil {
		t.Error("Non-existent transport works")
	}
}

func TestGetType(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	tp, err := conn.GetType()
	if err != nil {
		t.Error(err)
		return
	}
	if tp != "Test" {
		t.Fatalf("type should have been test: %s", tp)
		return
	}
}

func TestIsAlive(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	alive, err := conn.IsAlive()
	if err != nil {
		t.Error(err)
		return
	}
	if !alive {
		t.Fatal("Connection should be alive")
		return
	}
}

func TestIsEncryptedAndSecure(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	secure, err := conn.IsSecure()
	if err != nil {
		t.Log(err)
		return
	}
	enc, err := conn.IsEncrypted()
	if err != nil {
		t.Error(err)
		return
	}
	if !secure {
		t.Fatal("Test driver should be secure")
		return
	}
	if enc {
		t.Fatal("Test driver should not be encrypted")
		return
	}
}

func TestCapabilities(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	capabilities, err := conn.GetCapabilities()
	if err != nil {
		t.Error(err)
		return
	}
	if capabilities == "" {
		t.Error("Capabilities was empty")
		return
	}
}

func TestGetNodeInfo(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	ni, err := conn.GetNodeInfo()
	if err != nil {
		t.Error(err)
		return
	}
	if ni.GetModel() != "i686" {
		t.Error("Expected i686 model in test transport")
		return
	}
}

func TestHostname(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	hostname, err := conn.GetHostname()
	if err != nil {
		t.Error(err)
		return
	}
	if hostname == "" {
		t.Error("Hostname was empty")
		return
	}
}

func TestLibVersion(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	version, err := conn.GetLibVersion()
	if err != nil {
		t.Error(err)
		return
	}
	if version == 0 {
		t.Error("Version was 0")
		return
	}
}

func TestListDefinedDomains(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	doms, err := conn.ListDefinedDomains()
	if err != nil {
		t.Error(err)
		return
	}
	if doms == nil {
		t.Fatal("ListDefinedDomains shouldn't be nil")
		return
	}
}

func TestListDomains(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	doms, err := conn.ListDomains()
	if err != nil {
		t.Error(err)
		return
	}
	if doms == nil {
		t.Fatal("ListDomains shouldn't be nil")
		return
	}
}

func TestListInterfaces(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.ListInterfaces()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestListNetworks(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.ListNetworks()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestListStoragePools(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.ListStoragePools()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestLookupDomainById(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	ids, err := conn.ListDomains()
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(ids)
	if len(ids) == 0 {
		t.Fatal("Length of ListDomains shouldn't be zero")
		return
	}
	dom, err := conn.LookupDomainById(ids[0])
	if err != nil {
		t.Error(err)
		return
	}
	defer dom.Free()
}

func TestLookupDomainByUUIDString(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	doms, err := conn.ListAllDomains(0)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(doms)
	if len(doms) == 0 {
		t.Fatal("Length of ListAllDomains shouldn't be empty")
		return
	}
	uuid, err := doms[0].GetUUIDString()
	if err != nil {
		t.Error(err)
		return
	}
	dom, err := conn.LookupByUUIDString(uuid)
	if err != nil {
		t.Error(err)
		return
	}
	defer dom.Free()
}

func TestLookupInvalidDomainById(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.LookupDomainById(12345)
	if err == nil {
		t.Error("Domain #12345 shouldn't exist in test transport")
		return
	}
}

func TestLookupDomainByName(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	dom, err := conn.LookupDomainByName("test")
	if err != nil {
		t.Error(err)
		return
	}
	defer dom.Free()
}

func TestLookupInvalidDomainByName(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.LookupDomainByName("non_existent_domain")
	if err == nil {
		t.Error("Could find non-existent domain by name")
		return
	}
}

func TestDomainCreateXML(t *testing.T) {
	conn := buildTestConnection()
	nodom := VirDomain{}
	defer conn.CloseConnection()
	// Test a minimally valid xml
	defName := time.Now().String()
	xml := `<domain type="test">
		<name>` + defName + `</name>
		<memory unit="KiB">8192</memory>
		<os>
			<type>hvm</type>
		</os>
	</domain>`
	dom, err := conn.DomainCreateXML(xml, VIR_DOMAIN_NONE)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		if dom != nodom {
			dom.Destroy()
			dom.Free()
		}
	}()
	name, err := dom.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if name != defName {
		t.Fatalf("Name was not '%s': %s", defName, name)
		return
	}

	// Destroy the domain: it should not be persistent
	if err := dom.Destroy(); err != nil {
		t.Error(err)
		return
	}
	dom = nodom

	testeddom, err := conn.LookupDomainByName(defName)
	if testeddom != nodom {
		t.Fatal("Created domain is persisting")
		return
	}
}

func TestDomainDefineXML(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	// Test a minimally valid xml
	defName := time.Now().String()
	xml := `<domain type="test">
		<name>` + defName + `</name>
		<memory unit="KiB">8192</memory>
		<os>
			<type>hvm</type>
		</os>
	</domain>`
	dom, err := conn.DomainDefineXML(xml)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()
	name, err := dom.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if name != defName {
		t.Fatalf("Name was not 'test': %s", name)
		return
	}
	// And an invalid one
	xml = `<domain type="test"></domain>`
	_, err = conn.DomainDefineXML(xml)
	if err == nil {
		t.Fatal("Should have had an error")
		return
	}
}

func TestListDefinedInterfaces(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.ListDefinedInterfaces()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestListDefinedNetworks(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.ListDefinedNetworks()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestListDefinedStoragePools(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.ListDefinedStoragePools()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfDefinedInterfaces(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfDefinedInterfaces(); err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfDefinedNetworks(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfDefinedNetworks(); err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfDefinedStoragePools(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfDefinedStoragePools(); err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfDomains(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfDomains(); err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfInterfaces(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfInterfaces(); err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfNetworks(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfNetworks(); err != nil {
		t.Error(err)
		return
	}
}

func TestNumOfNWFilters(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfNWFilters(); err == nil {
		t.Fatalf("NumOfNWFilters should fail due to no support on test driver")
		return
	}
}

func TestNumOfSecrets(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	if _, err := conn.NumOfSecrets(); err == nil {
		t.Fatalf("NumOfSecrets should fail due to no support on test driver")
		return
	}
}

func TestGetURI(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	uri, err := conn.GetURI()
	if err != nil {
		t.Error(err)
	}
	origUri := "test:///default"
	if uri != origUri {
		t.Fatalf("should be %s but got %s", origUri, uri)
	}
}

func TestGetMaxVcpus(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	_, err := conn.GetMaxVcpus("")
	if err != nil {
		t.Error(err)
	}
}

func TestInterfaceDefineXML(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	defName := "ethTest0"
	xml := `<interface type='ethernet' name='` + defName + `'><mac address='` + generateRandomMac() + `'/></interface>`
	iface, err := conn.InterfaceDefineXML(xml, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer iface.Undefine()
	name, err := iface.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if name != defName {
		t.Fatalf("Expected interface name: %s,got: %s", defName, name)
		return
	}
	// Invalid configuration
	xml = `<interface type="test"></interface>`
	_, err = conn.InterfaceDefineXML(xml, 0)
	if err == nil {
		t.Fatal("Should have had an error")
		return
	}
}

func TestLookupInterfaceByName(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	testEth := "eth1"
	iface, err := conn.LookupInterfaceByName(testEth)
	if err != nil {
		t.Error(err)
		return
	}
	var ifName string
	ifName, err = iface.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if ifName != testEth {
		t.Fatalf("expected interface name: %s ,got: %s", testEth, ifName)
	}
}

func TestLookupInterfaceByMACString(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	testMAC := "aa:bb:cc:dd:ee:ff"
	iface, err := conn.LookupInterfaceByMACString(testMAC)
	if err != nil {
		t.Error(err)
		return
	}
	var ifMAC string
	ifMAC, err = iface.GetMACString()
	if err != nil {
		t.Error(err)
		return
	}
	if ifMAC != testMAC {
		t.Fatalf("expected interface MAC: %s ,got: %s", testMAC, ifMAC)
	}
}

func TestStoragePoolDefineXML(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	defName := "default-pool-test-0"
	xml := `<pool type='dir'><name>default-pool-test-0</name><target>
            <path>/default-pool</path></target></pool>`
	pool, err := conn.StoragePoolDefineXML(xml, 0)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer pool.Free()
	defer pool.Undefine()
	name, err := pool.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if name != defName {
		t.Fatalf("Expected storage pool name: %s,got: %s", defName, name)
		return
	}
	// Invalid configuration
	xml = `<pool type='bad'></pool>`
	_, err = conn.StoragePoolDefineXML(xml, 0)
	if err == nil {
		t.Fatal("Should have had an error")
		return
	}
}

func TestLookupStoragePoolByName(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	testPool := "default-pool"
	pool, err := conn.LookupStoragePoolByName(testPool)
	if err != nil {
		t.Error(err)
		return
	}
	defer pool.Free()
	var poolName string
	poolName, err = pool.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if poolName != testPool {
		t.Fatalf("expected storage pool name: %s ,got: %s", testPool, poolName)
	}
}

func TestLookupStoragePoolByUUIDString(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	poolName := "default-pool"
	pool, err := conn.LookupStoragePoolByName(poolName)
	if err != nil {
		t.Error(err)
		return
	}
	defer pool.Free()
	var poolUUID string
	poolUUID, err = pool.GetUUIDString()
	if err != nil {
		t.Error(err)
		return
	}
	pool, err = conn.LookupStoragePoolByUUIDString(poolUUID)
	if err != nil {
		t.Error(err)
		return
	}
	name, err := pool.GetName()
	if err != nil {
		t.Error(err)
	}
	if name != poolName {
		t.Fatalf("fetching by UUID: expected storage pool name: %s ,got: %s", name, poolName)
	}
}

func TestLookupStorageVolByKey(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	defPoolPath := "default-pool"
	defVolName := time.Now().String()
	defVolKey := "/" + defPoolPath + "/" + defVolName
	vol, err := pool.StorageVolCreateXML(testStorageVolXML(defVolName, defPoolPath), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()
	vol, err = conn.LookupStorageVolByKey(defVolKey)
	if err != nil {
		t.Error(err)
		return
	}
	key, err := vol.GetKey()
	if err != nil {
		t.Error(err)
		return
	}
	if key != defVolKey {
		t.Fatalf("expected storage volume key: %s ,got: %s", defVolKey, key)
	}
}

func TestLookupStorageVolByPath(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	defPoolPath := "default-pool"
	defVolName := time.Now().String()
	defVolPath := "/" + defPoolPath + "/" + defVolName
	vol, err := pool.StorageVolCreateXML(testStorageVolXML(defVolName, defPoolPath), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()
	vol, err = conn.LookupStorageVolByPath(defVolPath)
	if err != nil {
		t.Error(err)
		return
	}
	path, err := vol.GetPath()
	if err != nil {
		t.Error(err)
		return
	}
	if path != defVolPath {
		t.Fatalf("expected storage volume path: %s ,got: %s", defVolPath, path)
	}
}

func TestListAllDomains(t *testing.T) {
	conn := buildTestConnection()
	defer conn.CloseConnection()
	doms, err := conn.ListAllDomains(VIR_CONNECT_LIST_DOMAINS_PERSISTENT)
	if err != nil {
		t.Error(err)
		return
	}
	if len(doms) == 0 {
		t.Fatal("length of []VirDomain shouldn't be 0")
	}
	testDomName := "test"
	found := false
	for _, dom := range doms {
		name, _ := dom.GetName()
		if name == testDomName {
			found = true
		}
		// not mandatory for the tests but lets make it in a proper way
		dom.Free()
	}
	if found == false {
		t.Fatalf("domain %s not found", testDomName)
	}
}

func TestListAllNetworks(t *testing.T) {
	testNetwork := time.Now().String()
	net, conn := buildTestNetwork(testNetwork)
	defer func() {
		// actually,no nicessaty to destroy as the network is being removed as soon as
		// the test connection is closed
		net.Destroy()
		net.Free()
		conn.CloseConnection()
	}()
	nets, err := conn.ListAllNetworks(VIR_CONNECT_LIST_NETWORKS_INACTIVE)
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) == 0 {
		t.Fatal("length of []VirNetwork shouldn't be 0")
	}
	found := false
	for _, n := range nets {
		name, _ := n.GetName()
		if name == testNetwork {
			found = true
		}
		n.Free()
	}
	if found == false {
		t.Fatalf("network %s not found", testNetwork)
	}
}

func TestListAllStoragePools(t *testing.T) {
	testStoragePool := "default-pool-test-1"
	pool, conn := buildTestStoragePool(testStoragePool)
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	pools, err := conn.ListAllStoragePools(VIR_STORAGE_POOL_INACTIVE)
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) == 0 {
		t.Fatal("length of []VirStoragePool shouldn't be 0")
	}
	found := false
	for _, p := range pools {
		name, _ := p.GetName()
		if name == testStoragePool {
			found = true
		}
		p.Free()
	}
	if found == false {
		t.Fatalf("storage pool %s not found", testStoragePool)
	}
}
