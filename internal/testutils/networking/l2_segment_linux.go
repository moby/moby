package networking

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"syscall"
	"testing"

	"github.com/docker/docker/libnetwork/testutils"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"gotest.tools/v3/assert"
)

// Netns represents a network namespace that might be named, and thus mounted under /run/netns.
type Netns struct {
	handle netns.NsHandle
	name   string
}

// CurrentNetns returns an unnamed Netns for the current network namespace this thread is living in. If it fails to
// get a handle for the current net namespace, it bails out.
func CurrentNetns(t *testing.T) Netns {
	t.Helper()

	curNs, err := netns.Get()
	assert.NilError(t, err)

	return Netns{handle: curNs}
}

// NewNamedNetns creates a new named Netns and then switch back to the original net namespace. It bails out if it fails.
func NewNamedNetns(t *testing.T, name string, reusePrevious bool) Netns {
	t.Helper()

	var err error
	newNs := Netns{name: name}
	newNs.handle, err = netns.GetFromName(newNs.name)
	if err != nil && !os.IsNotExist(err) {
		assert.NilError(t, err)
	}
	if err == nil {
		if reusePrevious {
			return newNs
		}
		t.Fatalf("Network namespace %s already exists whereas TEST_REUSE_L2SEGMENT is empty.", newNs.name)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origNs, err := netns.Get()
	assert.NilError(t, err)

	newNs.handle, err = netns.NewNamed(newNs.name)
	assert.NilError(t, err)

	err = netns.Set(origNs)
	assert.NilError(t, err)

	return newNs
}

// Set joins the current goroutine to this Netns. The caller has to make sure the OS thread is locked.
func (ns Netns) Set() error {
	if !ns.handle.IsOpen() {
		return nil
	}
	if err := netns.Set(ns.handle); err != nil {
		return fmt.Errorf("could not switch to netns %s: %w", ns.String(), err)
	}
	return nil
}

// InNetns executes the provided fn in the Netns and then switches back to the original netns. If it fails to restore
// the original netns, it marks the test as failed and stops its execution. For that reason, InNetns() should be run in
// the test goroutine.
func (ns Netns) InNetns(t *testing.T, fn func() error) error {
	t.Helper()

	orig := CurrentNetns(t)
	runtime.LockOSThread()
	defer func() {
		if err := netns.Set(orig.handle); err != nil {
			t.Fatalf("could not switch back to the original netns: %v", err)
		}
		runtime.UnlockOSThread()
	}()

	if err := ns.Set(); err != nil {
		t.Error(err)
	}

	return fn()
}

func (ns Netns) String() string {
	if ns.name == "" {
		return ns.handle.String()
	}
	return fmt.Sprintf("<path:%s>", ns.Path())
}

// Path returns an absolute path to the namespace mount point, if it's a named netns and it has not been deleted yet.
// Otherwise, it returns an empty string.
func (ns Netns) Path() string {
	if ns.name != "" {
		return "/run/netns/" + ns.name
	}
	return ""
}

// NetlinkHandle returns a *netlink.Handle created from Netns.
func (ns Netns) NetlinkHandle() (*netlink.Handle, error) {
	if !ns.handle.IsOpen() {
		return nil, errors.New("can't get a netlink handle from a closed netns")
	}
	return netlink.NewHandleAt(ns.handle)
}

// NsFd returns a netlink.NsFd that can be used in netlink.Link*() methods.
func (ns Netns) NsFd() netlink.NsFd {
	return netlink.NsFd(ns.handle)
}

// Close closes the fd associated to this Netns. If this is a named Netns, Destroy should be preferred.
func (ns Netns) Close() error {
	if ns.handle.IsOpen() {
		return ns.handle.Close()
	}

	return nil
}

// Destroy unmounts the netns and deletes the mount point, if it's a named Netns. It takes care of closing the netns
// handle beforehand if it's still open.
func (ns Netns) Destroy(logger testutils.Logger) {
	if err := ns.Close(); err != nil {
		logger.Logf("failed to close Netns: %v", err)
		return
	}

	if ns.name == "" {
		return
	}

	if err := syscall.Unmount(ns.Path(), 0); err != nil {
		logger.Logf("failed to unmount netns %s: %v", ns, err)
	}
	if err := os.Remove(ns.Path()); err != nil {
		logger.Logf("failed to remove netns mountpoint %s: %v", ns, err)
	}

	ns.name = ""
}

// L2Segment represents a switched L2 segment, where each host is actually a net namespace with a veth interface
// connected to a shared bridge. The bridge itself is running in its own net namespace to not be influenced by any
// routing, iptables rules and whatnot set in the host netns where the test runs.
type L2Segment struct {
	BridgeNs    Netns
	bridgeNlh   *netlink.Handle
	bridgeIface netlink.Bridge
	nextIP      netip.Addr
	mask        net.IPMask
}

func prefixMask(p netip.Prefix) net.IPMask {
	return net.CIDRMask(p.Bits(), p.Addr().BitLen())
}

func NewL2Segment(t *testing.T, brName string, subnet netip.Prefix, testID uint32, reusePrevious bool) (*L2Segment, error) {
	t.Helper()

	sgmt := &L2Segment{
		nextIP: subnet.Addr().Next(),
		mask:   prefixMask(subnet),
	}

	var err error
	sgmt.BridgeNs = NewNamedNetns(t, fmt.Sprintf("bridge-%8x", testID), reusePrevious)
	if err != nil {
		return sgmt, fmt.Errorf("could not create bridge netns: %w", err)
	}

	if sgmt.bridgeNlh, err = sgmt.BridgeNs.NetlinkHandle(); err != nil {
		return sgmt, fmt.Errorf("could not get netlink handle for netns %s: %w", sgmt.BridgeNs, err)
	}

	sgmt.bridgeIface = netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:  brName,
			Flags: net.FlagUp,
		},
	}

	// If reusePrevious is true, we don't need to create a new bridge interface if it already exists
	_, err = sgmt.bridgeNlh.LinkByName(brName)
	if err != nil {
		var linkNotFoundErr netlink.LinkNotFoundError
		if !errors.As(err, &linkNotFoundErr) {
			assert.NilError(t, err)
		}
		if err := sgmt.bridgeNlh.LinkAdd(&sgmt.bridgeIface); err != nil {
			return sgmt, fmt.Errorf("failed to create bridge interface: %w", err)
		}
	}
	if err == nil && !reusePrevious {
		t.Logf("Bridge %s already exists whereas TEST_REUSE_L2SEGMENT is empty.", sgmt.bridgeIface.Name)
		t.FailNow()
	}

	return sgmt, nil
}

func (sgmt *L2Segment) AddHost(host *L3Host, reusePrevious bool) error {
	host.IPAddr = sgmt.nextIP
	host.IPMask = sgmt.mask
	sgmt.nextIP = host.IPAddr.Next()

	err := host.createVeth(sgmt.bridgeNlh, sgmt.bridgeIface, reusePrevious)
	if err != nil {
		return fmt.Errorf("failed to create veth: %w", err)
	}

	return nil
}

func (sgmt *L2Segment) Destroy(logger testutils.Logger) {
	sgmt.BridgeNs.Destroy(logger)
}

// L3Host represents a host in a L2Segment. The host might be a dedicated net namespace or the net namespace where
// the test is executed.
type L3Host struct {
	// Ns is the Netns representing this host.
	Ns Netns
	// HostIfaceName is the name of the veth interface that resides on the host Ns.
	HostIfaceName string
	// BridgedIfaceName is the name of the veth interface associated to HostIfaceName. It might not reside in the host Ns.
	BridgedIfaceName string
	MACAddr          net.HardwareAddr
	IPAddr           netip.Addr
	IPMask           net.IPMask
}

// createVeth creates a veth pair. The interface being named after hostIfaceName is put into hostNs, and the one being
// named after bridgedIfaceName in BridgeNs. The L3Host.IPAddr is set on the host-side veth interface. The MAC address
// assigned by the kernel is set in L3Host.MACAddr. The veth pair should be ready to use once this function returns.
func (host *L3Host) createVeth(bridgeNlh *netlink.Handle, bridgeIface netlink.Bridge, reusePrevious bool) error {
	hostNlh, err := host.Ns.NetlinkHandle()
	if err != nil {
		return fmt.Errorf("could not create netlink handles in host ns %s: %w", host.Ns, err)
	}

	veth := netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:      host.HostIfaceName,
			Flags:     net.FlagUp,
			Namespace: host.Ns.NsFd(),
		},
		PeerName: host.BridgedIfaceName,
	}

	// If reusePrevious is true, we don't need to create a new veth pair if it already exists
	_, err = bridgeNlh.LinkByName(veth.PeerName)
	var linkNotFoundErr netlink.LinkNotFoundError
	if err != nil {
		if !errors.As(err, &linkNotFoundErr) {
			return err
		}
		if err := bridgeNlh.LinkAdd(&veth); err != nil {
			return fmt.Errorf("failed to create veth interface %s: %w", host.HostIfaceName, err)
		}
	}
	if err == nil {
		if !reusePrevious {
			return fmt.Errorf("veth interface %s already exists whereas TEST_REUSE_L2SEGMENT is empty", veth.Name)
		}

		// Assume previous test run successfully created and configured the veth pair.
		macAddr, err := getHardwardAddr(hostNlh, host.HostIfaceName)
		if err != nil {
			return fmt.Errorf("could not get the MAC address of host iface %s: %w", host.HostIfaceName, err)
		}
		host.MACAddr = macAddr

		return nil
	}

	bridgedIface, err := bridgeNlh.LinkByName(host.BridgedIfaceName)
	if err != nil {
		return fmt.Errorf("could not get peer veth interface %s: %w", host.BridgedIfaceName, err)
	}

	if err := bridgeNlh.LinkSetMaster(bridgedIface, &bridgeIface); err != nil {
		return fmt.Errorf("could not attach veth iface to bridge %s: %w", bridgeIface.Name, err)
	}
	if err := bridgeNlh.LinkSetUp(bridgedIface); err != nil {
		return fmt.Errorf("could not up peer iface %s: %w", host.BridgedIfaceName, err)
	}

	hostIface, err := hostNlh.LinkByName(host.HostIfaceName)
	if err != nil {
		return fmt.Errorf("could not get host iface %s from netns %s: %w", host.HostIfaceName, host.Ns, err)
	}
	if err := hostNlh.AddrAdd(hostIface, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   host.IPAddr.AsSlice(),
			Mask: host.IPMask,
		},
	}); err != nil {
		return fmt.Errorf("could not add address %s to %s@%s: %w", host.IPAddr, bridgedIface.Attrs().Name, host.HostIfaceName, err)
	}

	macAddr, err := getHardwardAddr(hostNlh, host.HostIfaceName)
	if err != nil {
		return fmt.Errorf("could not get the MAC address of host iface %s: %w", host.HostIfaceName, err)
	}
	host.MACAddr = macAddr

	return nil
}

func getHardwardAddr(nlh *netlink.Handle, iface string) (net.HardwareAddr, error) {
	link, err := nlh.LinkByName(iface)
	if err != nil {
		return net.HardwareAddr{}, err
	}
	return link.Attrs().HardwareAddr, nil
}

func (host *L3Host) Destroy(logger testutils.Logger) {
	host.Ns.Destroy(logger)
}
