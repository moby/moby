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
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/internal/unshare"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl/kernel"
	"github.com/docker/docker/libnetwork/types"
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
	once             sync.Once
	garbagePathMap   = make(map[string]bool)
	gpmLock          sync.Mutex
	gpmWg            sync.WaitGroup
	gpmCleanupPeriod = 60 * time.Second
	gpmChan          = make(chan chan struct{})
	netnsBasePath    = filepath.Join(defaultPrefix, "netns")
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

	// Start the garbage collection go routine
	go removeUnusedPaths()
}

func removeUnusedPaths() {
	gpmLock.Lock()
	period := gpmCleanupPeriod
	gpmLock.Unlock()

	ticker := time.NewTicker(period)
	for {
		var (
			gc   chan struct{}
			gcOk bool
		)

		select {
		case <-ticker.C:
		case gc, gcOk = <-gpmChan:
		}

		gpmLock.Lock()
		pathList := make([]string, 0, len(garbagePathMap))
		for path := range garbagePathMap {
			pathList = append(pathList, path)
		}
		garbagePathMap = make(map[string]bool)
		gpmWg.Add(1)
		gpmLock.Unlock()

		for _, path := range pathList {
			os.Remove(path)
		}

		gpmWg.Done()
		if gcOk {
			close(gc)
		}
	}
}

func addToGarbagePaths(path string) {
	gpmLock.Lock()
	garbagePathMap[path] = true
	gpmLock.Unlock()
}

func removeFromGarbagePaths(path string) {
	gpmLock.Lock()
	delete(garbagePathMap, path)
	gpmLock.Unlock()
}

// GC triggers garbage collection of namespace path right away
// and waits for it.
func GC() {
	gpmLock.Lock()
	if len(garbagePathMap) == 0 {
		// No need for GC if map is empty
		gpmLock.Unlock()
		return
	}
	gpmLock.Unlock()

	// if content exists in the garbage paths
	// we can trigger GC to run, providing a
	// channel to be notified on completion
	waitGC := make(chan struct{})
	gpmChan <- waitGC
	// wait for GC completion
	<-waitGC
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
			if strings.HasSuffix(id, containerID[:maxLen-1]) {
				indexStr = strings.TrimSuffix(id, containerID[:maxLen-1])
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

	n := &Namespace{path: key, isDefault: !osCreate, nextIfIndex: make(map[string]int)}

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
	n := &Namespace{path: key, nextIfIndex: make(map[string]int)}

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
	// Remove it from garbage collection list if present
	removeFromGarbagePaths(path)

	// If the path is there unmount it first
	unmountNamespaceFile(path)

	// wait for garbage collection to complete if it is in progress
	// before trying to create the file.
	//
	// TODO(aker): This garbage-collection was for a kernel bug in kernels 3.18-4.0.1: is this still needed on current kernels (and on kernel 3.10)? see https://github.com/moby/moby/pull/46315/commits/c0a6beba8e61d4019e1806d5241ba22007072ca2#r1331327103
	gpmWg.Wait()

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
	path                string
	iFaces              []*Interface
	gw                  net.IP
	gwv6                net.IP
	staticRoutes        []*types.StaticRoute
	neighbors           []*neigh
	nextIfIndex         map[string]int
	isDefault           bool
	ipv6LoEnabledOnce   sync.Once
	ipv6LoEnabledCached bool
	nlHandle            nlwrap.Handle
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
func (n *Namespace) DisableARPForVIP(srcName string) (Err error) {
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
			Err = fmt.Errorf("Failed to set %s to 1: %v", path, err)
			return
		}
		path = filepath.Join("/proc/sys/net/ipv4/conf", dstName, "arp_announce")
		if err := os.WriteFile(path, []byte{'2', '\n'}, 0o644); err != nil {
			Err = fmt.Errorf("Failed to set %s to 2: %v", path, err)
			return
		}
	})
	if err != nil {
		return err
	}
	return
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

	// Stash it into the garbage collection list
	addToGarbagePaths(n.path)
	return nil
}

// Restore restores the network namespace.
func (n *Namespace) Restore(interfaces map[Iface][]IfaceOption, routes []*types.StaticRoute, gw net.IP, gw6 net.IP) error {
	// restore interfaces
	for iface, opts := range interfaces {
		i, err := newInterface(n, iface.SrcName, iface.DstPrefix, opts...)
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
				if i.dstName == "vxlan" && strings.HasPrefix(ifaceName, "vxlan") {
					i.dstName = ifaceName
					break
				}
				// find the interface name by ip
				if i.address != nil {
					addresses, err := n.nlHandle.AddrList(link, netlink.FAMILY_V4)
					if err != nil {
						return err
					}
					for _, addr := range addresses {
						if addr.IPNet.String() == i.address.String() {
							i.dstName = ifaceName
							break
						}
					}
					if i.dstName == ifaceName {
						break
					}
				}
				// This is to find the interface name of the pair in overlay sandbox
				if i.master != "" && i.dstName == "veth" && strings.HasPrefix(ifaceName, "veth") {
					i.dstName = ifaceName
				}
			}

			var index int
			if idx := strings.TrimPrefix(i.dstName, iface.DstPrefix); idx != "" {
				index, err = strconv.Atoi(idx)
				if err != nil {
					return fmt.Errorf("failed to restore interface in network namespace %q: invalid dstName for interface: %s: %v", n.path, i.dstName, err)
				}
			}
			index++
			n.mu.Lock()
			if index > n.nextIfIndex[iface.DstPrefix] {
				n.nextIfIndex[iface.DstPrefix] = index
			}
			n.iFaces = append(n.iFaces, i)
			n.mu.Unlock()
		}
	}

	// restore routes and gateways
	n.mu.Lock()
	n.staticRoutes = append(n.staticRoutes, routes...)
	if len(gw) > 0 {
		n.gw = gw
	}
	if len(gw6) > 0 {
		n.gwv6 = gw6
	}
	n.mu.Unlock()
	return nil
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
