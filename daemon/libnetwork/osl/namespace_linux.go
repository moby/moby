package osl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/unshare"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/daemon/libnetwork/osl/kernel"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const defaultPrefix = "/var/run/docker"

func init() {
	// Lock main() to the initial thread to exclude the goroutines spawned
	// by func (*Namespace) InvokeFunc() or func setIPv6() below from
	// being scheduled onto that thread. Changes to the network namespace of
	// the initial thread alter /proc/self/ns/net, which would break any
	// code which (incorrectly) assumes that the file is the network
	// namespace for the thread it is currently executing on.
	runtime.LockOSThread()
}

var (
	once          sync.Once
	netnsBasePath = filepath.Join(defaultPrefix, "netns")
)

// SetBasePath sets the base url prefix for the ns path
func SetBasePath(path string) {
	netnsBasePath = filepath.Join(path, "netns")
}

func basePath() string {
	return netnsBasePath
}

func createBasePath() {
	err := os.MkdirAll(basePath(), 0o755)
	if err != nil {
		panic("Could not create net namespace path directory")
	}
}

// GenerateKey generates a sandbox key based on the passed
// container id.
func GenerateKey(containerID string) string {
	maxLen := 12
	// Read sandbox key from host for overlay
	if strings.HasPrefix(containerID, "-") {
		var (
			index    int
			indexStr string
			tmpkey   string
		)
		dir, err := os.ReadDir(basePath())
		if err != nil {
			return ""
		}

		for _, v := range dir {
			id := v.Name()
			if before, ok := strings.CutSuffix(id, containerID[:maxLen-1]); ok {
				indexStr = before
				tmpindex, err := strconv.Atoi(indexStr)
				if err != nil {
					return ""
				}
				if tmpindex > index {
					index = tmpindex
					tmpkey = id
				}
			}
		}
		containerID = tmpkey
		if containerID == "" {
			return ""
		}
	}

	if len(containerID) < maxLen {
		maxLen = len(containerID)
	}

	return basePath() + "/" + containerID[:maxLen]
}

// NewSandbox provides a new Namespace instance created in an os specific way
// provided a key which uniquely identifies the sandbox.
func NewSandbox(key string, osCreate, isRestore bool) (*Namespace, error) {
	if !isRestore {
		err := createNetworkNamespace(key, osCreate)
		if err != nil {
			return nil, err
		}
	} else {
		once.Do(createBasePath)
	}

	n := &Namespace{path: key, isDefault: !osCreate}

	sboxNs, err := netns.GetFromPath(n.path)
	if err != nil {
		return nil, fmt.Errorf("failed get network namespace %q: %v", n.path, err)
	}
	defer sboxNs.Close()

	n.nlHandle, err = nlwrap.NewHandleAt(sboxNs, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("failed to create a netlink handle: %v", err)
	}

	err = n.nlHandle.SetSocketTimeout(ns.NetlinkSocketsTimeout)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to set the timeout on the sandbox netlink handle sockets: %v", err)
	}

	if err = n.loopbackUp(); err != nil {
		n.nlHandle.Close()
		return nil, err
	}

	return n, nil
}

func mountNetworkNamespace(basePath string, lnPath string) error {
	err := syscall.Mount(basePath, lnPath, "bind", syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("bind-mount %s -> %s: %w", basePath, lnPath, err)
	}
	return nil
}

// GetSandboxForExternalKey returns sandbox object for the supplied path
func GetSandboxForExternalKey(basePath string, key string) (*Namespace, error) {
	if err := createNamespaceFile(key); err != nil {
		return nil, err
	}

	if err := mountNetworkNamespace(basePath, key); err != nil {
		return nil, err
	}
	n := &Namespace{path: key}

	sboxNs, err := netns.GetFromPath(n.path)
	if err != nil {
		return nil, fmt.Errorf("failed get network namespace %q: %v", n.path, err)
	}
	defer sboxNs.Close()

	n.nlHandle, err = nlwrap.NewHandleAt(sboxNs, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("failed to create a netlink handle: %v", err)
	}

	err = n.nlHandle.SetSocketTimeout(ns.NetlinkSocketsTimeout)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to set the timeout on the sandbox netlink handle sockets: %v", err)
	}

	if err = n.loopbackUp(); err != nil {
		n.nlHandle.Close()
		return nil, err
	}

	return n, nil
}

func createNetworkNamespace(path string, osCreate bool) error {
	if err := createNamespaceFile(path); err != nil {
		return err
	}

	do := func() error {
		return mountNetworkNamespace(fmt.Sprintf("/proc/self/task/%d/ns/net", unix.Gettid()), path)
	}
	if osCreate {
		return unshare.Go(unix.CLONE_NEWNET, do, nil)
	}
	return do()
}

func unmountNamespaceFile(path string) {
	if _, err := os.Stat(path); err != nil {
		// ignore when we cannot stat the path
		return
	}
	if err := syscall.Unmount(path, syscall.MNT_DETACH); err != nil && !errors.Is(err, unix.EINVAL) {
		log.G(context.TODO()).WithError(err).Error("Error unmounting namespace file")
	}
}

func createNamespaceFile(path string) error {
	once.Do(createBasePath)

	// If the path is there unmount it first
	unmountNamespaceFile(path)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_ = f.Close()
	return nil
}

// Namespace represents a network sandbox. It represents a Linux network
// namespace, and moves an interface into it when called on method AddInterface
// or sets the gateway etc. It holds a list of Interfaces, routes etc., and more
// can be added dynamically.
type Namespace struct {
	path                string // path is the absolute path to the network namespace. It is safe to access it concurrently.
	iFaces              []*Interface
	gw                  net.IP
	gwv6                net.IP
	defRoute4SrcName    string
	defRoute6SrcName    string
	staticRoutes        []*types.StaticRoute
	isDefault           bool // isDefault is true when Namespace represents the host network namespace. It is safe to access it concurrently.
	ipv6LoEnabledOnce   sync.Once
	ipv6LoEnabledCached bool
	nlHandle            nlwrap.Handle // nlHandle is the netlink handle for the network namespace. It is safe to access it concurrently.
	mu                  sync.Mutex
}

// Interfaces returns the collection of Interface previously added with the AddInterface
// method. Note that this doesn't include network interfaces added in any
// other way (such as the default loopback interface which is automatically
// created on creation of a sandbox).
func (n *Namespace) Interfaces() []*Interface {
	ifaces := make([]*Interface, len(n.iFaces))
	copy(ifaces, n.iFaces)
	return ifaces
}

// InterfaceBySrcName returns a pointer to the Interface with a matching srcName, else nil.
func (n *Namespace) InterfaceBySrcName(srcName string) *Interface {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, iface := range n.iFaces {
		if iface.srcName == srcName {
			return iface
		}
	}
	return nil
}

func (n *Namespace) loopbackUp() error {
	iface, err := n.nlHandle.LinkByName("lo")
	if err != nil {
		return err
	}
	return n.nlHandle.LinkSetUp(iface)
}

// GetLoopbackIfaceName returns the name of the loopback interface
func (n *Namespace) GetLoopbackIfaceName() string {
	return "lo"
}

// AddAliasIP adds the passed IP address to the named interface
func (n *Namespace) AddAliasIP(ifName string, ip *net.IPNet) error {
	iface, err := n.nlHandle.LinkByName(ifName)
	if err != nil {
		return err
	}
	return n.nlHandle.AddrAdd(iface, &netlink.Addr{IPNet: ip})
}

// RemoveAliasIP removes the passed IP address from the named interface
func (n *Namespace) RemoveAliasIP(ifName string, ip *net.IPNet) error {
	iface, err := n.nlHandle.LinkByName(ifName)
	if err != nil {
		return err
	}
	return n.nlHandle.AddrDel(iface, &netlink.Addr{IPNet: ip})
}

// DisableARPForVIP disables ARP replies and requests for VIP addresses
// on a particular interface.
func (n *Namespace) DisableARPForVIP(srcName string) (retErr error) {
	dstName := ""
	for _, i := range n.Interfaces() {
		if i.SrcName() == srcName {
			dstName = i.DstName()
			break
		}
	}
	if dstName == "" {
		return fmt.Errorf("failed to find interface %s in sandbox", srcName)
	}

	err := n.InvokeFunc(func() {
		path := filepath.Join("/proc/sys/net/ipv4/conf", dstName, "arp_ignore")
		if err := os.WriteFile(path, []byte{'1', '\n'}, 0o644); err != nil {
			retErr = fmt.Errorf("Failed to set %s to 1: %v", path, err)
			return
		}
		path = filepath.Join("/proc/sys/net/ipv4/conf", dstName, "arp_announce")
		if err := os.WriteFile(path, []byte{'2', '\n'}, 0o644); err != nil {
			retErr = fmt.Errorf("Failed to set %s to 2: %v", path, err)
			return
		}
	})
	if err != nil {
		return err
	}
	return retErr
}

// InvokeFunc invoke a function in the network namespace.
func (n *Namespace) InvokeFunc(f func()) error {
	path := n.nsPath()
	newNS, err := netns.GetFromPath(path)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %w", path, err)
	}
	defer newNS.Close()

	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		// InvokeFunc() could have been called from a goroutine with
		// tampered thread state, e.g. from another InvokeFunc()
		// callback. The outer goroutine's thread state cannot be
		// trusted.
		origNS, err := netns.Get()
		if err != nil {
			runtime.UnlockOSThread()
			done <- fmt.Errorf("failed to get original network namespace: %w", err)
			return
		}
		defer origNS.Close()

		if err := netns.Set(newNS); err != nil {
			runtime.UnlockOSThread()
			done <- err
			return
		}
		defer func() {
			close(done)
			if err := netns.Set(origNS); err != nil {
				log.G(context.TODO()).WithError(err).Warn("failed to restore thread's network namespace")
				// Recover from the error by leaving this goroutine locked to
				// the thread. The runtime will terminate the thread and replace
				// it with a clean one when this goroutine returns.
			} else {
				runtime.UnlockOSThread()
			}
		}()
		f()
	}()
	return <-done
}

func (n *Namespace) nsPath() string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.path
}

// Key returns the path where the network namespace is mounted.
func (n *Namespace) Key() string {
	return n.path
}

// Destroy destroys the sandbox.
func (n *Namespace) Destroy() error {
	n.nlHandle.Handle.Close()
	// Assuming no running process is executing in this network namespace,
	// unmounting is sufficient to destroy it.
	if err := syscall.Unmount(n.path, syscall.MNT_DETACH); err != nil {
		return err
	}

	// Remove the path where the netns was mounted
	if err := os.Remove(n.path); err != nil {
		log.G(context.TODO()).WithError(err).Error("error removing namespace file")
	}
	return nil
}

// RestoreInterfaces restores the network namespace's interfaces.
func (n *Namespace) RestoreInterfaces(interfaces map[Iface][]IfaceOption) error {
	// restore interfaces
	for iface, opts := range interfaces {
		i, err := newInterface(n, iface.SrcName, iface.DstPrefix, iface.DstName, opts...)
		if err != nil {
			return err
		}
		if n.isDefault {
			i.dstName = i.srcName
		} else {
			links, err := n.nlHandle.LinkList()
			if err != nil {
				return fmt.Errorf("failed to retrieve list of links in network namespace %q during restore", n.path)
			}
			// due to the docker network connect/disconnect, so the dstName should
			// restore from the namespace
			for _, link := range links {
				ifaceName := link.Attrs().Name
				if i.dstPrefix == "vxlan" && strings.HasPrefix(ifaceName, "vxlan") {
					i.dstName = ifaceName
					break
				}
				// find the interface name by ip
				findIfname := func(needle *net.IPNet, haystack []netlink.Addr) (string, bool) {
					for _, addr := range haystack {
						if addr.IPNet.String() == needle.String() {
							return ifaceName, true
						}
					}
					return "", false
				}
				if i.address != nil {
					addresses, err := n.nlHandle.AddrList(link, netlink.FAMILY_V4)
					if err != nil {
						return err
					}
					if name, found := findIfname(i.address, addresses); found {
						i.dstName = name
						break
					}
				}
				if i.addressIPv6 != nil {
					addresses, err := n.nlHandle.AddrList(link, netlink.FAMILY_V6)
					if err != nil {
						return err
					}
					if name, found := findIfname(i.address, addresses); found {
						i.dstName = name
						break
					}
				}
				// This is to find the interface name of the pair in overlay sandbox
				if i.master != "" && i.dstPrefix == "veth" && strings.HasPrefix(ifaceName, "veth") {
					i.dstName = ifaceName
				}
			}

			n.mu.Lock()
			n.iFaces = append(n.iFaces, i)
			n.mu.Unlock()
		}
	}
	return nil
}

func (n *Namespace) RestoreRoutes(routes []*types.StaticRoute) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.staticRoutes = append(n.staticRoutes, routes...)
}

func (n *Namespace) RestoreGateway(ipv4 bool, gw net.IP, srcName string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if gw == nil {
		// There's no gateway address, so the default route is bound to the interface.
		if ipv4 {
			n.defRoute4SrcName = srcName
		} else {
			n.defRoute6SrcName = srcName
		}
		return
	}

	if ipv4 {
		n.gw = gw
	} else {
		n.gwv6 = gw
	}
}

// IPv6LoEnabled returns true if the loopback interface had an IPv6 address when
// last checked. It's always checked on the first call, and by RefreshIPv6LoEnabled.
// ('::1' is assigned by the kernel if IPv6 is enabled.)
func (n *Namespace) IPv6LoEnabled() bool {
	n.ipv6LoEnabledOnce.Do(func() {
		n.RefreshIPv6LoEnabled()
	})
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.ipv6LoEnabledCached
}

// RefreshIPv6LoEnabled refreshes the cached result returned by IPv6LoEnabled.
func (n *Namespace) RefreshIPv6LoEnabled() {
	n.mu.Lock()
	defer n.mu.Unlock()
	// If anything goes wrong, assume no-IPv6.
	n.ipv6LoEnabledCached = false
	iface, err := n.nlHandle.LinkByName("lo")
	if err != nil {
		log.G(context.TODO()).WithError(err).Warn("Unable to find 'lo' to determine IPv6 support")
		return
	}
	addrs, err := n.nlHandle.AddrList(iface, nl.FAMILY_V6)
	if err != nil {
		log.G(context.TODO()).WithError(err).Warn("Unable to get 'lo' addresses to determine IPv6 support")
		return
	}
	n.ipv6LoEnabledCached = len(addrs) > 0
}

// ApplyOSTweaks applies operating system specific knobs on the sandbox.
func (n *Namespace) ApplyOSTweaks(types []SandboxType) {
	for _, t := range types {
		switch t {
		case SandboxTypeLoadBalancer, SandboxTypeIngress:
			kernel.ApplyOSTweaks(map[string]*kernel.OSValue{
				// disables any special handling on port reuse of existing IPVS connection table entries
				// more info: https://github.com/torvalds/linux/blame/v5.15/Documentation/networking/ipvs-sysctl.rst#L32
				"net.ipv4.vs.conn_reuse_mode": {Value: "0", CheckFn: nil},
				// expires connection from the IPVS connection table when the backend is not available
				// more info: https://github.com/torvalds/linux/blame/v5.15/Documentation/networking/ipvs-sysctl.rst#L133
				"net.ipv4.vs.expire_nodest_conn": {Value: "1", CheckFn: nil},
				// expires persistent connections to destination servers with weights set to 0
				// more info: https://github.com/torvalds/linux/blame/v5.15/Documentation/networking/ipvs-sysctl.rst#L151
				"net.ipv4.vs.expire_quiescent_template": {Value: "1", CheckFn: nil},
			})
		}
	}
}

func setIPv6(nspath, iface string, enable bool) error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		namespace, err := netns.GetFromPath(nspath)
		if err != nil {
			errCh <- fmt.Errorf("failed get network namespace %q: %w", nspath, err)
			return
		}
		defer namespace.Close()

		runtime.LockOSThread()

		origNS, err := netns.Get()
		if err != nil {
			runtime.UnlockOSThread()
			errCh <- fmt.Errorf("failed to get current network namespace: %w", err)
			return
		}
		defer origNS.Close()

		if err = netns.Set(namespace); err != nil {
			runtime.UnlockOSThread()
			errCh <- fmt.Errorf("setting into container netns %q failed: %w", nspath, err)
			return
		}
		defer func() {
			if err := netns.Set(origNS); err != nil {
				log.G(context.TODO()).WithError(err).Error("libnetwork: restoring thread network namespace failed")
				// The error is only fatal for the current thread. Keep this
				// goroutine locked to the thread to make the runtime replace it
				// with a clean thread once this goroutine returns.
			} else {
				runtime.UnlockOSThread()
			}
		}()

		path := "/proc/sys/net/ipv6/conf/" + iface + "/disable_ipv6"
		value := byte('1')
		if enable {
			value = '0'
		}

		if curVal, err := os.ReadFile(path); err != nil {
			if os.IsNotExist(err) {
				if enable {
					log.G(context.TODO()).WithError(err).Warn("Cannot enable IPv6 on container interface. Has IPv6 been disabled in this node's kernel?")
				} else {
					log.G(context.TODO()).WithError(err).Debug("Not disabling IPv6 on container interface. Has IPv6 been disabled in this node's kernel?")
				}
				return
			}
			errCh <- err
			return
		} else if len(curVal) > 0 && curVal[0] == value {
			// Nothing to do, the setting is already correct.
			return
		}

		if err = os.WriteFile(path, []byte{value, '\n'}, 0o644); err != nil || os.Getenv("DOCKER_TEST_RO_DISABLE_IPV6") != "" {
			logger := log.G(context.TODO()).WithFields(log.Fields{
				"error":     err,
				"interface": iface,
			})
			if enable {
				// The user asked for IPv6 on the interface, and we can't give it to them.
				// But, in line with the IsNotExist case above, just log.
				logger.Warn("Cannot enable IPv6 on container interface, continuing.")
			} else {
				logger.Error("Cannot disable IPv6 on container interface.")
				errCh <- errors.New("failed to disable IPv6 on container's interface " + iface)
			}
			return
		}
	}()
	return <-errCh
}
