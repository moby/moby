// +build integration

package libvirt

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func defineTestLxcDomain(conn VirConnection, title string) (VirDomain, error) {
	if title == "" {
		title = time.Now().String()
	}
	xml := `<domain type='lxc'>
	  <name>` + title + `</name>
	  <title>` + title + `</title>
	  <memory>102400</memory>
	  <os>
	    <type>exe</type>
	    <init>/bin/sh</init>
	  </os>
	  <devices>
	    <console type='pty'/>
	  </devices>
	</domain>`
	dom, err := conn.DomainDefineXML(xml)
	return dom, err
}

// Integration tests are run against LXC using Libvirt 1.2.x
// on Debian Wheezy (libvirt from wheezy-backports)
//
// To run,
// 		go test -tags integration

func TestIntegrationGetMetadata(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	title := time.Now().String()
	dom, err := defineTestLxcDomain(conn, title)
	if err != nil {
		t.Error(err)
		return
	}
	defer dom.Free()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	v, err := dom.GetMetadata(VIR_DOMAIN_METADATA_TITLE, "", 0)
	dom.Destroy()
	if err != nil {
		t.Error(err)
		return
	}
	if v != title {
		t.Fatal("title didnt match: expected %s, got %s", title, v)
		return
	}
	if err := dom.Undefine(); err != nil {
		t.Error(err)
		return
	}
}

func TestIntegrationSetMetadata(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	dom, err := defineTestLxcDomain(conn, "")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()
	const domTitle = "newtitle"
	if err := dom.SetMetadata(VIR_DOMAIN_METADATA_TITLE, domTitle, "", "", 0); err != nil {
		t.Error(err)
		return
	}
	v, err := dom.GetMetadata(VIR_DOMAIN_METADATA_TITLE, "", 0)
	if err != nil {
		t.Error(err)
		return
	}
	if v != domTitle {
		t.Fatalf("VIR_DOMAIN_METADATA_TITLE should have been %s, not %s", domTitle, v)
		return
	}
}

func TestIntegrationGetSysinfo(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	info, err := conn.GetSysinfo(0)
	if err != nil {
		t.Error(err)
		return
	}
	if strings.Index(info, "<sysinfo") != 0 {
		t.Fatalf("Sysinfo not valid: %s", info)
		return
	}
}

func testNWFilterXML(name, chain string) string {
	defName := name
	if defName == "" {
		defName = time.Now().String()
	}
	return `<filter name='` + defName + `' chain='` + chain + `'>
            <rule action='drop' direction='out' priority='500'>
            <ip match='no' srcipaddr='$IP'/>
            </rule>
			</filter>`
}

func TestIntergrationDefineUndefineNWFilterXML(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML("", "ipv4"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := filter.Undefine(); err != nil {
			t.Fatal(err)
		}
		filter.Free()
	}()
	_, err = conn.NWFilterDefineXML(testNWFilterXML("", "bad"))
	if err == nil {
		t.Fatal("Should have had an error")
	}
}

func TestIntegrationNWFilterGetName(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML("", "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()
	if _, err := filter.GetName(); err != nil {
		t.Error(err)
	}
}

func TestIntegrationNWFilterGetUUID(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML("", "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()
	if _, err := filter.GetUUID(); err != nil {
		t.Error(err)
	}
}

func TestIntegrationNWFilterGetUUIDString(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML("", "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()
	if _, err := filter.GetUUIDString(); err != nil {
		t.Error(err)
	}
}

func TestIntegrationNWFilterGetXMLDesc(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML("", "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()
	if _, err := filter.GetXMLDesc(0); err != nil {
		t.Error(err)
	}
}

func TestIntegrationLookupNWFilterByName(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	origName := time.Now().String()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML(origName, "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()
	filter, err = conn.LookupNWFilterByName(origName)
	if err != nil {
		t.Error(err)
		return
	}
	var newName string
	newName, err = filter.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if newName != origName {
		t.Fatalf("expected filter name: %s ,got: %s", origName, newName)
	}
}

func TestIntegrationLookupNWFilterByUUIDString(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	origName := time.Now().String()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML(origName, "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()
	filter, err = conn.LookupNWFilterByName(origName)
	if err != nil {
		t.Error(err)
		return
	}
	var filterUUID string
	filterUUID, err = filter.GetUUIDString()
	if err != nil {
		t.Error(err)
		return
	}
	filter, err = conn.LookupNWFilterByUUIDString(filterUUID)
	if err != nil {
		t.Error(err)
		return
	}
	name, err := filter.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if name != origName {
		t.Fatalf("fetching by UUID: expected filter name: %s ,got: %s", name, origName)
	}
}

func TestIntegrationDomainAttachDetachDevice(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	dom, err := defineTestLxcDomain(conn, "")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()
	const nwXml = `<interface type='network'>
		<mac address='52:54:00:37:aa:c7'/>
		<source network='default'/>
		<model type='virtio'/>
		</interface>`
	if err := dom.AttachDeviceFlags(nwXml, VIR_DOMAIN_DEVICE_MODIFY_CONFIG); err != nil {
		t.Error(err)
		return
	}
	if err := dom.DetachDeviceFlags(nwXml, VIR_DOMAIN_DEVICE_MODIFY_CONFIG); err != nil {
		t.Error(err)
		return
	}
}

func TestStorageVolResize(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	poolPath, err := ioutil.TempDir("", "default-pool-test-1")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(poolPath)
	pool, err := conn.StoragePoolDefineXML(`<pool type='dir'>
                                          <name>default-pool-test-1</name>
                                          <target>
                                          <path>`+poolPath+`</path>
                                          </target>
                                          </pool>`, 0)
	defer func() {
		pool.Undefine()
		pool.Free()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	vol, err := pool.StorageVolCreateXML(testStorageVolXML("", poolPath), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()
	const newCapacityInBytes = 12582912
	if err := vol.Resize(newCapacityInBytes, VIR_STORAGE_VOL_RESIZE_ALLOCATE); err != nil {
		t.Fatal(err)
	}
}

func TestStorageVolWipe(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	poolPath, err := ioutil.TempDir("", "default-pool-test-1")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(poolPath)
	pool, err := conn.StoragePoolDefineXML(`<pool type='dir'>
                                          <name>default-pool-test-1</name>
                                          <target>
                                          <path>`+poolPath+`</path>
                                          </target>
                                          </pool>`, 0)
	defer func() {
		pool.Undefine()
		pool.Free()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	vol, err := pool.StorageVolCreateXML(testStorageVolXML("", poolPath), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()
	if err := vol.Wipe(0); err != nil {
		t.Fatal(err)
	}
}

func TestStorageVolWipePattern(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	poolPath, err := ioutil.TempDir("", "default-pool-test-1")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(poolPath)
	pool, err := conn.StoragePoolDefineXML(`<pool type='dir'>
                                          <name>default-pool-test-1</name>
                                          <target>
                                          <path>`+poolPath+`</path>
                                          </target>
                                          </pool>`, 0)
	defer func() {
		pool.Undefine()
		pool.Free()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	vol, err := pool.StorageVolCreateXML(testStorageVolXML("", poolPath), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()
	if err := vol.WipePattern(VIR_STORAGE_VOL_WIPE_ALG_ZERO, 0); err != nil {
		t.Fatal(err)
	}
}

func testSecretTypeCephFromXML(name string) string {
	var setName string
	if name == "" {
		setName = time.Now().String()
	} else {
		setName = name
	}
	return `<secret ephemeral='no' private='no'>
            <usage type='ceph'>
            <name>` + setName + `</name>
            </usage>
            </secret>`
}

func TestIntegrationSecretDefineUndefine(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(""), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer sec.Free()

	if err := sec.Undefine(); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationSecretGetUUID(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(""), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		sec.Undefine()
		sec.Free()
	}()
	if _, err := sec.GetUUID(); err != nil {
		t.Error(err)
	}
}

func TestIntegrationSecretGetUUIDString(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(""), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		sec.Undefine()
		sec.Free()
	}()
	if _, err := sec.GetUUIDString(); err != nil {
		t.Error(err)
	}
}

func TestIntegrationSecretGetXMLDesc(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(""), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		sec.Undefine()
		sec.Free()
	}()
	if _, err := sec.GetXMLDesc(0); err != nil {
		t.Error(err)
	}
}

func TestIntegrationSecretGetUsageType(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(""), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		sec.Undefine()
		sec.Free()
	}()
	uType, err := sec.GetUsageType()
	if err != nil {
		t.Error(err)
		return
	}
	if uType != VIR_SECRET_USAGE_TYPE_CEPH {
		t.Fatal("unexpected usage type.Expected usage type is Ceph")
	}
}

func TestIntegrationSecretGetUsageID(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	setUsageID := time.Now().String()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(setUsageID), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		sec.Undefine()
		sec.Free()
	}()
	recUsageID, err := sec.GetUsageID()
	if err != nil {
		t.Error(err)
		return
	}
	if recUsageID != setUsageID {
		t.Fatalf("exepected usage ID: %s, got: %s", setUsageID, recUsageID)
	}
}

func TestIntegrationLookupSecretByUsage(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	usageID := time.Now().String()
	sec, err := conn.SecretDefineXML(testSecretTypeCephFromXML(usageID), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		sec.Undefine()
		sec.Free()
	}()
	sec, err = conn.LookupSecretByUsage(VIR_SECRET_USAGE_TYPE_CEPH, usageID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationGetDomainCPUStats(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseConnection()
	dom, err := defineTestLxcDomain(conn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()

	if err := dom.Create(); err != nil {
		t.Fatal(err)
	}
	defer dom.Destroy()

	// ... if @params is NULL and @nparams is 0 and @ncpus is 0, the
	// number of cpus available to query is returned. From the host perspective,
	ncpus, err := dom.GetCPUStats(nil, 0, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ncpus != 1 {
		t.Fatal("Number of CPUs should be 1")
	}

	// ... if @params is NULL and @nparams is 0 and @ncpus is 1,
	// and the return value will be how many statistics are available for the given @start_cpu.
	nparams, err := dom.GetCPUStats(nil, 0, 0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}

	const lxcNumParams = 1
	const lxcParamName = "cpu_time"

	if nparams != lxcNumParams {
		t.Fatal("Number of parameters for this hypervisor should be 2, got ", nparams)
	}
	var params VirTypedParameters
	if _, err = dom.GetCPUStats(&params, nparams, 0, uint32(ncpus), 0); err != nil {
		t.Fatal(err)
	}

	if len(params) != lxcNumParams {
		t.Fatalf("Wanted %d returned parameters, got %d", lxcNumParams, len(params))
	}
	param := params[0]
	if param.Name != lxcParamName {
		t.Fatalf("Wanted param '%s', got '%s'", lxcParamName, param.Name)
	}
	if _, ok := param.Value.(uint64); !ok {
		t.Fatalf("Wanted uint64 param, got %v instead", param.Value)
	}
}

// Not supported on libvirt driver, so no integration test
// func TestGetInterfaceParameters(t *testing.T) {
// 	dom, conn := buildTestDomain()
// 	defer func() {
// 		dom.Undefine()
// 		dom.Free()
// 		conn.CloseConnection()
// 	}()
// 	iface := "either mac or path to interface"
// 	nparams := int(0)
// 	if _, err := dom.GetInterfaceParameters(iface, nil, &nparams, 0); err != nil {
// 		t.Error(err)
// 		return
// 	}

// 	var params VirTypedParameters
// 	if _, err := dom.GetInterfaceParameters(iface, &params, &nparams, 0); err != nil {
// 		t.Error(err)
// 		return
// 	}
// }

func TestIntegrationListAllInterfaces(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	ifaces, err := conn.ListAllInterfaces(0)
	if err != nil {
		t.Fatal(err)
	}
	lookingFor := "lo"
	found := false
	for _, iface := range ifaces {
		name, err := iface.GetName()
		if err != nil {
			t.Fatal(err)
		}
		if name == lookingFor {
			found = true
		}
		iface.Free()
	}
	if found == false {
		t.Fatalf("interface %s not found", lookingFor)
	}
}

func TestIntergrationListAllNWFilters(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	testNWFilterName := time.Now().String()
	filter, err := conn.NWFilterDefineXML(testNWFilterXML(testNWFilterName, "ipv4"))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		filter.Undefine()
		filter.Free()
	}()

	filters, err := conn.ListAllNWFilters(0)
	if len(filters) == 0 {
		t.Fatal("length of []VirNWFilter shouldn't be 0")
	}

	found := false
	for _, f := range filters {
		name, _ := f.GetName()
		if name == testNWFilterName {
			found = true
		}
		f.Free()
	}
	if found == false {
		t.Fatalf("NWFilter %s not found", testNWFilterName)
	}
}

func TestIntegrationDomainBlockStatsFlags(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseConnection()

	dom, err := defineTestLxcDomain(conn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()

	if err := dom.Create(); err != nil {
		t.Fatal(err)
	}
	defer dom.Destroy()

	// special case, count number of parameters
	_, err = dom.BlockStatsFlags("", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationDomainInterfaceStats(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseConnection()

	dom, err := defineTestLxcDomain(conn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()
	const nwXml = `<interface type='network'>
		<mac address='52:54:00:37:aa:c7'/>
		<source network='default'/>
		<model type='virtio'/>
		</interface>`
	if err := dom.AttachDeviceFlags(nwXml, VIR_DOMAIN_DEVICE_MODIFY_CONFIG); err != nil {
		t.Fatal(err)
	}

	if err := dom.Create(); err != nil {
		t.Fatal(err)
	}

	if _, err := dom.InterfaceStats("vnet0"); err != nil {
		t.Error(err)
	}

	if err := dom.Destroy(); err != nil {
		t.Fatal(err)
	}

	if err := dom.DetachDeviceFlags(nwXml, VIR_DOMAIN_DEVICE_MODIFY_CONFIG); err != nil {
		t.Fatal(err)
	}
}

func TestStorageVolUploadDownload(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	poolPath, err := ioutil.TempDir("", "default-pool-test-1")
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(poolPath)
	pool, err := conn.StoragePoolDefineXML(`<pool type='dir'>
                                          <name>default-pool-test-1</name>
                                          <target>
                                          <path>`+poolPath+`</path>
                                          </target>
                                          </pool>`, 0)
	defer func() {
		pool.Undefine()
		pool.Free()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	vol, err := pool.StorageVolCreateXML(testStorageVolXML("", poolPath), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()

	data := []byte{1, 2, 3, 4, 5, 6}

	// write above data to the vol
	// 1. create a stream
	stream, err := NewVirStream(&conn, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		stream.Free()
	}()

	// 2. set it up to upload from stream
	if err := vol.Upload(stream, 0, uint64(len(data)), 0); err != nil {
		stream.Abort()
		t.Fatal(err)
	}

	// 3. do the actual writing
	if n, err := stream.Write(data); err != nil || n != len(data) {
		t.Fatal(err, n)
	}

	// 4. finish!
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}

	// read back the data
	// 1. create a stream
	downStream, err := NewVirStream(&conn, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		downStream.Free()
	}()

	// 2. set it up to download from stream
	if err := vol.Download(downStream, 0, uint64(len(data)), 0); err != nil {
		downStream.Abort()
		t.Fatal(err)
	}

	// 3. do the actual reading
	buf := make([]byte, 1024)
	if n, err := downStream.Read(buf); err != nil || n != len(data) {
		t.Fatal(err, n)
	}

	t.Logf("read back: %#v", buf[:len(data)])

	// 4. finish!
	if err := downStream.Close(); err != nil {
		t.Fatal(err)
	}
}

/*func TestDomainMemoryStats(t *testing.T) {
	conn, err := NewVirConnection("lxc:///")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	dom, err := defineTestLxcDomain(conn, "")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		dom.Undefine()
		dom.Free()
	}()
	if err := dom.Create(); err != nil {
		t.Fatal(err)
	}
	defer dom.Destroy()

	ms, err := dom.MemoryStats(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatal("Should have got one result, got", len(ms))
	}
}*/
