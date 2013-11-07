package docker

import (
	_ "code.google.com/p/gosqlite/sqlite3"
	"container/list"
	"database/sql"
	"fmt"
	"github.com/dotcloud/docker/gograph"
	"github.com/dotcloud/docker/utils"
	"github.com/dotcloud/docker/graphdriver"
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
	volumes        *Graph
	srv            *Server
	config         *DaemonConfig
	containerGraph *gograph.Database
	driver         graphdriver.Driver
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
	if c, _ := runtime.GetByName(name); c != nil {
		return c
	}

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

	// Get the root filesystem from the driver
	rootfs, err := runtime.driver.Get(container.ID)
	if err != nil {
		return fmt.Errorf("Error getting container filesystem %s from driver %s: %s", container.ID, runtime.driver, err)
	}
	container.rootfs = rootfs

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
		go container.monitor(hostConfig)
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

	if err := runtime.driver.Remove(container.ID); err != nil {
		return fmt.Errorf("Driver %s failed to remove root filesystem %s: %s", runtime.driver, container.ID, err)
	}

	if _, err := runtime.containerGraph.Purge(container.ID); err != nil {
		utils.Debugf("Unable to remove container from link graph: %s", err)
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
	containers := make(map[string]*Container)

	for i, v := range dir {
		id := v.Name()
		container, err := runtime.load(id)
		if i%21 == 0 && os.Getenv("DEBUG") == "" && os.Getenv("TEST") == "" {
			fmt.Printf("\b%c", wheel[i%4])
		}
		if err != nil {
			utils.Errorf("Failed to load container %v: %v", id, err)
			continue
		}
		utils.Debugf("Loaded container %v", container.ID)
		containers[container.ID] = container
	}

	register := func(container *Container) {
		if err := runtime.Register(container); err != nil {
			utils.Debugf("Failed to register container %s: %s", container.ID, err)
		}
	}

	if entities := runtime.containerGraph.List("/", -1); entities != nil {
		for _, p := range entities.Paths() {
			e := entities[p]
			if container, ok := containers[e.ID()]; ok {
				register(container)
				delete(containers, e.ID())
			}
		}
	}

	// Any containers that are left over do not exist in the graph
	for _, container := range containers {
		// Try to set the default name for a container if it exists prior to links
		name, err := generateRandomName(runtime)
		if err != nil {
			container.Name = container.ShortID()
		}
		container.Name = name

		if _, err := runtime.containerGraph.Set(name, container.ID); err != nil {
			utils.Debugf("Setting default id - %s", err)
		}
		register(container)
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

// Create creates a new container from the given configuration with a given name.
func (runtime *Runtime) Create(config *Config, name string) (*Container, []string, error) {
	// Lookup image
	img, err := runtime.repositories.LookupImage(config.Image)
	if err != nil {
		return nil, nil, err
	}

	checkDeprecatedExpose := func(config *Config) bool {
		if config != nil {
			if config.PortSpecs != nil {
				for _, p := range config.PortSpecs {
					if strings.Contains(p, ":") {
						return true
					}
				}
			}
		}
		return false
	}

	warnings := []string{}
	if checkDeprecatedExpose(img.Config) || checkDeprecatedExpose(config) {
		warnings = append(warnings, "The mapping to public ports on your host has been deprecated. Use -p to publish the ports.")
	}

	if img.Config != nil {
		if err := MergeConfig(config, img.Config); err != nil {
			return nil, nil, err
		}
	}

	if len(config.Entrypoint) != 0 && config.Cmd == nil {
		config.Cmd = []string{}
	} else if config.Cmd == nil || len(config.Cmd) == 0 {
		return nil, nil, fmt.Errorf("No command specified")
	}

	sysInitPath := utils.DockerInitPath()
	if sysInitPath == "" {
		return nil, nil, fmt.Errorf("Could not locate dockerinit: This usually means docker was built incorrectly. See http://docs.docker.io/en/latest/contributing/devenvironment for official build instructions.")
	}

	// Generate id
	id := GenerateID()

	if name == "" {
		name, err = generateRandomName(runtime)
		if err != nil {
			name = utils.TruncateID(id)
		}
	}
	if name[0] != '/' {
		name = "/" + name
	}

	// Set the enitity in the graph using the default name specified
	if _, err := runtime.containerGraph.Set(name, id); err != nil {
		if strings.HasSuffix(err.Error(), "name are not unique") {
			return nil, nil, fmt.Errorf("Conflict, %s already exists.", name)
		}
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
		Name:        name,
	}
	container.root = runtime.containerRoot(container.ID)
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return nil, nil, err
	}

	initID := fmt.Sprintf("%s-init", container.ID)
	if err := runtime.driver.Create(initID, img.ID); err != nil {
		return nil, nil, err
	}
	initPath, err := runtime.driver.Get(initID)
	if err != nil {
		return nil, nil, err
	}
	if err := setupInitLayer(initPath); err != nil {
		return nil, nil, err
	}

	if err := runtime.driver.Create(container.ID, initID); err != nil {
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

func (runtime *Runtime) getFullName(name string) string {
	if name[0] != '/' {
		name = "/" + name
	}
	return name
}

func (runtime *Runtime) GetByName(name string) (*Container, error) {
	entity := runtime.containerGraph.Get(runtime.getFullName(name))
	if entity == nil {
		return nil, fmt.Errorf("Could not find entity for %s", name)
	}
	e := runtime.getContainerElement(entity.ID())
	if e == nil {
		return nil, fmt.Errorf("Could not find container for entity id %s", entity.ID())
	}
	return e.Value.(*Container), nil
}

func (runtime *Runtime) Children(name string) (map[string]*Container, error) {
	name = runtime.getFullName(name)
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

func (runtime *Runtime) RegisterLink(parent, child *Container, alias string) error {
	fullName := path.Join(parent.Name, alias)
	if !runtime.containerGraph.Exists(fullName) {
		_, err := runtime.containerGraph.Set(fullName, child.ID)
		return err
	}
	return nil
}

// FIXME: harmonize with NewGraph()
func NewRuntime(config *DaemonConfig) (*Runtime, error) {
	runtime, err := NewRuntimeFromDirectory(config)
	if err != nil {
		return nil, err
	}
	runtime.UpdateCapabilities(false)
	return runtime, nil
}

func NewRuntimeFromDirectory(config *DaemonConfig) (*Runtime, error) {
	// Load storage driver
	driver, err := graphdriver.New(config.Root)
	if err != nil {
		return nil, err
	}

	runtimeRepo := path.Join(config.Root, "containers")

	if err := os.MkdirAll(runtimeRepo, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	g, err := NewGraph(path.Join(config.Root, "graph"), driver)
	if err != nil {
		return nil, err
	}
	volumes, err := NewGraph(path.Join(config.Root, "volumes"), driver)
	if err != nil {
		return nil, err
	}
	repositories, err := NewTagStore(path.Join(config.Root, "repositories"), g)
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

	gographPath := path.Join(config.Root, "linkgraph.db")
	initDatabase := false
	if _, err := os.Stat(gographPath); err != nil {
		if os.IsNotExist(err) {
			initDatabase = true
		} else {
			return nil, err
		}
	}
	conn, err := sql.Open("sqlite3", gographPath)
	if err != nil {
		return nil, err
	}
	graph, err := gograph.NewDatabase(conn, initDatabase)
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
		driver:		driver,
	}

	if err := runtime.restore(); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (runtime *Runtime) Close() error {
	runtime.networkManager.Close()
	runtime.driver.Cleanup()
	return runtime.containerGraph.Close()
}

func (runtime *Runtime) Mount(container *Container) error {
	dir, err := runtime.driver.Get(container.ID)
	if err != nil {
		return fmt.Errorf("Error getting container %s from driver %s: %s", container.ID, runtime.driver, err)
	}
	if container.rootfs == "" {
		container.rootfs = dir
	} else if container.rootfs != dir {
		return fmt.Errorf("Error: driver %s is returning inconsistent paths for container %s ('%s' then '%s')",
			runtime.driver, container.ID, container.rootfs, dir)
	}
	return nil
}

func (runtime *Runtime) Unmount(container *Container) error {
	// FIXME: Unmount is deprecated because drivers are responsible for mounting
	// and unmounting when necessary. Use driver.Remove() instead.
	return nil
}

func (runtime *Runtime) Changes(container *Container) ([]graphdriver.Change, error) {
	return runtime.driver.Changes(container.ID)
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
