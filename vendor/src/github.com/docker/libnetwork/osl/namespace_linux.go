package osl

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/ns"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const (
	prefix           = "/var/run/docker/netns"
	oslsandboxPrefix = "oslsandbox"
)

var (
	once             sync.Once
	garbagePathMap   = make(map[string]bool)
	gpmLock          sync.Mutex
	gpmWg            sync.WaitGroup
	gpmCleanupPeriod = 60 * time.Second
	gpmChan          = make(chan chan struct{})
	nsOnce           sync.Once
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
	nextIfIndex  int
	isDefault    bool
	dbIndex      uint64
	dbExists     bool
	sync.Mutex
}

func (n *networkNamespace) MarshalJSON() ([]byte, error) {
	nMap := make(map[string]interface{})
	nMap["path"] = n.path
	nMap["nextIfIndex"] = n.nextIfIndex
	nMap["isDefault"] = n.isDefault
	nMap["gw"] = n.gw.String()
	nMap["gw6"] = n.gwv6.String()
	nMap["staticRoutes"] = n.staticRoutes
	nMap["neighbors"] = n.neighbors
	nMap["iFaces"] = n.iFaces

	return json.Marshal(nMap)
}

func (n *networkNamespace) UnmarshalJSON(b []byte) (err error) {
	var epMap map[string]interface{}
	if err := json.Unmarshal(b, &epMap); err != nil {
		return err
	}

	n.path = epMap["path"].(string)
	n.nextIfIndex = int(epMap["nextIfIndex"].(float64))
	n.isDefault = epMap["isDefault"].(bool)

	var iFaces []nwIface
	if v, ok := epMap["iFaces"]; ok {
		is, err := json.Marshal(v)
		if err != nil {
			return err
		}
		json.Unmarshal(is, &iFaces)
	}
	var iFacesPtr []*nwIface
	for i, _ := range iFaces {
		iFacesPtr = append(iFacesPtr, &iFaces[i])
	}
	n.iFaces = iFacesPtr

	if v, ok := epMap["gw"]; ok {
		n.gw = net.ParseIP(v.(string))
	}
	if v, ok := epMap["gw6"]; ok {
		n.gwv6 = net.ParseIP(v.(string))
	}

	var staticRoutes []types.StaticRoute
	if v, ok := epMap["staticRoutes"]; ok {
		sr, err := json.Marshal(v)
		if err != nil {
			return err
		}
		json.Unmarshal(sr, &staticRoutes)
	}
	var staticRoutePtrs []*types.StaticRoute
	for i, _ := range staticRoutes {
		staticRoutePtrs = append(staticRoutePtrs, &staticRoutes[i])
	}
	n.staticRoutes = staticRoutePtrs

	var neighbors []neigh
	if v, ok := epMap["neighbors"]; ok {
		ns, err := json.Marshal(v)
		if err != nil {
			return err
		}
		json.Unmarshal(ns, &neighbors)
	}
	var neighborpts []*neigh
	for _, neigh := range neighbors {
		neighborpts = append(neighborpts, &neigh)
	}
	n.neighbors = neighborpts

	return nil
}

func (n *networkNamespace) CopyTo(o datastore.KVObject) error {
	// TODO
	dstN := o.(*networkNamespace)
	dstN.path = n.path
	dstN.gw = types.GetIPCopy(n.gw)
	dstN.gwv6 = types.GetIPCopy(n.gwv6)
	dstN.nextIfIndex = n.nextIfIndex
	dstN.isDefault = n.isDefault
	dstN.dbIndex = n.dbIndex
	dstN.dbExists = n.dbExists
	dstN.staticRoutes = make([]*types.StaticRoute, len(n.staticRoutes))
	copy(dstN.staticRoutes, n.staticRoutes)

	dstN.iFaces = make([]*nwIface, len(n.iFaces))
	copy(dstN.iFaces, n.iFaces)

	dstN.neighbors = make([]*neigh, len(n.neighbors))
	copy(dstN.neighbors, n.neighbors)
	return nil
}

func init() {
	reexec.Register("netns-create", reexecCreateNamespace)
}

func createBasePath() {
	err := os.MkdirAll(prefix, 0755)
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
	if len(containerID) < maxLen {
		maxLen = len(containerID)
	}

	return prefix + "/" + containerID[:maxLen]
}

// NewSandbox provides a new sandbox instance created in an os specific way
// provided a key which uniquely identifies the sandbox
func NewSandbox(key string, osCreate bool) (Sandbox, error) {
	err := createNetworkNamespace(key, osCreate)
	if err != nil {
		return nil, err
	}

	return &networkNamespace{path: key, isDefault: !osCreate}, nil
}

func NewNullSandbox(key string) Sandbox {
	return &networkNamespace{path: key}
}

func (n *networkNamespace) InterfaceOptions() IfaceOptionSetter {
	return n
}

func (n *networkNamespace) NeighborOptions() NeighborOptionSetter {
	return n
}

func mountNetworkNamespace(basePath string, lnPath string) error {
	if err := syscall.Mount(basePath, lnPath, "bind", syscall.MS_BIND, ""); err != nil {
		return err
	}

	if err := loopbackUp(); err != nil {
		return err
	}
	return nil
}

// GetSandboxForExternalKey returns sandbox object for the supplied path
func GetSandboxForExternalKey(basePath string, key string) (Sandbox, error) {
	var err error
	if err = createNamespaceFile(key); err != nil {
		return nil, err
	}
	n := &networkNamespace{path: basePath}
	n.InvokeFunc(func() {
		err = mountNetworkNamespace(basePath, key)
	})
	if err != nil {
		return nil, err
	}
	return &networkNamespace{path: key}, nil
}

func reexecCreateNamespace() {
	if len(os.Args) < 2 {
		log.Fatal("no namespace path provided")
	}
	if err := mountNetworkNamespace("/proc/self/ns/net", os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

func createNetworkNamespace(path string, osCreate bool) error {
	if err := createNamespaceFile(path); err != nil {
		return err
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"netns-create"}, path),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if osCreate {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWNET
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("namespace creation reexec command failed: %v", err)
	}

	return nil
}

func unmountNamespaceFile(path string) {
	if _, err := os.Stat(path); err == nil {
		syscall.Unmount(path, syscall.MNT_DETACH)
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

func loopbackUp() error {
	iface, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(iface)
}

func (n *networkNamespace) InvokeFunc(f func()) error {
	return nsInvoke(n.nsPath(), func(nsFD int) error { return nil }, func(callerFD int) error {
		f()
		return nil
	})
}

// InitOSContext initializes OS context while configuring network resources
func InitOSContext() func() {
	runtime.LockOSThread()
	nsOnce.Do(ns.Init)
	if err := ns.SetNamespace(); err != nil {
		log.Error(err)
	}

	return runtime.UnlockOSThread
}

func nsInvoke(path string, prefunc func(nsFD int) error, postfunc func(callerFD int) error) error {
	defer InitOSContext()()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", path, err)
	}
	defer f.Close()

	nsFD := f.Fd()

	// Invoked before the namespace switch happens but after the namespace file
	// handle is obtained.
	if err := prefunc(int(nsFD)); err != nil {
		return fmt.Errorf("failed in prefunc: %v", err)
	}

	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer ns.SetNamespace()

	// Invoked after the namespace switch.
	return postfunc(ns.ParseHandlerInt())
}

func (n *networkNamespace) nsPath() string {
	n.Lock()
	defer n.Unlock()

	return n.path
}

func (n *networkNamespace) Info() Info {
	return n
}

func (n *networkNamespace) GetKey() string {
	return n.path
}

func (n *networkNamespace) Destroy() error {
	// Assuming no running process is executing in this network namespace,
	// unmounting is sufficient to destroy it.
	if err := syscall.Unmount(n.path, syscall.MNT_DETACH); err != nil {
		return err
	}

	// Stash it into the garbage collection list
	addToGarbagePaths(n.path)
	return nil
}

func (n *networkNamespace) Key() []string {
	return []string{oslsandboxPrefix, n.path}
}

func (n *networkNamespace) KeyPrefix() []string {
	return []string{oslsandboxPrefix}
}

func (n *networkNamespace) Value() []byte {
	b, err := json.Marshal(n)
	if err != nil {
		return nil
	}
	return b
}

func (n *networkNamespace) SetValue(value []byte) error {
	return json.Unmarshal(value, n)
}

func (n *networkNamespace) Index() uint64 {
	n.Lock()
	defer n.Unlock()

	return n.dbIndex
}

func (n *networkNamespace) SetIndex(index uint64) {
	n.Lock()
	defer n.Unlock()

	n.dbIndex = index
	n.dbExists = true
}

func (n *networkNamespace) Exists() bool {
	n.Lock()
	defer n.Unlock()

	return n.dbExists
}

func (n *networkNamespace) Skip() bool {
	return false
}

func (n *networkNamespace) New() datastore.KVObject {
	return &networkNamespace{path: n.path}
}

func (n *networkNamespace) DataScope() string {
	return datastore.LocalScope
}

func (n *networkNamespace) UpdateToStore(store UpdateToStore) error {
	return store.UpdateToStore(n)
}

func (n *networkNamespace) GetFromStore(store datastore.DataStore) error {
	err := store.GetObject(datastore.Key(n.Key()...), n)
	if err != nil {
		return err
	}
	for i, _ := range n.iFaces {
		n.iFaces[i].ns = n
	}
	return nil
}

func (n *networkNamespace) DeleteFromStore(store DeleteFromStore) error {
	return store.DeleteFromStore(n)
}
