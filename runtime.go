package docker

import (
	"container/list"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
)

type Capabilities struct {
	MemoryLimit bool
	SwapLimit   bool
}

type Runtime struct {
	root           string
	repository     string
	containers     *list.List
	networkManager *NetworkManager
	graph          *Graph
	repositories   *TagStore
	idIndex        *utils.TruncIndex
	capabilities   *Capabilities
	kernelVersion  *utils.KernelVersionInfo
	autoRestart    bool
	volumes        *Graph
	srv            *Server
	Dns            []string
}

var sysInitPath string

func init() {
	sysInitPath = utils.SelfPath()
}

func (runtime *Runtime) List() []*Container {
	containers := new(History)
	for e := runtime.containers.Front(); e != nil; e = e.Next() {
		containers.Add(e.Value.(*Container))
	}
	return *containers
}

func (runtime *Runtime) getContainerElement(id string) *list.Element {
	for e := runtime.containers.Front(); e != nil; e = e.Next() {
		container := e.Value.(*Container)
		if container.ID == id {
			return e
		}
	}
	return nil
}

func (runtime *Runtime) Get(name string) *Container {
	id, err := runtime.idIndex.Get(name)
	if err != nil {
		return nil
	}
	e := runtime.getContainerElement(id)
	if e == nil {
		return nil
	}
	return e.Value.(*Container)
}

func (runtime *Runtime) Exists(id string) bool {
	return runtime.Get(id) != nil
}

func (runtime *Runtime) containerRoot(id string) string {
	return path.Join(runtime.repository, id)
}

func (runtime *Runtime) Load(id string) (*Container, error) {
	container := &Container{root: runtime.containerRoot(id)}
	if err := container.FromDisk(); err != nil {
		return nil, err
	}
	if container.ID != id {
		return container, fmt.Errorf("Container %s is stored at %s", container.ID, id)
	}
	if container.State.Running {
		container.State.Ghost = true
	}
	if err := runtime.Register(container); err != nil {
		return nil, err
	}
	return container, nil
}

// Register makes a container object usable by the runtime as <container.ID>
func (runtime *Runtime) Register(container *Container) error {
	if container.runtime != nil || runtime.Exists(container.ID) {
		return fmt.Errorf("Container is already loaded")
	}
	if err := validateID(container.ID); err != nil {
		return err
	}

	// init the wait lock
	container.waitLock = make(chan struct{})

	container.runtime = runtime

	// Attach to stdout and stderr
	container.stderr = utils.NewWriteBroadcaster()
	container.stdout = utils.NewWriteBroadcaster()
	// Attach to stdin
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	} else {
		container.stdinPipe = utils.NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}
	// done
	runtime.containers.PushBack(container)
	runtime.idIndex.Add(container.ID)

	// When we actually restart, Start() do the monitoring.
	// However, when we simply 'reattach', we have to restart a monitor
	nomonitor := false

	// FIXME: if the container is supposed to be running but is not, auto restart it?
	//        if so, then we need to restart monitor and init a new lock
	// If the container is supposed to be running, make sure of it
	if container.State.Running {
		output, err := exec.Command("lxc-info", "-n", container.ID).CombinedOutput()
		if err != nil {
			return err
		}
		if !strings.Contains(string(output), "RUNNING") {
			utils.Debugf("Container %s was supposed to be running be is not.", container.ID)
			if runtime.autoRestart {
				utils.Debugf("Restarting")
				container.State.Ghost = false
				container.State.setStopped(0)
				// assume empty host config
				hostConfig := &HostConfig{}
				if err := container.Start(hostConfig); err != nil {
					return err
				}
				nomonitor = true
			} else {
				utils.Debugf("Marking as stopped")
				container.State.setStopped(-127)
				if err := container.ToDisk(); err != nil {
					return err
				}
			}
		}
	}

	// If the container is not running or just has been flagged not running
	// then close the wait lock chan (will be reset upon start)
	if !container.State.Running {
		close(container.waitLock)
	} else if !nomonitor {
		container.allocateNetwork()
		go container.monitor()
	}
	return nil
}

func (runtime *Runtime) LogToDisk(src *utils.WriteBroadcaster, dst string) error {
	log, err := os.OpenFile(dst, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	src.AddWriter(log)
	return nil
}

func (runtime *Runtime) Destroy(container *Container) error {
	if container == nil {
		return fmt.Errorf("The given container is <nil>")
	}

	element := runtime.getContainerElement(container.ID)
	if element == nil {
		return fmt.Errorf("Container %v not found - maybe it was already destroyed?", container.ID)
	}

	if err := container.Stop(3); err != nil {
		return err
	}
	if mounted, err := container.Mounted(); err != nil {
		return err
	} else if mounted {
		if err := container.Unmount(); err != nil {
			return fmt.Errorf("Unable to unmount container %v: %v", container.ID, err)
		}
	}
	// Deregister the container before removing its directory, to avoid race conditions
	runtime.idIndex.Delete(container.ID)
	runtime.containers.Remove(element)
	if err := os.RemoveAll(container.root); err != nil {
		return fmt.Errorf("Unable to remove filesystem for %v: %v", container.ID, err)
	}
	return nil
}

func (runtime *Runtime) restore() error {
	dir, err := ioutil.ReadDir(runtime.repository)
	if err != nil {
		return err
	}
	for _, v := range dir {
		id := v.Name()
		container, err := runtime.Load(id)
		if err != nil {
			utils.Debugf("Failed to load container %v: %v", id, err)
			continue
		}
		utils.Debugf("Loaded container %v", container.ID)
	}
	return nil
}

func (runtime *Runtime) UpdateCapabilities(quiet bool) {
	if cgroupMemoryMountpoint, err := utils.FindCgroupMountpoint("memory"); err != nil {
		if !quiet {
			log.Printf("WARNING: %s\n", err)
		}
	} else {
		_, err1 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.limit_in_bytes"))
		_, err2 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.soft_limit_in_bytes"))
		runtime.capabilities.MemoryLimit = err1 == nil && err2 == nil
		if !runtime.capabilities.MemoryLimit && !quiet {
			log.Printf("WARNING: Your kernel does not support cgroup memory limit.")
		}

		_, err = ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.memsw.limit_in_bytes"))
		runtime.capabilities.SwapLimit = err == nil
		if !runtime.capabilities.SwapLimit && !quiet {
			log.Printf("WARNING: Your kernel does not support cgroup swap limit.")
		}
	}
}

// FIXME: harmonize with NewGraph()
func NewRuntime(flGraphPath string, autoRestart bool, dns []string) (*Runtime, error) {
	runtime, err := NewRuntimeFromDirectory(flGraphPath, autoRestart)
	if err != nil {
		return nil, err
	}
	runtime.Dns = dns

	if k, err := utils.GetKernelVersion(); err != nil {
		log.Printf("WARNING: %s\n", err)
	} else {
		runtime.kernelVersion = k
		if utils.CompareKernelVersion(k, &utils.KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0}) < 0 {
			log.Printf("WARNING: You are running linux kernel version %s, which might be unstable running docker. Please upgrade your kernel to 3.8.0.", k.String())
		}
	}
	runtime.UpdateCapabilities(false)
	return runtime, nil
}

func NewRuntimeFromDirectory(root string, autoRestart bool) (*Runtime, error) {
	runtimeRepo := path.Join(root, "containers")

	if err := os.MkdirAll(runtimeRepo, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	g, err := NewGraph(path.Join(root, "graph"))
	if err != nil {
		return nil, err
	}
	volumes, err := NewGraph(path.Join(root, "volumes"))
	if err != nil {
		return nil, err
	}
	repositories, err := NewTagStore(path.Join(root, "repositories"), g)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Tag store: %s", err)
	}
	if NetworkBridgeIface == "" {
		NetworkBridgeIface = DefaultNetworkBridge
	}
	netManager, err := newNetworkManager(NetworkBridgeIface)
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{
		root:           root,
		repository:     runtimeRepo,
		containers:     list.New(),
		networkManager: netManager,
		graph:          g,
		repositories:   repositories,
		idIndex:        utils.NewTruncIndex(),
		capabilities:   &Capabilities{},
		autoRestart:    autoRestart,
		volumes:        volumes,
	}

	if err := runtime.restore(); err != nil {
		return nil, err
	}
	return runtime, nil
}

type History []*Container

func (history *History) Len() int {
	return len(*history)
}

func (history *History) Less(i, j int) bool {
	containers := *history
	return containers[j].When().Before(containers[i].When())
}

func (history *History) Swap(i, j int) {
	containers := *history
	tmp := containers[i]
	containers[i] = containers[j]
	containers[j] = tmp
}

func (history *History) Add(container *Container) {
	*history = append(*history, container)
	sort.Sort(history)
}
