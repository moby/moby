package docker

import (
	"container/list"
	"fmt"
	"github.com/dotcloud/docker/gograph"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
)

var defaultDns = []string{"8.8.8.8", "8.8.4.4"}

type Capabilities struct {
	MemoryLimit            bool
	SwapLimit              bool
	IPv4ForwardingDisabled bool
}

type Runtime struct {
	repository     string
	containers     *list.List
	networkManager *NetworkManager
	graph          *Graph
	repositories   *TagStore
	idIndex        *utils.TruncIndex
	capabilities   *Capabilities
	kernelVersion  *utils.KernelVersionInfo
	volumes        *Graph
	srv            *Server
	config         *DaemonConfig
	containerGraph *gograph.Database
}

var sysInitPath string

func init() {
	sysInitPath = utils.SelfPath()
}

// List returns an array of all containers registered in the runtime.
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

// Get looks for a container by the specified ID or name, and returns it.
// If the container is not found, or if an error occurs, nil is returned.
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

// Exists returns a true if a container of the specified ID or name exists,
// false otherwise.
func (runtime *Runtime) Exists(id string) bool {
	return runtime.Get(id) != nil
}

func (runtime *Runtime) containerRoot(id string) string {
	return path.Join(runtime.repository, id)
}

// Load reads the contents of a container from disk
// This is typically done at startup.
func (runtime *Runtime) load(id string) (*Container, error) {
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
			if runtime.config.AutoRestart {
				utils.Debugf("Restarting")
				container.State.Ghost = false
				container.State.setStopped(0)
				hostConfig, _ := container.ReadHostConfig()
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
		hostConfig, _ := container.ReadHostConfig()
		container.allocateNetwork(hostConfig)
		go container.monitor()
	}
	return nil
}

func (runtime *Runtime) LogToDisk(src *utils.WriteBroadcaster, dst, stream string) error {
	log, err := os.OpenFile(dst, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	src.AddWriter(log, stream)
	return nil
}

// Destroy unregisters a container from the runtime and cleanly removes its contents from the filesystem.
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
	wheel := "-\\|/"
	if os.Getenv("DEBUG") == "" && os.Getenv("TEST") == "" {
		fmt.Printf("Loading containers:  ")
	}
	dir, err := ioutil.ReadDir(runtime.repository)
	if err != nil {
		return err
	}
	containers := []*Container{}
	for i, v := range dir {
		id := v.Name()
		container, err := runtime.load(id)
		if i%21 == 0 && os.Getenv("DEBUG") == "" && os.Getenv("TEST") == "" {
			fmt.Printf("\b%c", wheel[i%4])
		}
		if err != nil {
			utils.Debugf("Failed to load container %v: %v", id, err)
			continue
		}
		utils.Debugf("Loaded container %v", container.ID)
		containers = append(containers, container)
	}
	sortContainers(containers, func(i, j *Container) bool {
		ic, _ := i.ReadHostConfig()
		jc, _ := j.ReadHostConfig()

		if ic == nil || ic.Links == nil {
			return true
		}
		if jc == nil || jc.Links == nil {
			return false
		}
		return len(ic.Links) < len(jc.Links)
	})
	for _, container := range containers {
		if err := runtime.Register(container); err != nil {
			utils.Debugf("Failed to register container %s: %s", container.ID, err)
			continue
		}
	}
	if os.Getenv("DEBUG") == "" && os.Getenv("TEST") == "" {
		fmt.Printf("\bdone.\n")
	}

	return nil
}

// FIXME: comment please!
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

	content, err3 := ioutil.ReadFile("/proc/sys/net/ipv4/ip_forward")
	runtime.capabilities.IPv4ForwardingDisabled = err3 != nil || len(content) == 0 || content[0] != '1'
	if runtime.capabilities.IPv4ForwardingDisabled && !quiet {
		log.Printf("WARNING: IPv4 forwarding is disabled.")
	}
}

// Create creates a new container from the given configuration.
func (runtime *Runtime) Create(config *Config) (*Container, []string, error) {
	// Lookup image
	img, err := runtime.repositories.LookupImage(config.Image)
	if err != nil {
		return nil, nil, err
	}

	warnings := []string{}
	if img.Config != nil {
		if img.Config.PortSpecs != nil && warnings != nil {
			for _, p := range img.Config.PortSpecs {
				if strings.Contains(p, ":") {
					warnings = append(warnings, "This image expects private ports to be mapped to public ports on your host. "+
						"This has been deprecated and the public mappings will not be honored."+
						"Use -p to publish the ports.")
					break
				}
			}
		}
		MergeConfig(config, img.Config)
	}

	if len(config.Entrypoint) != 0 && config.Cmd == nil {
		config.Cmd = []string{}
	} else if config.Cmd == nil || len(config.Cmd) == 0 {
		return nil, nil, fmt.Errorf("No command specified")
	}

	// Generate id
	id := GenerateID()

	// Set the default enitity in the graph
	if _, err := runtime.containerGraph.Set(fmt.Sprintf("/%s", id), id); err != nil {
		return nil, nil, err
	}

	// Generate default hostname
	// FIXME: the lxc template no longer needs to set a default hostname
	if config.Hostname == "" {
		config.Hostname = id[:12]
	}

	var args []string
	var entrypoint string

	if len(config.Entrypoint) != 0 {
		entrypoint = config.Entrypoint[0]
		args = append(config.Entrypoint[1:], config.Cmd...)
	} else {
		entrypoint = config.Cmd[0]
		args = config.Cmd[1:]
	}

	container := &Container{
		// FIXME: we should generate the ID here instead of receiving it as an argument
		ID:              id,
		Created:         time.Now(),
		Path:            entrypoint,
		Args:            args, //FIXME: de-duplicate from config
		Config:          config,
		Image:           img.ID, // Always use the resolved image id
		NetworkSettings: &NetworkSettings{},
		// FIXME: do we need to store this in the container?
		SysInitPath: sysInitPath,
	}
	container.root = runtime.containerRoot(container.ID)
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return nil, nil, err
	}

	resolvConf, err := utils.GetResolvConf()
	if err != nil {
		return nil, nil, err
	}

	if len(config.Dns) == 0 && len(runtime.config.Dns) == 0 && utils.CheckLocalDns(resolvConf) {
		//"WARNING: Docker detected local DNS server on resolv.conf. Using default external servers: %v", defaultDns
		runtime.config.Dns = defaultDns
	}

	// If custom dns exists, then create a resolv.conf for the container
	if len(config.Dns) > 0 || len(runtime.config.Dns) > 0 {
		var dns []string
		if len(config.Dns) > 0 {
			dns = config.Dns
		} else {
			dns = runtime.config.Dns
		}
		container.ResolvConfPath = path.Join(container.root, "resolv.conf")
		f, err := os.Create(container.ResolvConfPath)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()
		for _, dns := range dns {
			if _, err := f.Write([]byte("nameserver " + dns + "\n")); err != nil {
				return nil, nil, err
			}
		}
	} else {
		container.ResolvConfPath = "/etc/resolv.conf"
	}

	// Step 2: save the container json
	if err := container.ToDisk(); err != nil {
		return nil, nil, err
	}

	// Step 3: if hostname, build hostname and hosts files
	container.HostnamePath = path.Join(container.root, "hostname")
	ioutil.WriteFile(container.HostnamePath, []byte(container.Config.Hostname+"\n"), 0644)

	hostsContent := []byte(`
127.0.0.1	localhost
::1		localhost ip6-localhost ip6-loopback
fe00::0		ip6-localnet
ff00::0		ip6-mcastprefix
ff02::1		ip6-allnodes
ff02::2		ip6-allrouters
`)

	container.HostsPath = path.Join(container.root, "hosts")

	if container.Config.Domainname != "" {
		hostsContent = append([]byte(fmt.Sprintf("::1\t\t%s.%s %s\n", container.Config.Hostname, container.Config.Domainname, container.Config.Hostname)), hostsContent...)
		hostsContent = append([]byte(fmt.Sprintf("127.0.0.1\t%s.%s %s\n", container.Config.Hostname, container.Config.Domainname, container.Config.Hostname)), hostsContent...)
	} else {
		hostsContent = append([]byte(fmt.Sprintf("::1\t\t%s\n", container.Config.Hostname)), hostsContent...)
		hostsContent = append([]byte(fmt.Sprintf("127.0.0.1\t%s\n", container.Config.Hostname)), hostsContent...)
	}

	ioutil.WriteFile(container.HostsPath, hostsContent, 0644)

	// Step 4: register the container
	if err := runtime.Register(container); err != nil {
		return nil, nil, err
	}
	return container, warnings, nil
}

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository
func (runtime *Runtime) Commit(container *Container, repository, tag, comment, author string, config *Config) (*Image, error) {
	// FIXME: freeze the container before copying it to avoid data corruption?
	// FIXME: this shouldn't be in commands.
	if err := container.EnsureMounted(); err != nil {
		return nil, err
	}

	rwTar, err := container.ExportRw()
	if err != nil {
		return nil, err
	}
	// Create a new image from the container's base layers + a new layer from container changes
	img, err := runtime.graph.Create(rwTar, container, comment, author, config)
	if err != nil {
		return nil, err
	}
	// Register the image if needed
	if repository != "" {
		if err := runtime.repositories.Set(repository, tag, img.ID, true); err != nil {
			return img, err
		}
	}
	return img, nil
}

func (runtime *Runtime) GetByName(name string) (*Container, error) {
	entity := runtime.containerGraph.Get(name)
	if entity == nil {
		return nil, fmt.Errorf("Could not find entity for %s", name)
	}
	container := runtime.Get(entity.ID())
	if container == nil {
		return nil, fmt.Errorf("Could not find container for entity id %s", entity.ID())
	}
	return container, nil
}

func (runtime *Runtime) Children(name string) (map[string]*Container, error) {
	children := make(map[string]*Container)

	err := runtime.containerGraph.Walk(name, func(p string, e *gograph.Entity) error {
		c := runtime.Get(e.ID())
		if c == nil {
			return fmt.Errorf("Could not get container for name %s and id %s", e.ID(), p)
		}
		children[p] = c
		return nil
	}, 0)

	if err != nil {
		return nil, err
	}
	return children, nil
}

func (runtime *Runtime) RenameLink(oldName, newName string) error {
	entity := runtime.containerGraph.Get(oldName)
	if entity == nil {
		return fmt.Errorf("Could not find entity for %s", oldName)
	}

	// This is not rename but adding a new link for the default name
	// Strip the leading '/'
	if entity.ID() == oldName[1:] {
		_, err := runtime.containerGraph.Set(newName, entity.ID())
		return err
	}
	return runtime.containerGraph.Rename(oldName, newName)
}

func (runtime *Runtime) Link(parentName, childName, alias string) error {
	parent := runtime.containerGraph.Get(parentName)
	if parent == nil {
		return fmt.Errorf("Could not get container for %s", parentName)
	}
	child := runtime.containerGraph.Get(childName)
	if child == nil {
		return fmt.Errorf("Could not get container for %s", childName)
	}
	cc := runtime.Get(child.ID())

	_, err := runtime.containerGraph.Set(path.Join(parentName, alias), cc.ID)
	return err
}

// FIXME: harmonize with NewGraph()
func NewRuntime(config *DaemonConfig) (*Runtime, error) {
	runtime, err := NewRuntimeFromDirectory(config)
	if err != nil {
		return nil, err
	}

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

func NewRuntimeFromDirectory(config *DaemonConfig) (*Runtime, error) {
	runtimeRepo := path.Join(config.GraphPath, "containers")

	if err := os.MkdirAll(runtimeRepo, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	g, err := NewGraph(path.Join(config.GraphPath, "graph"))
	if err != nil {
		return nil, err
	}
	volumes, err := NewGraph(path.Join(config.GraphPath, "volumes"))
	if err != nil {
		return nil, err
	}
	repositories, err := NewTagStore(path.Join(config.GraphPath, "repositories"), g)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Tag store: %s", err)
	}
	if config.BridgeIface == "" {
		config.BridgeIface = DefaultNetworkBridge
	}
	netManager, err := newNetworkManager(config)
	if err != nil {
		return nil, err
	}
	graph, err := gograph.NewDatabase("", "engine")
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		repository:     runtimeRepo,
		containers:     list.New(),
		networkManager: netManager,
		graph:          g,
		repositories:   repositories,
		idIndex:        utils.NewTruncIndex(),
		capabilities:   &Capabilities{},
		volumes:        volumes,
		config:         config,
		containerGraph: graph,
	}

	if err := runtime.restore(); err != nil {
		return nil, err
	}
	return runtime, nil
}

// History is a convenience type for storing a list of containers,
// ordered by creation date.
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
