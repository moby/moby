// +build linux

package libvirt

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	libvirtgo "github.com/rgbkrk/libvirt-go"
        "github.com/Sirupsen/logrus"
        "github.com/docker/docker/container"
)

var LibvirtdAddress = "qemu:///system"

type LibvirtDriver struct {
	sync.Mutex
	conn libvirtgo.VirConnection
}

type LibvirtContext struct {
	driver *LibvirtDriver
	domain *libvirtgo.VirDomain
}

type BootConfig struct {
	CPU              int
        DefaultMaxCpus   int
        DefaultMaxMem    int
	Memory           int
        BootDisk         string
        MountPath        string
        MemoryPath       string
}

func InitDriver() *LibvirtDriver {
	/* Libvirt adds memballoon device by default */
	hypervisor.PciAddrFrom = 0x06
	conn, err := libvirtgo.NewVirConnection(LibvirtdAddress)
	if err != nil {
		logrus.Error("fail to connect to libvirtd ", LibvirtdAddress, err)
		return nil
	}

	return &LibvirtDriver{
		conn: conn,
	}
}

func (ld *LibvirtDriver) Name() string {
	return "libvirt"
}

func (ld *LibvirtDriver) InitContext(containerId string) LibvirtContext {
	domain, err := ld.lookupDomainByName(containerId)
	if err == nil {
		return nil, fmt.Errorf("Already created a container with id as %v", name)
	}

	return &LibvirtContext{
		driver: ld,
	}
}
		domain: &domain,

func (ld *LibvirtDriver) checkConnection() error {
	if alive, _ := ld.conn.IsAlive(); !alive {
		logrus.Info("libvirt disconnected, reconnect")
		conn, err := libvirtgo.NewVirConnection(LibvirtdAddress)
		if err != nil {
			return err
		}
		ld.conn.CloseConnection()
		ld.conn = conn
		return nil
	}
	return fmt.Errorf("connection is alive")
}

func (ld *LibvirtDriver) lookupDomainByName(name string) (libvirtgo.VirDomain, error) {
	ld.Lock()
	defer ld.Unlock()

	domain, err := ld.conn.LookupDomainByName(name)
	if err != nil {
		if res := ld.checkConnection(); res != nil {
			logrus.Error(res)
			return domain, err
		}
		domain, err = ld.conn.LookupDomainByName(name)
	}

	return domain, err
}

func CreateTemplateQemuWrapper(execPath, qemuPath string, boot *BootConfig) error {
	templateQemuWrapper := `#!/bin/bash

# qemu wrapper for libvirt driver for templating
# Do NOT modify

memsize="%d"          # template memory size
maxcpuid="%d"         # MaxCpus-1
qemupath="%s"         # qemu real path, exec.LookPath("qemu-system-x86_64")

argv=()

while true
do
	arg="$1"
	shift || break

	# wrap qemu after we see the -numa argument
	if [ "x${arg}" = "x-numa" ]; then
		if [ "next_arg=$1" != "next_arg=node,nodeid=0,cpus=0-${maxcpuid},mem=${memsize}" ]; then
			echo "unexpected numa argument: $1" >&2
			exit 1
		fi

		if [ -e "${statepath}" ]; then
			argv+=("-incoming" "exec:cat $statepath")
			share=off
		else
			share=on
		fi

		#argv+=(-global kvm-pit.lost_tick_policy=discard)
		argv+=("-object" "memory-backend-file,id=isolated-template-memory,size=${memsize}M,mem-path=${mempath},share=${share}")
		argv+=("-numa" "node,nodeid=0,cpus=0-${maxcpuid},memdev=isolated-template-memory")
		shift # skip next arg
	else
		argv+=("${arg}")
	fi
done

exec "${qemupath}" "${argv[@]}"
`

	data := []byte(fmt.Sprintf(templateQemuWrapper, boot.Memory, boot.DefaultMaxCpus-1, qemuPath))

	return ioutil.WriteFile(execPath, data, 0700)
}

type memory struct {
	Unit    string `xml:"unit,attr"`
	Content int    `xml:",chardata"`
}

type maxmem struct {
	Unit    string `xml:"unit,attr"`
	Slots   string `xml:"slots,attr"`
	Content int    `xml:",chardata"`
}

type vcpu struct {
	Placement string `xml:"placement,attr"`
	Current   string `xml:"current,attr"`
	Content   int    `xml:",chardata"`
}

type cpumodel struct {
	Fallback string `xml:"fallback,attr"`
	Content  string `xml:",chardata"`
}

type cell struct {
	Id     string `xml:"id,attr"`
	Cpus   string `xml:"cpus,attr"`
	Memory string `xml:"memory,attr"`
	Unit   string `xml:"unit,attr"`
}

type numa struct {
	Cell []cell `xml:"cell"`
}

type cpu struct {
	Mode  string    `xml:"mode,attr"`
	Match string    `xml:"match,attr,omitempty"`
	Model *cpumodel `xml:"model,omitempty"`
	Numa  *numa     `xml:"numa,omitempty"`
}

type ostype struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr"`
	Content string `xml:",chardata"`
}

type osloader struct {
	Type     string `xml:"type,attr"`
	ReadOnly string `xml:"readonly,attr"`
	Content  string `xml:",chardata"`
}

type domainos struct {
	Supported string    `xml:"supported,attr"`
	Type      ostype    `xml:"type"`
	Kernel    string    `xml:"kernel,omitempty"`
	Initrd    string    `xml:"initrd,omitempty"`
	Cmdline   string    `xml:"cmdline,omitempty"`
	Loader    *osloader `xml:"loader,omitempty"`
	Nvram     string    `xml:"nvram,omitempty"`
}

type features struct {
	Acpi string `xml:"acpi"`
}

type address struct {
	Type       string `xml:"type,attr"`
	Domain     string `xml:"domain,attr,omitempty"`
	Controller string `xml:"controller,attr,omitempty"`
	Bus        string `xml:"bus,attr"`
	Slot       string `xml:"slot,attr,omitempty"`
	Function   string `xml:"function,attr,omitempty"`
	Target     int    `xml:"target,attr,omitempty"`
	Unit       int    `xml:"unit,attr,omitempty"`
}

type controller struct {
	Type    string   `xml:"type,attr"`
	Index   string   `xml:"index,attr,omitempty"`
	Model   string   `xml:"model,attr,omitempty"`
	Address *address `xml:"address,omitempty"`
}

type fsdriver struct {
	Type string `xml:"type,attr"`
}

type fspath struct {
	Dir string `xml:"dir,attr"`
}

type filesystem struct {
	Type       string   `xml:"type,attr"`
	Accessmode string   `xml:"accessmode,attr"`
	Driver     fsdriver `xml:"driver"`
	Source     fspath   `xml:"source"`
	Target     fspath   `xml:"target"`
	Address    *address `xml:"address"`
}

type channsrc struct {
	Mode string `xml:"mode,attr"`
	Path string `xml:"path,attr"`
}

type channtgt struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`
}

type channel struct {
	Type   string   `xml:"type,attr"`
	Source channsrc `xml:"source"`
	Target channtgt `xml:"target"`
}

type constgt struct {
	Type string `xml:"type,attr"`
	Port string `xml:"port,attr"`
}

type console struct {
	Type   string   `xml:"type,attr"`
	Source channsrc `xml:"source"`
	Target constgt  `xml:"target"`
}

type memballoon struct {
	Model   string   `xml:"model,attr"`
	Address *address `xml:"address"`
}

type device struct {
	Emulator    string       `xml:"emulator"`
	Controllers []controller `xml:"controller"`
	Filesystems []filesystem `xml:"filesystem"`
	Channels    []channel    `xml:"channel"`
	Console     console      `xml:"console"`
	Memballoon  memballoon   `xml:"memballoon"`
}

type seclab struct {
	Type string `xml:"type,attr"`
}

type domain struct {
	XMLName    xml.Name `xml:"domain"`
	Type       string   `xml:"type,attr"`
	Name       string   `xml:"name"`
	Memory     memory   `xml:"memory"`
	MaxMem     *maxmem  `xml:"maxMemory,omitempty"`
	VCpu       vcpu     `xml:"vcpu"`
	OS         domainos `xml:"os"`
	Features   features `xml:"features"`
	CPU        cpu      `xml:"cpu"`
	OnPowerOff string   `xml:"on_poweroff"`
	OnReboot   string   `xml:"on_reboot"`
	OnCrash    string   `xml:"on_crash"`
	Devices    device   `xml:"devices"`
	SecLabel   seclab   `xml:"seclabel"`
}

func (lc *LibvirtContext) domainXml(container *container.Container) (string, error) {
	boot := &BootConfig{
	         CPU:            1,
                 DefaultMaxCpus: 2,
                 DefaultMaxMem:  128,
		 Memory:         128,
                 MemoryPath:     "/var/lib/docker/libvirt",
		}

	dom := &domain{
		Type: "kvm",
		Name: container.ID,
	}

	dom.Memory.Unit = "MiB"
	dom.Memory.Content = boot.Memory

	dom.VCpu.Placement = "static"
	dom.VCpu.Current = strconv.Itoa(boot.CPU)
	dom.VCpu.Content = boot.CPU

	dom.OS.Supported = "yes"
	dom.OS.Type.Arch = "x86_64"
	dom.OS.Type.Machine = "pc-i440fx-2.0"
	dom.OS.Type.Content = "hvm"

	dom.SecLabel.Type = "none"

	dom.CPU.Mode = "host-passthrough"
	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		dom.Type = "qemu"
		dom.CPU.Mode = "host-model"
		dom.CPU.Match = "exact"
		dom.CPU.Model = &cpumodel{
			Fallback: "allow",
			Content:  "core2duo",
		}
	}

	if ctx.Boot.HotAddCpuMem {
		dom.OS.Type.Machine = "pc-i440fx-2.1"
		dom.VCpu.Content = boot.DefaultMaxCpus
		dom.MaxMem = &maxmem{Unit: "MiB", Slots: "1", Content: boot.DefaultMaxMem}

		cells := make([]cell, 1)
		cells[0].Id = "0"
		cells[0].Cpus = fmt.Sprintf("0-%d", boot.DefaultMaxCpus-1)
		cells[0].Memory = strconv.Itoa(boot.Memory * 1024) // older libvirt always considers unit='KiB'
		cells[0].Unit = "KiB"

		dom.CPU.Numa = &numa{Cell: cells}
	}

	cmd, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return "", fmt.Errorf("cannot find qemu-system-x86_64 binary")
	}
	dom.Devices.Emulator = cmd

	qemuTemplateWrapper := filepath.Join(filepath.Dir(boot.MemoryPath), "libvirt-qemu-template-wrapper.sh")
	if boot.BootToBeTemplate {
		err := CreateTemplateQemuWrapper(qemuTemplateWrapper, cmd, boot)
		if err != nil {
			return "", err
		}
		dom.Devices.Emulator = qemuTemplateWrapper
	} else if boot.BootFromTemplate {
		// the wrapper was created when the template was created
		dom.Devices.Emulator = qemuTemplateWrapper
	}

	dom.OnPowerOff = "destroy"
	dom.OnReboot = "destroy"
	dom.OnCrash = "destroy"

	pcicontroller := controller{
		Type:  "pci",
		Index: "0",
		Model: "pci-root",
	}
	dom.Devices.Controllers = append(dom.Devices.Controllers, pcicontroller)

	serialcontroller := controller{
		Type:  "virtio-serial",
		Index: "0",
		Address: &address{
			Type:     "pci",
			Domain:   "0x0000",
			Bus:      "0x00",
			Slot:     "0x02",
			Function: "0x00",
		},
	}
	dom.Devices.Controllers = append(dom.Devices.Controllers, serialcontroller)

	scsicontroller := controller{
		Type:  "scsi",
		Index: "0",
		Model: "virtio-scsi",
		Address: &address{
			Type:     "pci",
			Domain:   "0x0000",
			Bus:      "0x00",
			Slot:     "0x03",
			Function: "0x00",
		},
	}
	dom.Devices.Controllers = append(dom.Devices.Controllers, scsicontroller)

	usbcontroller := controller{
		Type:  "usb",
		Model: "none",
	}
	dom.Devices.Controllers = append(dom.Devices.Controllers, usbcontroller)

	sharedfs := filesystem{
		Type:       "mount",
		Accessmode: "squash",
		Driver: fsdriver{
			Type: "path",
		},
		Source: fspath{
			Dir: ctx.ShareDir,
		},
		Target: fspath{
			Dir: hypervisor.ShareDirTag,
		},
		Address: &address{
			Type:     "pci",
			Domain:   "0x0000",
			Bus:      "0x00",
			Slot:     "0x04",
			Function: "0x00",
		},
	}
	dom.Devices.Filesystems = append(dom.Devices.Filesystems, sharedfs)

	dom.Devices.Memballoon = memballoon{
		Model: "virtio",
		Address: &address{
			Type:     "pci",
			Domain:   "0x0000",
			Bus:      "0x00",
			Slot:     "0x05",
			Function: "0x00",
		},
	}

	data, err := xml.Marshal(dom)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (lc *LibvirtContext) Launch(container *container.Container) {
	domainXml, err := lc.domainXml(container)
	if err != nil {
		logrus.Error("Fail to get domain xml configuration:", err)
		return
	}
	logrus.Infof("domainXML: %v", domainXml)
	var domain libvirtgo.VirDomain

	domain, err = lc.driver.conn.DomainCreateXML(domainXml, libvirtgo.VIR_DOMAIN_NONE)
	if err != nil {
		logrus.Error("Fail to launch domain ", err)
		return
	}

	lc.domain = &domain
	err = lc.domain.SetMemoryStatsPeriod(1, 0)
	if err != nil {
		logrus.Errorf("SetMemoryStatsPeriod failed for domain %v", ctx.Id)
	}
}

func (lc *LibvirtContext) Shutdown() {
	if lc.domain == nil {
		return
	}

	lc.domain.DestroyFlags(libvirtgo.VIR_DOMAIN_DESTROY_DEFAULT)
	logrus.Infof("Domain shutdown")
}

func (lc *LibvirtContext) Close() {
	lc.domain = nil
}

func (lc *LibvirtContext) Pause(pause bool) error {
	if lc.domain == nil {
		return fmt.Errorf("Cannot find domain")
	}

	if pause {
		logrus.Infof("Domain suspended:", lc.domain.Suspend())
	} else {
		logrus.Infof("Domain resumed:", lc.domain.Resume())
	}
}

/*type nicmac struct {
	Address string `xml:"address,attr"`
}

type nicsrc struct {
	Bridge string `xml:"bridge,attr"`
}

type nictgt struct {
	Device string `xml:"dev,attr,omitempty"`
}

type nicmodel fsdriver

type nicBound struct {
	// in kilobytes/second
	Average string `xml:"average,attr"`
	Peak    string `xml:"peak,attr"`
}

type bandwidth struct {
	XMLName  xml.Name  `xml:"bandwidth"`
	Inbound  *nicBound `xml:"inbound,omitempty"`
	Outbound *nicBound `xml:"outbound,omitempty"`
}

type nic struct {
	XMLName   xml.Name   `xml:"interface"`
	Type      string     `xml:"type,attr"`
	Mac       nicmac     `xml:"mac"`
	Source    nicsrc     `xml:"source"`
	Target    *nictgt    `xml:"target,omitempty"`
	Model     nicmodel   `xml:"model"`
	Address   *address   `xml:"address"`
	Bandwidth *bandwidth `xml:"bandwidth,omitempty"`
}

func nicXml(container *container.Container, config *BootConfig) (string, error) {
	slot := fmt.Sprintf("0x%x", addr)

	n := nic{
		Type: "bridge",
		Mac: nicmac{
			Address: mac,
		},
		Source: nicsrc{
			Bridge: bridge,
		},
		Target: &nictgt{
			Device: device,
		},
		Model: nicmodel{
			Type: "virtio",
		},
		Address: &address{
			Type:     "pci",
			Domain:   "0x0000",
			Bus:      "0x00",
			Slot:     slot,
			Function: "0x0",
		},
	}

	if config.InboundAverage != "" || config.OutboundAverage != "" {
		b := &bandwidth{}
		if config.InboundAverage != "" {
			b.Inbound = &nicBound{Average: config.InboundAverage, Peak: config.InboundPeak}
		}
		if config.OutboundAverage != "" {
			b.Outbound = &nicBound{Average: config.OutboundAverage, Peak: config.OutboundPeak}
		}
		n.Bandwidth = b
	}

	data, err := xml.Marshal(n)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (lc *LibvirtContext) AddNic(ctx *hypervisor.VmContext, host *hypervisor.HostNicInfo, guest *hypervisor.GuestNicInfo, result chan<- hypervisor.VmEvent) {
	if lc.domain == nil {
		logrus.Error("Cannot find domain")
		result <- &hypervisor.DeviceFailed{
			Session: nil,
		}
		return
	}

	nicXml, err := nicXml(host.Bridge, host.Device, host.Mac, guest.Busaddr, ctx.Boot)
	if err != nil {
		logrus.Error("generate attach-nic-xml failed, ", err.Error())
		result <- &hypervisor.DeviceFailed{
			Session: nil,
		}
		return
	}
	logrus.Infof("nicxml: %s", nicXml)

	err = lc.domain.AttachDeviceFlags(nicXml, libvirtgo.VIR_DOMAIN_DEVICE_MODIFY_LIVE)
	if err != nil {
		logrus.Error("attach nic failed, ", err.Error())
		result <- &hypervisor.DeviceFailed{
			Session: nil,
		}
		return
	}

	result <- &hypervisor.NetDevInsertedEvent{
		Index:      guest.Index,
		DeviceName: guest.Device,
		Address:    guest.Busaddr,
	}
}

func (lc *LibvirtContext) RemoveNic(ctx *hypervisor.VmContext, n *hypervisor.InterfaceCreated, callback hypervisor.VmEvent) {
	if lc.domain == nil {
		logrus.Error("Cannot find domain")
		ctx.Hub <- &hypervisor.DeviceFailed{
			Session: nil,
		}
		return
	}

	nicXml, err := nicXml(n.Bridge, n.HostDevice, n.MacAddr, n.PCIAddr, ctx.Boot)
	if err != nil {
		logrus.Error("generate detach-nic-xml failed, ", err.Error())
		ctx.Hub <- &hypervisor.DeviceFailed{
			Session: callback,
		}
		return
	}

	err = lc.domain.DetachDeviceFlags(nicXml, libvirtgo.VIR_DOMAIN_DEVICE_MODIFY_LIVE)
	if err != nil {
		logrus.Error("detach nic failed, ", err.Error())
		ctx.Hub <- &hypervisor.DeviceFailed{
			Session: callback,
		}
		return
	}
	ctx.Hub <- callback
}

func (lc *LibvirtContext) SetCpus(ctx *hypervisor.VmContext, cpus int, result chan<- error) {
	logrus.Infof("setcpus %d", cpus)
	if lc.domain == nil {
		result <- fmt.Errorf("Cannot find domain")
		return
	}

	err := lc.domain.SetVcpusFlags(uint(cpus), libvirtgo.VIR_DOMAIN_VCPU_LIVE)
	result <- err
}

func (lc *LibvirtContext) AddMem(ctx *hypervisor.VmContext, slot, size int, result chan<- error) {
	memdevXml := fmt.Sprintf("<memory model='dimm'><target><size unit='MiB'>%d</size><node>0</node></target></memory>", size)
	logrus.Infof("memdevXml: %s", memdevXml)
	if lc.domain == nil {
		result <- fmt.Errorf("Cannot find domain")
		return
	}

	err := lc.domain.AttachDeviceFlags(memdevXml, libvirtgo.VIR_DOMAIN_DEVICE_MODIFY_LIVE)
	result <- err
}

func (lc *LibvirtContext) Save(ctx *hypervisor.VmContext, path string, result chan<- error) {
	logrus.Infof("save domain to: %s", path)

	if ctx.Boot.BootToBeTemplate {
		err := exec.Command("virsh", "-c", LibvirtdAddress, "qemu-monitor-command", ctx.Id, "--hmp", "migrate_set_capability bypass-shared-memory on").Run()
		if err != nil {
			result <- err
			return
		}
	}

	// lc.domain.Save(path) will have libvirt header and will destroy the vm
	// TODO: use virsh qemu-monitor-event to query until completed
	err := exec.Command("virsh", "-c", LibvirtdAddress, "qemu-monitor-command", ctx.Id, "--hmp", fmt.Sprintf("migrate exec:cat>%s", path)).Run()
	result <- err
}*/
