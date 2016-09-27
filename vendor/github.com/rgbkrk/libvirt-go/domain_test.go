package libvirt

import (
	"strings"
	"testing"
	"time"
)

func buildTestQEMUDomain() (VirDomain, VirConnection) {
	conn := buildTestQEMUConnection()
	dom, err := conn.DomainDefineXML(`<domain type="qemu">
		<name>` + strings.Replace(time.Now().String(), " ", "_", -1) + `</name>
		<memory unit="KiB">128</memory>
		<os>
			<type>hvm</type>
		</os>
	</domain>`)
	if err != nil {
		panic(err)
	}
	return dom, conn
}
func buildTestDomain() (VirDomain, VirConnection) {
	conn := buildTestConnection()
	dom, err := conn.DomainDefineXML(`<domain type="test">
		<name>` + time.Now().String() + `</name>
		<memory unit="KiB">8192</memory>
		<os>
			<type>hvm</type>
		</os>
	</domain>`)
	if err != nil {
		panic(err)
	}
	return dom, conn
}

func TestUndefineDomain(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	name, err := dom.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if err := dom.Undefine(); err != nil {
		t.Error(err)
		return
	}
	if _, err := conn.LookupDomainByName(name); err == nil {
		t.Fatal("Shouldn't have been able to find domain")
		return
	}
}

func TestGetDomainName(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Undefine()
		dom.Free()
		conn.CloseConnection()
	}()
	if _, err := dom.GetName(); err != nil {
		t.Error(err)
		return
	}
}

func TestGetDomainState(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	state, err := dom.GetState()
	if err != nil {
		t.Error(err)
		return
	}
	if len(state) != 2 {
		t.Error("Length of domain state should be 2")
		return
	}
	if state[0] != 5 || state[1] != 0 {
		t.Error("Domain state in test transport should be [5 0]")
		return
	}
}

func TestGetDomainID(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()

	if err := dom.Create(); err != nil {
		t.Error("Failed to create domain")
	}

	if id, err := dom.GetID(); id == ^uint(0) || err != nil {
		dom.Destroy()
		t.Error("Couldn't get domain ID")
		return
	}
	dom.Destroy()
}

func TestGetDomainUUID(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	_, err := dom.GetUUID()
	// how to test uuid validity?
	if err != nil {
		t.Error(err)
		return
	}
}

func TestGetDomainUUIDString(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	_, err := dom.GetUUIDString()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestGetDomainInfo(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	_, err := dom.GetInfo()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestGetDomainXMLDesc(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	_, err := dom.GetXMLDesc(0)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestCreateDomainSnapshotXML(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	ss, err := dom.CreateSnapshotXML(`
		<domainsnapshot>
			<description>Test snapshot that will fail because its unsupported</description>
		</domainsnapshot>
	`, 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer ss.Free()
}

func TestSaveDomain(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	// get the name so we can get a handle on it later
	domName, err := dom.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	const tmpFile = "/tmp/libvirt-go-test.tmp"
	if err := dom.Save(tmpFile); err != nil {
		t.Error(err)
		return
	}
	if err := conn.Restore(tmpFile); err != nil {
		t.Error(err)
		return
	}
	if _, err = conn.LookupDomainByName(domName); err != nil {
		t.Error(err)
		return
	}
}

func TestSaveDomainFlags(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	const srcFile = "/tmp/libvirt-go-test.tmp"
	if err := dom.SaveFlags(srcFile, "", 0); err == nil {
		t.Fatal("expected xml modification unsupported")
		return
	}
}

func TestCreateDestroyDomain(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	state, err := dom.GetState()
	if err != nil {
		t.Error(err)
		return
	}
	if state[0] != VIR_DOMAIN_RUNNING {
		t.Fatal("Domain should be running")
		return
	}
	if err = dom.Destroy(); err != nil {
		t.Error(err)
		return
	}
	state, err = dom.GetState()
	if err != nil {
		t.Error(err)
		return
	}
	if state[0] != VIR_DOMAIN_SHUTOFF {
		t.Fatal("Domain should be destroyed")
		return
	}
}

func TestShutdownDomain(t *testing.T) {
	dom, conn := buildTestDomain()
	defer conn.CloseConnection()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	if err := dom.Shutdown(); err != nil {
		t.Error(err)
		return
	}
	state, err := dom.GetState()
	if err != nil {
		t.Error(err)
		return
	}
	if state[0] != 5 || state[1] != 1 {
		t.Fatal("state should be [5 1]")
		return
	}
}

func TestShutdownReboot(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Reboot(0); err != nil {
		t.Error(err)
		return
	}
}

func TestDomainAutostart(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	as, err := dom.GetAutostart()
	if err != nil {
		t.Error(err)
		return
	}
	if as {
		t.Fatal("autostart should be false")
		return
	}
	if err := dom.SetAutostart(true); err != nil {
		t.Error(err)
		return
	}
	as, err = dom.GetAutostart()
	if err != nil {
		t.Error(err)
		return
	}
	if !as {
		t.Fatal("autostart should be true")
		return
	}
}

func TestDomainIsActive(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Log(err)
		return
	}
	active, err := dom.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if !active {
		t.Fatal("Domain should be active")
		return
	}
	if err := dom.Destroy(); err != nil {
		t.Error(err)
		return
	}
	active, err = dom.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if active {
		t.Fatal("Domain should be inactive")
		return
	}
}

func TestDomainSetMaxMemory(t *testing.T) {
	const mem = 8192 * 100
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.SetMaxMemory(mem); err != nil {
		t.Error(err)
		return
	}
}

func TestDomainSetMemory(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	if err := dom.SetMemory(1024); err != nil {
		t.Error(err)
		return
	}
}

func TestDomainSetVcpus(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	if err := dom.SetVcpus(1); err != nil {
		t.Error(err)
		return
	}
	if err := dom.SetVcpusFlags(1, VIR_DOMAIN_VCPU_LIVE); err != nil {
		t.Error(err)
		return
	}
}

func TestDomainFree(t *testing.T) {
	dom, conn := buildTestDomain()
	defer conn.CloseConnection()
	if err := dom.Free(); err != nil {
		t.Error(err)
		return
	}
}

func TestDomainSuspend(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	defer dom.Destroy()
	if err := dom.Suspend(); err != nil {
		t.Error(err)
		return
	}
	defer dom.Resume()
}

func TesDomainShutdownFlags(t *testing.T) {
	dom, conn := buildTestDomain()
	defer conn.CloseConnection()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	if err := dom.ShutdownFlags(VIR_DOMAIN_SHUTDOWN_SIGNAL); err != nil {
		t.Error(err)
		return
	}
	state, err := dom.GetState()
	if err != nil {
		t.Error(err)
		return
	}
	if state[0] != 5 || state[1] != 1 {
		t.Fatal("state should be [5 1]")
		return
	}
}

func TesDomainDestoryFlags(t *testing.T) {
	dom, conn := buildTestDomain()
	defer conn.CloseConnection()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	if err := dom.DestroyFlags(VIR_DOMAIN_DESTROY_GRACEFUL); err != nil {
		t.Error(err)
		return
	}
	state, err := dom.GetState()
	if err != nil {
		t.Error(err)
		return
	}
	if state[0] != 5 || state[1] != 1 {
		t.Fatal("state should be [5 1]")
		return
	}
}

func TestDomainScreenshot(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	stream, err := NewVirStream(&conn, 0)
	if err != nil {
		t.Fatalf("failed to create new stream: %s", err)
	}
	defer stream.Free()
	mime, err := dom.Screenshot(stream, 0, 0)
	if err != nil {
		t.Fatalf("failed to take screenshot: %s", err)
	}
	if strings.Index(mime, "image/") != 0 {
		t.Fatalf("Wanted image/*, got %s", mime)
	}
}

func TestDomainGetVcpus(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	defer dom.Destroy()

	stats, err := dom.GetVcpus(1)
	if err != nil {
		t.Fatal(err)
	}

	if len(stats) != 1 {
		t.Fatal("should have 1 cpu")
	}

	if stats[0].State != 1 {
		t.Fatal("state should be 1")
	}
}

func TestDomainGetVcpusFlags(t *testing.T) {
	dom, conn := buildTestDomain()
	defer func() {
		dom.Free()
		conn.CloseConnection()
	}()
	if err := dom.Create(); err != nil {
		t.Error(err)
		return
	}
	defer dom.Destroy()

	num, err := dom.GetVcpusFlags(0)
	if err != nil {
		t.Fatal(err)
	}

	if num != 1 {
		t.Fatal("should have 1 cpu", num)
	}
}

func TestQemuMonitorCommand(t *testing.T) {
	dom, conn := buildTestQEMUDomain()
	defer func() {
        dom.Destroy()
		dom.Undefine()
		dom.Free()
		conn.CloseConnection()
	}()

    if err := dom.Create(); err != nil {
        t.Error(err)
        return
    }

	if _, err := dom.QemuMonitorCommand(VIR_DOMAIN_QEMU_MONITOR_COMMAND_DEFAULT, "{\"execute\" : \"query-cpus\"}"); err != nil {
		t.Error(err)
		return
	}

	if _, err := dom.QemuMonitorCommand(VIR_DOMAIN_QEMU_MONITOR_COMMAND_HMP, "info cpus"); err != nil {
		t.Error(err)
		return
	}
}
