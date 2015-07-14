package sandbox

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netns"
)

// Sandbox represents a network sandbox, identified by a specific key.  It
// holds a list of Interfaces, routes etc, and more can be added dynamically.
type Sandbox interface {
	// The path where the network namespace is mounted.
	Key() string

	// The collection of Interface previously added with the AddInterface
	// method. Note that this doesn't incude network interfaces added in any
	// other way (such as the default loopback interface which are automatically
	// created on creation of a sandbox).
	Interfaces() []*Interface

	// Add an existing Interface to this sandbox. The operation will rename
	// from the Interface SrcName to DstName as it moves, and reconfigure the
	// interface according to the specified settings. The caller is expected
	// to only provide a prefix for DstName. The AddInterface api will auto-generate
	// an appropriate suffix for the DstName to disambiguate.
	AddInterface(*Interface) error

	// Remove an interface from the sandbox by renamin to original name
	// and moving it out of the sandbox.
	RemoveInterface(*Interface) error

	// Set default IPv4 gateway for the sandbox
	SetGateway(gw net.IP) error

	// Set default IPv6 gateway for the sandbox
	SetGatewayIPv6(gw net.IP) error

	// Destroy the sandbox
	Destroy() error
}

// Info represents all possible information that
// the driver wants to place in the sandbox which includes
// interfaces, routes and gateway
type Info struct {
	Interfaces []*Interface

	// IPv4 gateway for the sandbox.
	Gateway net.IP

	// IPv6 gateway for the sandbox.
	GatewayIPv6 net.IP

	// TODO: Add routes and ip tables etc.
}

// Interface represents the settings and identity of a network device. It is
// used as a return type for Network.Link, and it is common practice for the
// caller to use this information when moving interface SrcName from host
// namespace to DstName in a different net namespace with the appropriate
// network settings.
type Interface struct {
	// The name of the interface in the origin network namespace.
	SrcName string

	// The name that will be assigned to the interface once moves inside a
	// network namespace. When the caller passes in a DstName, it is only
	// expected to pass a prefix. The name will modified with an appropriately
	// auto-generated suffix.
	DstName string

	// IPv4 address for the interface.
	Address *net.IPNet

	// IPv6 address for the interface.
	AddressIPv6 *net.IPNet

	// Parent sandbox's key
	sandboxKey string
}

// GetCopy returns a copy of this Interface structure
func (i *Interface) GetCopy() *Interface {
	return &Interface{
		SrcName:     i.SrcName,
		DstName:     i.DstName,
		Address:     types.GetIPNetCopy(i.Address),
		AddressIPv6: types.GetIPNetCopy(i.AddressIPv6),
	}
}

// Equal checks if this instance of Interface is equal to the passed one
func (i *Interface) Equal(o *Interface) bool {
	if i == o {
		return true
	}

	if o == nil {
		return false
	}

	if i.SrcName != o.SrcName || i.DstName != o.DstName {
		return false
	}

	if !types.CompareIPNet(i.Address, o.Address) {
		return false
	}

	if !types.CompareIPNet(i.AddressIPv6, o.AddressIPv6) {
		return false
	}

	return true
}

// GetCopy returns a copy of this SandboxInfo structure
func (s *Info) GetCopy() *Info {
	list := make([]*Interface, len(s.Interfaces))
	for i, iface := range s.Interfaces {
		list[i] = iface.GetCopy()
	}
	gw := types.GetIPCopy(s.Gateway)
	gw6 := types.GetIPCopy(s.GatewayIPv6)

	return &Info{Interfaces: list, Gateway: gw, GatewayIPv6: gw6}
}

// Equal checks if this instance of SandboxInfo is equal to the passed one
func (s *Info) Equal(o *Info) bool {
	if s == o {
		return true
	}

	if o == nil {
		return false
	}

	if !s.Gateway.Equal(o.Gateway) {
		return false
	}

	if !s.GatewayIPv6.Equal(o.GatewayIPv6) {
		return false
	}

	if (s.Interfaces == nil && o.Interfaces != nil) ||
		(s.Interfaces != nil && o.Interfaces == nil) ||
		(len(s.Interfaces) != len(o.Interfaces)) {
		return false
	}

	// Note: At the moment, the two lists must be in the same order
	for i := 0; i < len(s.Interfaces); i++ {
		if !s.Interfaces[i].Equal(o.Interfaces[i]) {
			return false
		}
	}

	return true

}

func nsInvoke(path string, inNsfunc func(callerFD int) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return err
	}
	defer origns.Close()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", path, err)
	}
	defer f.Close()

	nsFD := f.Fd()

	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer netns.Set(origns)

	// Invoked after the namespace switch.
	return inNsfunc(int(origns))
}

// Statistics returns the statistics for this interface
func (i *Interface) Statistics() (*InterfaceStatistics, error) {

	s := &InterfaceStatistics{}

	err := nsInvoke(i.sandboxKey, func(callerFD int) error {
		// For some reason ioutil.ReadFile(netStatsFile) reads the file in
		// the default netns when this code is invoked from docker.
		// Executing "cat <netStatsFile>" works as expected.
		data, err := exec.Command("cat", netStatsFile).Output()
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", netStatsFile, err)
		}
		return scanInterfaceStats(string(data), i.DstName, s)
	})

	if err != nil {
		err = fmt.Errorf("failed to retrieve the statistics for %s in netns %s: %v", i.DstName, i.sandboxKey, err)
	}

	return s, err
}

// InterfaceStatistics represents the interface's statistics
type InterfaceStatistics struct {
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDropped uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDropped uint64
}

func (is *InterfaceStatistics) String() string {
	return fmt.Sprintf("\nRxBytes: %d, RxPackets: %d, RxErrors: %d, RxDropped: %d, TxBytes: %d, TxPackets: %d, TxErrors: %d, TxDropped: %d",
		is.RxBytes, is.RxPackets, is.RxErrors, is.RxDropped, is.TxBytes, is.TxPackets, is.TxErrors, is.TxDropped)
}

// In older kernels (like the one in Centos 6.6 distro) sysctl does not have netns support. Therefore
// we cannot gather the statistics from /sys/class/net/<dev>/statistics/<counter> files. Per-netns stats
// are naturally found in /proc/net/dev in kernels which support netns (ifconfig relyes on that).
const (
	netStatsFile = "/proc/net/dev"
	base         = "[ ]*%s:([ ]+[0-9]+){16}"
)

func scanInterfaceStats(data, ifName string, i *InterfaceStatistics) error {
	var (
		bktStr string
		bkt    uint64
	)

	regex := fmt.Sprintf(base, ifName)
	re := regexp.MustCompile(regex)
	line := re.FindString(data)

	_, err := fmt.Sscanf(line, "%s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&bktStr, &i.RxBytes, &i.RxPackets, &i.RxErrors, &i.RxDropped, &bkt, &bkt, &bkt,
		&bkt, &i.TxBytes, &i.TxPackets, &i.TxErrors, &i.TxDropped, &bkt, &bkt, &bkt, &bkt)

	return err
}
