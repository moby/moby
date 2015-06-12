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

	// Add an existing Interface to this sandbox. The operation will rename
	// from the Interface SrcName to DstName as it moves, and reconfigure the
	// interface according to the specified settings. The caller is expected
	// to only provide a prefix for DstName. The AddInterface api will auto-generate
	// an appropriate suffix for the DstName to disambiguate.
	AddInterface(SrcName string, DstPrefix string, options ...IfaceOption) error

	// Set default IPv4 gateway for the sandbox
	SetGateway(gw net.IP) error

	// Set default IPv6 gateway for the sandbox
	SetGatewayIPv6(gw net.IP) error

	// Unset the previously set default IPv4 gateway in the sandbox
	UnsetGateway() error

	// Unset the previously set default IPv6 gateway in the sandbox
	UnsetGatewayIPv6() error

	// Add a static route to the sandbox.
	AddStaticRoute(*types.StaticRoute) error

	// Remove a static route from the sandbox.
	RemoveStaticRoute(*types.StaticRoute) error

	// Returns an interface with methods to set interface options.
	InterfaceOptions() IfaceOptionSetter

	// Returns an interface with methods to get sandbox state.
	Info() Info

	// Destroy the sandbox
	Destroy() error
}

// IfaceOptionSetter interface defines the option setter methods for interface options.
type IfaceOptionSetter interface {
	// Bridge returns an option setter to set if the interface is a bridge.
	Bridge(bool) IfaceOption

	// Address returns an option setter to set IPv4 address.
	Address(*net.IPNet) IfaceOption

	// Address returns an option setter to set IPv6 address.
	AddressIPv6(*net.IPNet) IfaceOption

	// Master returns an option setter to set the master interface if any for this
	// interface. The master interface name should refer to the srcname of a
	// previously added interface of type bridge.
	Master(string) IfaceOption

	// Address returns an option setter to set interface routes.
	Routes([]*net.IPNet) IfaceOption
}

// Info represents all possible information that
// the driver wants to place in the sandbox which includes
// interfaces, routes and gateway
type Info interface {
	// The collection of Interface previously added with the AddInterface
	// method. Note that this doesn't incude network interfaces added in any
	// other way (such as the default loopback interface which are automatically
	// created on creation of a sandbox).
	Interfaces() []Interface

	// IPv4 gateway for the sandbox.
	Gateway() net.IP

	// IPv6 gateway for the sandbox.
	GatewayIPv6() net.IP

	// Additional static routes for the sandbox.  (Note that directly
	// connected routes are stored on the particular interface they refer to.)
	StaticRoutes() []*types.StaticRoute

	// TODO: Add ip tables etc.
}

// Interface represents the settings and identity of a network device. It is
// used as a return type for Network.Link, and it is common practice for the
// caller to use this information when moving interface SrcName from host
// namespace to DstName in a different net namespace with the appropriate
// network settings.
type Interface interface {
	// The name of the interface in the origin network namespace.
	SrcName() string

	// The name that will be assigned to the interface once moves inside a
	// network namespace. When the caller passes in a DstName, it is only
	// expected to pass a prefix. The name will modified with an appropriately
	// auto-generated suffix.
	DstName() string

	// IPv4 address for the interface.
	Address() *net.IPNet

	// IPv6 address for the interface.
	AddressIPv6() *net.IPNet

	// IP routes for the interface.
	Routes() []*net.IPNet

	// Bridge returns true if the interface is a bridge
	Bridge() bool

	// Master returns the srcname of the master interface for this interface.
	Master() string

	// Remove an interface from the sandbox by renaming to original name
	// and moving it out of the sandbox.
	Remove() error
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
