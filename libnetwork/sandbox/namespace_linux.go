package sandbox

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// The networkNamespace type is the linux implementation of the Sandbox
// interface. It represents a linux network namespace, and moves an interface
// into it when called on method AddInterface or sets the gateway etc.
type networkNamespace struct {
	path  string
	sinfo *Info
}

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string) (Sandbox, error) {
	return createNetworkNamespace(key)
}

func createNetworkNamespace(path string) (Sandbox, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return nil, err
	}
	defer origns.Close()

	if err := createNamespaceFile(path); err != nil {
		return nil, err
	}

	defer netns.Set(origns)
	newns, err := netns.New()
	if err != nil {
		return nil, err
	}
	defer newns.Close()

	if err := loopbackUp(); err != nil {
		return nil, err
	}

	procNet := fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), syscall.Gettid())

	if err := syscall.Mount(procNet, path, "bind", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}

	interfaces := []*Interface{}
	sinfo := &Info{Interfaces: interfaces}
	return &networkNamespace{path: path, sinfo: sinfo}, nil
}

func createNamespaceFile(path string) (err error) {
	var f *os.File
	if f, err = os.Create(path); err == nil {
		f.Close()
	}
	return err
}

func loopbackUp() error {
	iface, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(iface)
}

func (n *networkNamespace) AddInterface(i *Interface) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return err
	}
	defer origns.Close()

	f, err := os.OpenFile(n.path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", n.path, err)
	}
	defer f.Close()

	// Find the network inteerface identified by the SrcName attribute.
	iface, err := netlink.LinkByName(i.SrcName)
	if err != nil {
		return err
	}

	// Move the network interface to the destination namespace.
	nsFD := f.Fd()
	if err := netlink.LinkSetNsFd(iface, int(nsFD)); err != nil {
		return err
	}

	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer netns.Set(origns)

	// Down the interface before configuring
	if err := netlink.LinkSetDown(iface); err != nil {
		return err
	}

	// Configure the interface now this is moved in the proper namespace.
	if err := configureInterface(iface, i); err != nil {
		return err
	}

	// Up the interface.
	if err := netlink.LinkSetUp(iface); err != nil {
		return err
	}

	n.sinfo.Interfaces = append(n.sinfo.Interfaces, i)
	return nil
}

func (n *networkNamespace) SetGateway(gw net.IP) error {
	err := setGatewayIP(n.path, gw)
	if err == nil {
		n.sinfo.Gateway = gw
	}

	return err
}

func (n *networkNamespace) SetGatewayIPv6(gw net.IP) error {
	if len(gw) == 0 {
		return nil
	}

	err := setGatewayIP(n.path, gw)
	if err == nil {
		n.sinfo.GatewayIPv6 = gw
	}

	return err
}

func (n *networkNamespace) Interfaces() []*Interface {
	return n.sinfo.Interfaces
}

func (n *networkNamespace) Key() string {
	return n.path
}

func (n *networkNamespace) Destroy() error {
	// Assuming no running process is executing in this network namespace,
	// unmounting is sufficient to destroy it.
	return syscall.Unmount(n.path, syscall.MNT_DETACH)
}
