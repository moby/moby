package sandbox

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const prefix = "/var/run/docker/netns"

var once sync.Once

// The networkNamespace type is the linux implementation of the Sandbox
// interface. It represents a linux network namespace, and moves an interface
// into it when called on method AddInterface or sets the gateway etc.
type networkNamespace struct {
	path  string
	sinfo *Info
}

func createBasePath() {
	err := os.MkdirAll(prefix, 0644)
	if err != nil && !os.IsExist(err) {
		panic("Could not create net namespace path directory")
	}
}

// GenerateKey generates a sandbox key based on the passed
// container id.
func GenerateKey(containerID string) string {
	maxLen := 12
	if len(containerID) < maxLen {
		maxLen = len(containerID)
	}

	return prefix + "/" + containerID[:maxLen]
}

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string, osCreate bool) (Sandbox, error) {
	info, err := createNetworkNamespace(key, osCreate)
	if err != nil {
		return nil, err
	}

	return &networkNamespace{path: key, sinfo: info}, nil
}

func createNetworkNamespace(path string, osCreate bool) (*Info, error) {
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

	if osCreate {
		defer netns.Set(origns)
		newns, err := netns.New()
		if err != nil {
			return nil, err
		}
		defer newns.Close()

		if err := loopbackUp(); err != nil {
			return nil, err
		}
	}

	procNet := fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), syscall.Gettid())

	if err := syscall.Mount(procNet, path, "bind", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}

	interfaces := []*Interface{}
	info := &Info{Interfaces: interfaces}
	return info, nil
}

func cleanupNamespaceFile(path string) {
	if _, err := os.Stat(path); err == nil {
		n := &networkNamespace{path: path}
		n.Destroy()
	}
}

func createNamespaceFile(path string) (err error) {
	var f *os.File

	once.Do(createBasePath)
	// cleanup namespace file if it already exists because of a previous ungraceful exit.
	cleanupNamespaceFile(path)
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

func (n *networkNamespace) RemoveInterface(i *Interface) error {
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

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer netns.Set(origns)

	// Find the network inteerface identified by the DstName attribute.
	iface, err := netlink.LinkByName(i.DstName)
	if err != nil {
		return err
	}

	// Down the interface before configuring
	if err := netlink.LinkSetDown(iface); err != nil {
		return err
	}

	err = netlink.LinkSetName(iface, i.SrcName)
	if err != nil {
		fmt.Println("LinkSetName failed: ", err)
		return err
	}

	// Move the network interface to caller namespace.
	if err := netlink.LinkSetNsFd(iface, int(origns)); err != nil {
		fmt.Println("LinkSetNsPid failed: ", err)
		return err
	}

	return nil
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

	// Find the network interface identified by the SrcName attribute.
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
	if len(gw) == 0 {
		return nil
	}

	err := programGateway(n.path, gw)
	if err == nil {
		n.sinfo.Gateway = gw
	}

	return err
}

func (n *networkNamespace) SetGatewayIPv6(gw net.IP) error {
	if len(gw) == 0 {
		return nil
	}

	err := programGateway(n.path, gw)
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
	if err := syscall.Unmount(n.path, syscall.MNT_DETACH); err != nil {
		return err
	}

	return os.Remove(n.path)
}
