package libnetwork

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// The networkNamespace type is the default implementation of the Namespace
// interface. It simply creates a new network namespace, and moves an interface
// into it when called on method AddInterface.
type networkNamespace struct {
	path       string
	interfaces []*Interface
}

func createNetworkNamespace(path string) (Namespace, error) {
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

	if err := syscall.Mount("/proc/self/ns/net", path, "bind", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}

	return &networkNamespace{path: path}, nil
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

	// Configure the interface now this is moved in the proper namespace.
	if err := configureInterface(iface, i); err != nil {
		return err
	}

	// Up the interface.
	if err := netlink.LinkSetUp(iface); err != nil {
		return err
	}

	n.interfaces = append(n.interfaces, i)
	return nil
}

func (n *networkNamespace) Interfaces() []*Interface {
	return n.interfaces
}

func (n *networkNamespace) Path() string {
	return n.path
}

func (n *networkNamespace) Destroy() error {
	// Assuming no running process is executing in this network namespace,
	// unmounting is sufficient to destroy it.
	return syscall.Unmount(n.path, syscall.MNT_DETACH)
}
