package osl

import (
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

	"github.com/docker/docker/internal/unshare"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl/kernel"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const defaultPrefix = "/var/run/docker"

func init() {
	// Lock main() to the initial thread to exclude the goroutines spawned
	// by func (*networkNamespace) InvokeFunc() or func setIPv6() below from
	// being scheduled onto that thread. Changes to the network namespace of
	// the initial thread alter /proc/self/ns/net, which would break any
	// code which (incorrectly) assumes that that file is the network
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
	prefix           = defaultPrefix
)

// The networkNamespace type is the linux implementation of the Sandbox
// interface. It represents a linux network namespace, and moves an interface
// into it when called on method AddInterface or sets the gateway etc.
type networkNamespace struct {
	path         string
	iFaces       []*nwIface
	gw           net.IP
	gwv6         net.IP
	staticRoutes []*types.StaticRoute
	neighbors    []*neigh
	nextIfIndex  map[string]int
	isDefault    bool
	nlHandle     *netlink.Handle
	loV6Enabled  bool
	sync.Mutex
}

// SetBasePath sets the base url prefix for the ns path
func SetBasePath(path string) {
	prefix = path
}

func basePath() string {
	return filepath.Join(prefix, "netns")
}

func createBasePath() {
	err := os.MkdirAll(basePath(), 0755)
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

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string, osCreate, isRestore bool) (Sandbox, error) {
	if !isRestore {
		err := createNetworkNamespace(key, osCreate)
		if err != nil {
			return nil, err
		}
	} else {
		once.Do(createBasePath)
	}

	n := &networkNamespace{path: key, isDefault: !osCreate, nextIfIndex: make(map[string]int)}

	sboxNs, err := netns.GetFromPath(n.path)
	if err != nil {
		return nil, fmt.Errorf("failed get network namespace %q: %v", n.path, err)
	}
	defer sboxNs.Close()

	n.nlHandle, err = netlink.NewHandleAt(sboxNs, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("failed to create a netlink handle: %v", err)
	}

	err = n.nlHandle.SetSocketTimeout(ns.NetlinkSocketsTimeout)
	if err != nil {
		logrus.Warnf("Failed to set the timeout on the sandbox netlink handle sockets: %v", err)
	}
	// In live-restore mode, IPV6 entries are getting cleaned up due to below code
	// We should retain IPV6 configurations in live-restore mode when Docker Daemon
	// comes back. It should work as it is on other cases
	// As starting point, disable IPv6 on all interfaces
	if !isRestore && !n.isDefault {
		err = setIPv6(n.path, "all", false)
		if err != nil {
			logrus.Warnf("Failed to disable IPv6 on all interfaces on network namespace %q: %v", n.path, err)
		}
	}

	if err = n.loopbackUp(); err != nil {
		n.nlHandle.Close()
		return nil, err
	}

	return n, nil
}

func (n *networkNamespace) InterfaceOptions() IfaceOptionSetter {
	return n
}

func (n *networkNamespace) NeighborOptions() NeighborOptionSetter {
	return n
}

func mountNetworkNamespace(basePath string, lnPath string) error {
	return syscall.Mount(basePath, lnPath, "bind", syscall.MS_BIND, "")
}

// GetSandboxForExternalKey returns sandbox object for the supplied path
func GetSandboxForExternalKey(basePath string, key string) (Sandbox, error) {
	if err := createNamespaceFile(key); err != nil {
		return nil, err
	}

	if err := mountNetworkNamespace(basePath, key); err != nil {
		return nil, err
	}
	n := &networkNamespace{path: key, nextIfIndex: make(map[string]int)}

	sboxNs, err := netns.GetFromPath(n.path)
	if err != nil {
		return nil, fmt.Errorf("failed get network namespace %q: %v", n.path, err)
	}
	defer sboxNs.Close()

	n.nlHandle, err = netlink.NewHandleAt(sboxNs, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("failed to create a netlink handle: %v", err)
	}

	err = n.nlHandle.SetSocketTimeout(ns.NetlinkSocketsTimeout)
	if err != nil {
		logrus.Warnf("Failed to set the timeout on the sandbox netlink handle sockets: %v", err)
	}

	// As starting point, disable IPv6 on all interfaces
	err = setIPv6(n.path, "all", false)
	if err != nil {
		logrus.Warnf("Failed to disable IPv6 on all interfaces on network namespace %q: %v", n.path, err)
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
	if _, err := os.Stat(path); err == nil {
		if err := syscall.Unmount(path, syscall.MNT_DETACH); err != nil && !errors.Is(err, unix.EINVAL) {
			logrus.WithError(err).Error("Error unmounting namespace file")
		}
	}
}

func createNamespaceFile(path string) (err error) {
	var f *os.File

	once.Do(createBasePath)
	// Remove it from garbage collection list if present
	removeFromGarbagePaths(path)

	// If the path is there unmount it first
	unmountNamespaceFile(path)

	// wait for garbage collection to complete if it is in progress
	// before trying to create the file.
	gpmWg.Wait()

	if f, err = os.Create(path); err == nil {
		f.Close()
	}

	return err
}

func (n *networkNamespace) loopbackUp() error {
	iface, err := n.nlHandle.LinkByName("lo")
	if err != nil {
		return err
	}
	return n.nlHandle.LinkSetUp(iface)
}

func (n *networkNamespace) GetLoopbackIfaceName() string {
	return "lo"
}

func (n *networkNamespace) AddAliasIP(ifName string, ip *net.IPNet) error {
	iface, err := n.nlHandle.LinkByName(ifName)
	if err != nil {
		return err
	}
	return n.nlHandle.AddrAdd(iface, &netlink.Addr{IPNet: ip})
}

func (n *networkNamespace) RemoveAliasIP(ifName string, ip *net.IPNet) error {
	iface, err := n.nlHandle.LinkByName(ifName)
	if err != nil {
		return err
	}
	return n.nlHandle.AddrDel(iface, &netlink.Addr{IPNet: ip})
}

func (n *networkNamespace) DisableARPForVIP(srcName string) (Err error) {
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
		if err := os.WriteFile(path, []byte{'1', '\n'}, 0644); err != nil {
			Err = fmt.Errorf("Failed to set %s to 1: %v", path, err)
			return
		}
		path = filepath.Join("/proc/sys/net/ipv4/conf", dstName, "arp_announce")
		if err := os.WriteFile(path, []byte{'2', '\n'}, 0644); err != nil {
			Err = fmt.Errorf("Failed to set %s to 2: %v", path, err)
			return
		}
	})
	if err != nil {
		return err
	}
	return
}

func (n *networkNamespace) InvokeFunc(f func()) error {
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
				logrus.WithError(err).Warn("failed to restore thread's network namespace")
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

func (n *networkNamespace) nsPath() string {
	n.Lock()
	defer n.Unlock()

	return n.path
}

func (n *networkNamespace) Info() Info {
	return n
}

func (n *networkNamespace) Key() string {
	return n.path
}

func (n *networkNamespace) Destroy() error {
	if n.nlHandle != nil {
		n.nlHandle.Close()
	}
	// Assuming no running process is executing in this network namespace,
	// unmounting is sufficient to destroy it.
	if err := syscall.Unmount(n.path, syscall.MNT_DETACH); err != nil {
		return err
	}

	// Stash it into the garbage collection list
	addToGarbagePaths(n.path)
	return nil
}

// Restore restore the network namespace
func (n *networkNamespace) Restore(ifsopt map[string][]IfaceOption, routes []*types.StaticRoute, gw net.IP, gw6 net.IP) error {
	// restore interfaces
	for name, opts := range ifsopt {
		if !strings.Contains(name, "+") {
			return fmt.Errorf("wrong iface name in restore osl sandbox interface: %s", name)
		}
		seps := strings.Split(name, "+")
		srcName := seps[0]
		dstPrefix := seps[1]
		i := &nwIface{srcName: srcName, dstName: dstPrefix, ns: n}
		i.processInterfaceOptions(opts...)
		if i.master != "" {
			i.dstMaster = n.findDst(i.master, true)
			if i.dstMaster == "" {
				return fmt.Errorf("could not find an appropriate master %q for %q",
					i.master, i.srcName)
			}
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
				addrs, err := n.nlHandle.AddrList(link, netlink.FAMILY_V4)
				if err != nil {
					return err
				}
				ifaceName := link.Attrs().Name
				if strings.HasPrefix(ifaceName, "vxlan") {
					if i.dstName == "vxlan" {
						i.dstName = ifaceName
						break
					}
				}
				// find the interface name by ip
				if i.address != nil {
					for _, addr := range addrs {
						if addr.IPNet.String() == i.address.String() {
							i.dstName = ifaceName
							break
						}
						continue
					}
					if i.dstName == ifaceName {
						break
					}
				}
				// This is to find the interface name of the pair in overlay sandbox
				if strings.HasPrefix(ifaceName, "veth") {
					if i.master != "" && i.dstName == "veth" {
						i.dstName = ifaceName
					}
				}
			}

			var index int
			indexStr := strings.TrimPrefix(i.dstName, dstPrefix)
			if indexStr != "" {
				index, err = strconv.Atoi(indexStr)
				if err != nil {
					return err
				}
			}
			index++
			n.Lock()
			if index > n.nextIfIndex[dstPrefix] {
				n.nextIfIndex[dstPrefix] = index
			}
			n.iFaces = append(n.iFaces, i)
			n.Unlock()
		}
	}

	// restore routes
	for _, r := range routes {
		n.Lock()
		n.staticRoutes = append(n.staticRoutes, r)
		n.Unlock()
	}

	// restore gateway
	if len(gw) > 0 {
		n.Lock()
		n.gw = gw
		n.Unlock()
	}

	if len(gw6) > 0 {
		n.Lock()
		n.gwv6 = gw6
		n.Unlock()
	}

	return nil
}

// Checks whether IPv6 needs to be enabled/disabled on the loopback interface
func (n *networkNamespace) checkLoV6() {
	var (
		enable = false
		action = "disable"
	)

	n.Lock()
	for _, iface := range n.iFaces {
		if iface.AddressIPv6() != nil {
			enable = true
			action = "enable"
			break
		}
	}
	n.Unlock()

	if n.loV6Enabled == enable {
		return
	}

	if err := setIPv6(n.path, "lo", enable); err != nil {
		logrus.Warnf("Failed to %s IPv6 on loopback interface on network namespace %q: %v", action, n.path, err)
	}

	n.loV6Enabled = enable
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
				logrus.WithError(err).Error("libnetwork: restoring thread network namespace failed")
				// The error is only fatal for the current thread. Keep this
				// goroutine locked to the thread to make the runtime replace it
				// with a clean thread once this goroutine returns.
			} else {
				runtime.UnlockOSThread()
			}
		}()

		var (
			action = "disable"
			value  = byte('1')
			path   = fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/disable_ipv6", iface)
		)

		if enable {
			action = "enable"
			value = '0'
		}

		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				logrus.WithError(err).Warn("Cannot configure IPv6 forwarding on container interface. Has IPv6 been disabled in this node's kernel?")
				return
			}
			errCh <- err
			return
		}

		if err = os.WriteFile(path, []byte{value, '\n'}, 0o644); err != nil {
			errCh <- fmt.Errorf("failed to %s IPv6 forwarding for container's interface %s: %w", action, iface, err)
			return
		}
	}()
	return <-errCh
}

// ApplyOSTweaks applies linux configs on the sandbox
func (n *networkNamespace) ApplyOSTweaks(types []SandboxType) {
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
