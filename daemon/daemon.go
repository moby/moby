package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/libcontainer/label"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/execdrivers"
	"github.com/docker/docker/daemon/execdriver/lxc"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/vfs"
	_ "github.com/docker/docker/daemon/networkdriver/bridge"
	"github.com/docker/docker/daemon/networkdriver/portallocator"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/broadcastwriter"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/networkfs/resolvconf"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

var (
	DefaultDns                = []string{"8.8.8.8", "8.8.4.4"}
	validContainerNameChars   = `[a-zA-Z0-9_.-]`
	validContainerNamePattern = regexp.MustCompile(`^/?` + validContainerNameChars + `+$`)
)

type contStore struct {
	s map[string]*Container
	sync.Mutex
}

func (c *contStore) Add(id string, cont *Container) {
	c.Lock()
	c.s[id] = cont
	c.Unlock()
}

func (c *contStore) Get(id string) *Container {
	c.Lock()
	res := c.s[id]
	c.Unlock()
	return res
}

func (c *contStore) Delete(id string) {
	c.Lock()
	delete(c.s, id)
	c.Unlock()
}

func (c *contStore) List() []*Container {
	containers := new(History)
	c.Lock()
	for _, cont := range c.s {
		containers.Add(cont)
	}
	c.Unlock()
	containers.Sort()
	return *containers
}

type Daemon struct {
	repository     string
	sysInitPath    string
	containers     *contStore
	graph          *graph.Graph
	repositories   *graph.TagStore
	idIndex        *truncindex.TruncIndex
	sysInfo        *sysinfo.SysInfo
	volumes        *graph.Graph
	eng            *engine.Engine
	config         *Config
	containerGraph *graphdb.Database
	driver         graphdriver.Driver
	execDriver     execdriver.Driver
}

// Install installs daemon capabilities to eng.
func (daemon *Daemon) Install(eng *engine.Engine) error {
	// FIXME: remove ImageDelete's dependency on Daemon, then move to graph/
	for name, method := range map[string]engine.Handler{
		"attach":            daemon.ContainerAttach,
		"build":             daemon.CmdBuild,
		"commit":            daemon.ContainerCommit,
		"container_changes": daemon.ContainerChanges,
		"container_copy":    daemon.ContainerCopy,
		"container_inspect": daemon.ContainerInspect,
		"containers":        daemon.Containers,
		"create":            daemon.ContainerCreate,
		"rm":                daemon.ContainerRm,
		"export":            daemon.ContainerExport,
		"info":              daemon.CmdInfo,
		"kill":              daemon.ContainerKill,
		"logs":              daemon.ContainerLogs,
		"pause":             daemon.ContainerPause,
		"resize":            daemon.ContainerResize,
		"restart":           daemon.ContainerRestart,
		"start":             daemon.ContainerStart,
		"stop":              daemon.ContainerStop,
		"top":               daemon.ContainerTop,
		"unpause":           daemon.ContainerUnpause,
		"wait":              daemon.ContainerWait,
		"image_delete":      daemon.ImageDelete, // FIXME: see above
	} {
		if err := eng.Register(name, method); err != nil {
			return err
		}
	}
	if err := daemon.Repositories().Install(eng); err != nil {
		return err
	}
	// FIXME: this hack is necessary for legacy integration tests to access
	// the daemon object.
	eng.Hack_SetGlobalVar("httpapi.daemon", daemon)
	return nil
}

// Get looks for a container by the specified ID or name, and returns it.
// If the container is not found, or if an error occurs, nil is returned.
func (daemon *Daemon) Get(name string) *Container {
	if id, err := daemon.idIndex.Get(name); err == nil {
		return daemon.containers.Get(id)
	}
	if c, _ := daemon.GetByName(name); c != nil {
		return c
	}
	return nil
}

// Exists returns a true if a container of the specified ID or name exists,
// false otherwise.
func (daemon *Daemon) Exists(id string) bool {
	return daemon.Get(id) != nil
}

func (daemon *Daemon) containerRoot(id string) string {
	return path.Join(daemon.repository, id)
}

// Load reads the contents of a container from disk
// This is typically done at startup.
func (daemon *Daemon) load(id string) (*Container, error) {
	container := &Container{root: daemon.containerRoot(id), State: NewState()}
	if err := container.FromDisk(); err != nil {
		return nil, err
	}

	if container.ID != id {
		return container, fmt.Errorf("Container %s is stored at %s", container.ID, id)
	}

	container.readHostConfig()

	return container, nil
}

// Register makes a container object usable by the daemon as <container.ID>
// This is a wrapper for register
func (daemon *Daemon) Register(container *Container) error {
	return daemon.register(container, true)
}

// register makes a container object usable by the daemon as <container.ID>
func (daemon *Daemon) register(container *Container, updateSuffixarray bool) error {
	if container.daemon != nil || daemon.Exists(container.ID) {
		return fmt.Errorf("Container is already loaded")
	}
	if err := validateID(container.ID); err != nil {
		return err
	}
	if err := daemon.ensureName(container); err != nil {
		return err
	}

	container.daemon = daemon

	// Attach to stdout and stderr
	container.stderr = broadcastwriter.New()
	container.stdout = broadcastwriter.New()
	// Attach to stdin
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	} else {
		container.stdinPipe = utils.NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}
	// done
	daemon.containers.Add(container.ID, container)

	// don't update the Suffixarray if we're starting up
	// we'll waste time if we update it for every container
	daemon.idIndex.Add(container.ID)

	// FIXME: if the container is supposed to be running but is not, auto restart it?
	//        if so, then we need to restart monitor and init a new lock
	// If the container is supposed to be running, make sure of it
	if container.State.IsRunning() {
		log.Debugf("killing old running container %s", container.ID)

		existingPid := container.State.Pid
		container.State.SetStopped(0)

		// We only have to handle this for lxc because the other drivers will ensure that
		// no processes are left when docker dies
		if container.ExecDriver == "" || strings.Contains(container.ExecDriver, "lxc") {
			lxc.KillLxc(container.ID, 9)
		} else {
			// use the current driver and ensure that the container is dead x.x
			cmd := &execdriver.Command{
				ID: container.ID,
			}
			var err error
			cmd.Process, err = os.FindProcess(existingPid)
			if err != nil {
				log.Debugf("cannot find existing process for %d", existingPid)
			}
			daemon.execDriver.Terminate(cmd)
		}

		if err := container.Unmount(); err != nil {
			log.Debugf("unmount error %s", err)
		}
		if err := container.ToDisk(); err != nil {
			log.Debugf("saving stopped state to disk %s", err)
		}

		info := daemon.execDriver.Info(container.ID)
		if !info.IsRunning() {
			log.Debugf("Container %s was supposed to be running but is not.", container.ID)

			log.Debugf("Marking as stopped")

			container.State.SetStopped(-127)
			if err := container.ToDisk(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (daemon *Daemon) ensureName(container *Container) error {
	if container.Name == "" {
		name, err := daemon.generateNewName(container.ID)
		if err != nil {
			return err
		}
		container.Name = name

		if err := container.ToDisk(); err != nil {
			log.Debugf("Error saving container name %s", err)
		}
	}
	return nil
}

func (daemon *Daemon) LogToDisk(src *broadcastwriter.BroadcastWriter, dst, stream string) error {
	log, err := os.OpenFile(dst, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	src.AddWriter(log, stream)
	return nil
}

func (daemon *Daemon) restore() error {
	var (
		debug         = (os.Getenv("DEBUG") != "" || os.Getenv("TEST") != "")
		containers    = make(map[string]*Container)
		currentDriver = daemon.driver.String()
	)

	if !debug {
		log.Infof("Loading containers: ")
	}
	dir, err := ioutil.ReadDir(daemon.repository)
	if err != nil {
		return err
	}

	for _, v := range dir {
		id := v.Name()
		container, err := daemon.load(id)
		if !debug {
			fmt.Print(".")
		}
		if err != nil {
			log.Errorf("Failed to load container %v: %v", id, err)
			continue
		}

		// Ignore the container if it does not support the current driver being used by the graph
		if (container.Driver == "" && currentDriver == "aufs") || container.Driver == currentDriver {
			log.Debugf("Loaded container %v", container.ID)

			containers[container.ID] = container
		} else {
			log.Debugf("Cannot load container %s because it was created with another graph driver.", container.ID)
		}
	}

	registeredContainers := []*Container{}

	if entities := daemon.containerGraph.List("/", -1); entities != nil {
		for _, p := range entities.Paths() {
			if !debug {
				fmt.Print(".")
			}

			e := entities[p]

			if container, ok := containers[e.ID()]; ok {
				if err := daemon.register(container, false); err != nil {
					log.Debugf("Failed to register container %s: %s", container.ID, err)
				}

				registeredContainers = append(registeredContainers, container)

				// delete from the map so that a new name is not automatically generated
				delete(containers, e.ID())
			}
		}
	}

	// Any containers that are left over do not exist in the graph
	for _, container := range containers {
		// Try to set the default name for a container if it exists prior to links
		container.Name, err = daemon.generateNewName(container.ID)
		if err != nil {
			log.Debugf("Setting default id - %s", err)
		}

		if err := daemon.register(container, false); err != nil {
			log.Debugf("Failed to register container %s: %s", container.ID, err)
		}

		registeredContainers = append(registeredContainers, container)
	}

	// check the restart policy on the containers and restart any container with
	// the restart policy of "always"
	if daemon.config.AutoRestart {
		log.Debugf("Restarting containers...")

		for _, container := range registeredContainers {
			if container.hostConfig.RestartPolicy.Name == "always" ||
				(container.hostConfig.RestartPolicy.Name == "on-failure" && container.State.ExitCode != 0) {
				log.Debugf("Starting container %s", container.ID)

				if err := container.Start(); err != nil {
					log.Debugf("Failed to start container %s: %s", container.ID, err)
				}
			}
		}
	}

	if !debug {
		log.Infof(": done.")
	}

	return nil
}

func (daemon *Daemon) checkDeprecatedExpose(config *runconfig.Config) bool {
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

func (daemon *Daemon) mergeAndVerifyConfig(config *runconfig.Config, img *image.Image) ([]string, error) {
	warnings := []string{}
	if daemon.checkDeprecatedExpose(img.Config) || daemon.checkDeprecatedExpose(config) {
		warnings = append(warnings, "The mapping to public ports on your host via Dockerfile EXPOSE (host:port:port) has been deprecated. Use -p to publish the ports.")
	}
	if img.Config != nil {
		if err := runconfig.Merge(config, img.Config); err != nil {
			return nil, err
		}
	}
	if len(config.Entrypoint) == 0 && len(config.Cmd) == 0 {
		return nil, fmt.Errorf("No command specified")
	}
	return warnings, nil
}

func (daemon *Daemon) generateIdAndName(name string) (string, string, error) {
	var (
		err error
		id  = utils.GenerateRandomID()
	)

	if name == "" {
		if name, err = daemon.generateNewName(id); err != nil {
			return "", "", err
		}
		return id, name, nil
	}

	if name, err = daemon.reserveName(id, name); err != nil {
		return "", "", err
	}

	return id, name, nil
}

func (daemon *Daemon) reserveName(id, name string) (string, error) {
	if !validContainerNamePattern.MatchString(name) {
		return "", fmt.Errorf("Invalid container name (%s), only %s are allowed", name, validContainerNameChars)
	}

	if name[0] != '/' {
		name = "/" + name
	}

	if _, err := daemon.containerGraph.Set(name, id); err != nil {
		if !graphdb.IsNonUniqueNameError(err) {
			return "", err
		}

		conflictingContainer, err := daemon.GetByName(name)
		if err != nil {
			if strings.Contains(err.Error(), "Could not find entity") {
				return "", err
			}

			// Remove name and continue starting the container
			if err := daemon.containerGraph.Delete(name); err != nil {
				return "", err
			}
		} else {
			nameAsKnownByUser := strings.TrimPrefix(name, "/")
			return "", fmt.Errorf(
				"Conflict, The name %s is already assigned to %s. You have to delete (or rename) that container to be able to assign %s to a container again.", nameAsKnownByUser,
				utils.TruncateID(conflictingContainer.ID), nameAsKnownByUser)
		}
	}
	return name, nil
}

func (daemon *Daemon) generateNewName(id string) (string, error) {
	var name string
	for i := 0; i < 6; i++ {
		name = namesgenerator.GetRandomName(i)
		if name[0] != '/' {
			name = "/" + name
		}

		if _, err := daemon.containerGraph.Set(name, id); err != nil {
			if !graphdb.IsNonUniqueNameError(err) {
				return "", err
			}
			continue
		}
		return name, nil
	}

	name = "/" + utils.TruncateID(id)
	if _, err := daemon.containerGraph.Set(name, id); err != nil {
		return "", err
	}
	return name, nil
}

func (daemon *Daemon) generateHostname(id string, config *runconfig.Config) {
	// Generate default hostname
	// FIXME: the lxc template no longer needs to set a default hostname
	if config.Hostname == "" {
		config.Hostname = id[:12]
	}
}

func (daemon *Daemon) getEntrypointAndArgs(config *runconfig.Config) (string, []string) {
	var (
		entrypoint string
		args       []string
	)
	if len(config.Entrypoint) != 0 {
		entrypoint = config.Entrypoint[0]
		args = append(config.Entrypoint[1:], config.Cmd...)
	} else {
		entrypoint = config.Cmd[0]
		args = config.Cmd[1:]
	}
	return entrypoint, args
}

func (daemon *Daemon) newContainer(name string, config *runconfig.Config, img *image.Image) (*Container, error) {
	var (
		id  string
		err error
	)
	id, name, err = daemon.generateIdAndName(name)
	if err != nil {
		return nil, err
	}

	daemon.generateHostname(id, config)
	entrypoint, args := daemon.getEntrypointAndArgs(config)

	container := &Container{
		// FIXME: we should generate the ID here instead of receiving it as an argument
		ID:              id,
		Created:         time.Now().UTC(),
		Path:            entrypoint,
		Args:            args, //FIXME: de-duplicate from config
		Config:          config,
		hostConfig:      &runconfig.HostConfig{},
		Image:           img.ID, // Always use the resolved image id
		NetworkSettings: &NetworkSettings{},
		Name:            name,
		Driver:          daemon.driver.String(),
		ExecDriver:      daemon.execDriver.Name(),
		State:           NewState(),
	}
	container.root = daemon.containerRoot(container.ID)

	if container.ProcessLabel, container.MountLabel, err = label.GenLabels(""); err != nil {
		return nil, err
	}
	return container, nil
}

func (daemon *Daemon) createRootfs(container *Container, img *image.Image) error {
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return err
	}
	initID := fmt.Sprintf("%s-init", container.ID)
	if err := daemon.driver.Create(initID, img.ID); err != nil {
		return err
	}
	initPath, err := daemon.driver.Get(initID, "")
	if err != nil {
		return err
	}
	defer daemon.driver.Put(initID)

	if err := graph.SetupInitLayer(initPath); err != nil {
		return err
	}

	if err := daemon.driver.Create(container.ID, initID); err != nil {
		return err
	}
	return nil
}

func GetFullContainerName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("Container name cannot be empty")
	}
	if name[0] != '/' {
		name = "/" + name
	}
	return name, nil
}

func (daemon *Daemon) GetByName(name string) (*Container, error) {
	fullName, err := GetFullContainerName(name)
	if err != nil {
		return nil, err
	}
	entity := daemon.containerGraph.Get(fullName)
	if entity == nil {
		return nil, fmt.Errorf("Could not find entity for %s", name)
	}
	e := daemon.containers.Get(entity.ID())
	if e == nil {
		return nil, fmt.Errorf("Could not find container for entity id %s", entity.ID())
	}
	return e, nil
}

func (daemon *Daemon) Children(name string) (map[string]*Container, error) {
	name, err := GetFullContainerName(name)
	if err != nil {
		return nil, err
	}
	children := make(map[string]*Container)

	err = daemon.containerGraph.Walk(name, func(p string, e *graphdb.Entity) error {
		c := daemon.Get(e.ID())
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

func (daemon *Daemon) RegisterLink(parent, child *Container, alias string) error {
	fullName := path.Join(parent.Name, alias)
	if !daemon.containerGraph.Exists(fullName) {
		_, err := daemon.containerGraph.Set(fullName, child.ID)
		return err
	}
	return nil
}

func (daemon *Daemon) RegisterLinks(container *Container, hostConfig *runconfig.HostConfig) error {
	if hostConfig != nil && hostConfig.Links != nil {
		for _, l := range hostConfig.Links {
			parts, err := parsers.PartParser("name:alias", l)
			if err != nil {
				return err
			}
			child, err := daemon.GetByName(parts["name"])
			if err != nil {
				return err
			}
			if child == nil {
				return fmt.Errorf("Could not get container for %s", parts["name"])
			}
			if err := daemon.RegisterLink(container, child, parts["alias"]); err != nil {
				return err
			}
		}

		// After we load all the links into the daemon
		// set them to nil on the hostconfig
		hostConfig.Links = nil
		if err := container.WriteHostConfig(); err != nil {
			return err
		}
	}
	return nil
}

// FIXME: harmonize with NewGraph()
func NewDaemon(config *Config, eng *engine.Engine) (*Daemon, error) {
	daemon, err := NewDaemonFromDirectory(config, eng)
	if err != nil {
		return nil, err
	}
	return daemon, nil
}

func NewDaemonFromDirectory(config *Config, eng *engine.Engine) (*Daemon, error) {
	// Apply configuration defaults
	if config.Mtu == 0 {
		// FIXME: GetDefaultNetwork Mtu doesn't need to be public anymore
		config.Mtu = GetDefaultNetworkMtu()
	}
	// Check for mutually incompatible config options
	if config.BridgeIface != "" && config.BridgeIP != "" {
		return nil, fmt.Errorf("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}
	if !config.EnableIptables && !config.InterContainerCommunication {
		return nil, fmt.Errorf("You specified --iptables=false with --icc=false. ICC uses iptables to function. Please set --icc or --iptables to true.")
	}
	// FIXME: DisableNetworkBidge doesn't need to be public anymore
	config.DisableNetwork = config.BridgeIface == DisableNetworkBridge

	// Claim the pidfile first, to avoid any and all unexpected race conditions.
	// Some of the init doesn't need a pidfile lock - but let's not try to be smart.
	if config.Pidfile != "" {
		if err := utils.CreatePidFile(config.Pidfile); err != nil {
			return nil, err
		}
		eng.OnShutdown(func() {
			// Always release the pidfile last, just in case
			utils.RemovePidFile(config.Pidfile)
		})
	}

	// Check that the system is supported and we have sufficient privileges
	// FIXME: return errors instead of calling Fatal
	if runtime.GOOS != "linux" {
		log.Fatalf("The Docker daemon is only supported on linux")
	}
	if os.Geteuid() != 0 {
		log.Fatalf("The Docker daemon needs to be run as root")
	}
	if err := checkKernelAndArch(); err != nil {
		log.Fatalf(err.Error())
	}

	// set up the TempDir to use a canonical path
	tmp, err := utils.TempDir(config.Root)
	if err != nil {
		log.Fatalf("Unable to get the TempDir under %s: %s", config.Root, err)
	}
	realTmp, err := utils.ReadSymlinkedDirectory(tmp)
	if err != nil {
		log.Fatalf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
	}
	os.Setenv("TMPDIR", realTmp)
	if !config.EnableSelinuxSupport {
		selinuxSetDisabled()
	}

	// get the canonical path to the Docker root directory
	var realRoot string
	if _, err := os.Stat(config.Root); err != nil && os.IsNotExist(err) {
		realRoot = config.Root
	} else {
		realRoot, err = utils.ReadSymlinkedDirectory(config.Root)
		if err != nil {
			log.Fatalf("Unable to get the full path to root (%s): %s", config.Root, err)
		}
	}
	config.Root = realRoot
	// Create the root directory if it doesn't exists
	if err := os.MkdirAll(config.Root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// Set the default driver
	graphdriver.DefaultDriver = config.GraphDriver

	// Load storage driver
	driver, err := graphdriver.New(config.Root, config.GraphOptions)
	if err != nil {
		return nil, err
	}
	log.Debugf("Using graph driver %s", driver)

	// As Docker on btrfs and SELinux are incompatible at present, error on both being enabled
	if config.EnableSelinuxSupport && driver.String() == "btrfs" {
		return nil, fmt.Errorf("SELinux is not supported with the BTRFS graph driver!")
	}

	daemonRepo := path.Join(config.Root, "containers")

	if err := os.MkdirAll(daemonRepo, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// Migrate the container if it is aufs and aufs is enabled
	if err = migrateIfAufs(driver, config.Root); err != nil {
		return nil, err
	}

	log.Debugf("Creating images graph")
	g, err := graph.NewGraph(path.Join(config.Root, "graph"), driver)
	if err != nil {
		return nil, err
	}

	// We don't want to use a complex driver like aufs or devmapper
	// for volumes, just a plain filesystem
	volumesDriver, err := graphdriver.GetDriver("vfs", config.Root, config.GraphOptions)
	if err != nil {
		return nil, err
	}
	log.Debugf("Creating volumes graph")
	volumes, err := graph.NewGraph(path.Join(config.Root, "volumes"), volumesDriver)
	if err != nil {
		return nil, err
	}
	log.Debugf("Creating repository list")
	repositories, err := graph.NewTagStore(path.Join(config.Root, "repositories-"+driver.String()), g)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Tag store: %s", err)
	}

	if !config.DisableNetwork {
		job := eng.Job("init_networkdriver")

		job.SetenvBool("EnableIptables", config.EnableIptables)
		job.SetenvBool("InterContainerCommunication", config.InterContainerCommunication)
		job.SetenvBool("EnableIpForward", config.EnableIpForward)
		job.Setenv("BridgeIface", config.BridgeIface)
		job.Setenv("BridgeIP", config.BridgeIP)
		job.Setenv("DefaultBindingIP", config.DefaultIp.String())

		if err := job.Run(); err != nil {
			return nil, err
		}
	}

	graphdbPath := path.Join(config.Root, "linkgraph.db")
	graph, err := graphdb.NewSqliteConn(graphdbPath)
	if err != nil {
		return nil, err
	}

	localCopy := path.Join(config.Root, "init", fmt.Sprintf("dockerinit-%s", dockerversion.VERSION))
	sysInitPath := utils.DockerInitPath(localCopy)
	if sysInitPath == "" {
		return nil, fmt.Errorf("Could not locate dockerinit: This usually means docker was built incorrectly. See http://docs.docker.com/contributing/devenvironment for official build instructions.")
	}

	if sysInitPath != localCopy {
		// When we find a suitable dockerinit binary (even if it's our local binary), we copy it into config.Root at localCopy for future use (so that the original can go away without that being a problem, for example during a package upgrade).
		if err := os.Mkdir(path.Dir(localCopy), 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
		if _, err := utils.CopyFile(sysInitPath, localCopy); err != nil {
			return nil, err
		}
		if err := os.Chmod(localCopy, 0700); err != nil {
			return nil, err
		}
		sysInitPath = localCopy
	}

	sysInfo := sysinfo.New(false)
	ed, err := execdrivers.NewDriver(config.ExecDriver, config.Root, sysInitPath, sysInfo)
	if err != nil {
		return nil, err
	}

	daemon := &Daemon{
		repository:     daemonRepo,
		containers:     &contStore{s: make(map[string]*Container)},
		graph:          g,
		repositories:   repositories,
		idIndex:        truncindex.NewTruncIndex([]string{}),
		sysInfo:        sysInfo,
		volumes:        volumes,
		config:         config,
		containerGraph: graph,
		driver:         driver,
		sysInitPath:    sysInitPath,
		execDriver:     ed,
		eng:            eng,
	}
	if err := daemon.checkLocaldns(); err != nil {
		return nil, err
	}
	if err := daemon.restore(); err != nil {
		return nil, err
	}
	// Setup shutdown handlers
	// FIXME: can these shutdown handlers be registered closer to their source?
	eng.OnShutdown(func() {
		// FIXME: if these cleanup steps can be called concurrently, register
		// them as separate handlers to speed up total shutdown time
		// FIXME: use engine logging instead of log.Errorf
		if err := daemon.shutdown(); err != nil {
			log.Errorf("daemon.shutdown(): %s", err)
		}
		if err := portallocator.ReleaseAll(); err != nil {
			log.Errorf("portallocator.ReleaseAll(): %s", err)
		}
		if err := daemon.driver.Cleanup(); err != nil {
			log.Errorf("daemon.driver.Cleanup(): %s", err.Error())
		}
		if err := daemon.containerGraph.Close(); err != nil {
			log.Errorf("daemon.containerGraph.Close(): %s", err.Error())
		}
	})

	return daemon, nil
}

func (daemon *Daemon) shutdown() error {
	group := sync.WaitGroup{}
	log.Debugf("starting clean shutdown of all containers...")
	for _, container := range daemon.List() {
		c := container
		if c.State.IsRunning() {
			log.Debugf("stopping %s", c.ID)
			group.Add(1)

			go func() {
				defer group.Done()
				if err := c.KillSig(15); err != nil {
					log.Debugf("kill 15 error for %s - %s", c.ID, err)
				}
				c.State.WaitStop(-1 * time.Second)
				log.Debugf("container stopped %s", c.ID)
			}()
		}
	}
	group.Wait()

	return nil
}

func (daemon *Daemon) Mount(container *Container) error {
	dir, err := daemon.driver.Get(container.ID, container.GetMountLabel())
	if err != nil {
		return fmt.Errorf("Error getting container %s from driver %s: %s", container.ID, daemon.driver, err)
	}
	if container.basefs == "" {
		container.basefs = dir
	} else if container.basefs != dir {
		return fmt.Errorf("Error: driver %s is returning inconsistent paths for container %s ('%s' then '%s')",
			daemon.driver, container.ID, container.basefs, dir)
	}
	return nil
}

func (daemon *Daemon) Unmount(container *Container) error {
	daemon.driver.Put(container.ID)
	return nil
}

func (daemon *Daemon) Changes(container *Container) ([]archive.Change, error) {
	if differ, ok := daemon.driver.(graphdriver.Differ); ok {
		return differ.Changes(container.ID)
	}
	cDir, err := daemon.driver.Get(container.ID, "")
	if err != nil {
		return nil, fmt.Errorf("Error getting container rootfs %s from driver %s: %s", container.ID, container.daemon.driver, err)
	}
	defer daemon.driver.Put(container.ID)
	initDir, err := daemon.driver.Get(container.ID+"-init", "")
	if err != nil {
		return nil, fmt.Errorf("Error getting container init rootfs %s from driver %s: %s", container.ID, container.daemon.driver, err)
	}
	defer daemon.driver.Put(container.ID + "-init")
	return archive.ChangesDirs(cDir, initDir)
}

func (daemon *Daemon) Diff(container *Container) (archive.Archive, error) {
	if differ, ok := daemon.driver.(graphdriver.Differ); ok {
		return differ.Diff(container.ID)
	}

	changes, err := daemon.Changes(container)
	if err != nil {
		return nil, err
	}

	cDir, err := daemon.driver.Get(container.ID, "")
	if err != nil {
		return nil, fmt.Errorf("Error getting container rootfs %s from driver %s: %s", container.ID, container.daemon.driver, err)
	}

	archive, err := archive.ExportChanges(cDir, changes)
	if err != nil {
		return nil, err
	}
	return utils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		daemon.driver.Put(container.ID)
		return err
	}), nil
}

func (daemon *Daemon) Run(c *Container, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	return daemon.execDriver.Run(c.command, pipes, startCallback)
}

func (daemon *Daemon) Pause(c *Container) error {
	if err := daemon.execDriver.Pause(c.command); err != nil {
		return err
	}
	c.State.SetPaused()
	return nil
}

func (daemon *Daemon) Unpause(c *Container) error {
	if err := daemon.execDriver.Unpause(c.command); err != nil {
		return err
	}
	c.State.SetUnpaused()
	return nil
}

func (daemon *Daemon) Kill(c *Container, sig int) error {
	return daemon.execDriver.Kill(c.command, sig)
}

// Nuke kills all containers then removes all content
// from the content root, including images, volumes and
// container filesystems.
// Again: this will remove your entire docker daemon!
// FIXME: this is deprecated, and only used in legacy
// tests. Please remove.
func (daemon *Daemon) Nuke() error {
	var wg sync.WaitGroup
	for _, container := range daemon.List() {
		wg.Add(1)
		go func(c *Container) {
			c.Kill()
			wg.Done()
		}(container)
	}
	wg.Wait()

	return os.RemoveAll(daemon.config.Root)
}

// FIXME: this is a convenience function for integration tests
// which need direct access to daemon.graph.
// Once the tests switch to using engine and jobs, this method
// can go away.
func (daemon *Daemon) Graph() *graph.Graph {
	return daemon.graph
}

func (daemon *Daemon) Repositories() *graph.TagStore {
	return daemon.repositories
}

func (daemon *Daemon) Config() *Config {
	return daemon.config
}

func (daemon *Daemon) SystemConfig() *sysinfo.SysInfo {
	return daemon.sysInfo
}

func (daemon *Daemon) SystemInitPath() string {
	return daemon.sysInitPath
}

func (daemon *Daemon) GraphDriver() graphdriver.Driver {
	return daemon.driver
}

func (daemon *Daemon) ExecutionDriver() execdriver.Driver {
	return daemon.execDriver
}

func (daemon *Daemon) Volumes() *graph.Graph {
	return daemon.volumes
}

func (daemon *Daemon) ContainerGraph() *graphdb.Database {
	return daemon.containerGraph
}

func (daemon *Daemon) checkLocaldns() error {
	resolvConf, err := resolvconf.Get()
	if err != nil {
		return err
	}
	if len(daemon.config.Dns) == 0 && utils.CheckLocalDns(resolvConf) {
		log.Infof("Local (127.0.0.1) DNS resolver found in resolv.conf and containers can't use it. Using default external servers : %v", DefaultDns)
		daemon.config.Dns = DefaultDns
	}
	return nil
}

func (daemon *Daemon) ImageGetCached(imgID string, config *runconfig.Config) (*image.Image, error) {
	// Retrieve all images
	images, err := daemon.Graph().Map()
	if err != nil {
		return nil, err
	}

	// Store the tree in a map of map (map[parentId][childId])
	imageMap := make(map[string]map[string]struct{})
	for _, img := range images {
		if _, exists := imageMap[img.Parent]; !exists {
			imageMap[img.Parent] = make(map[string]struct{})
		}
		imageMap[img.Parent][img.ID] = struct{}{}
	}

	// Loop on the children of the given image and check the config
	var match *image.Image
	for elem := range imageMap[imgID] {
		img, err := daemon.Graph().Get(elem)
		if err != nil {
			return nil, err
		}
		if runconfig.Compare(&img.ContainerConfig, config) {
			if match == nil || match.Created.Before(img.Created) {
				match = img
			}
		}
	}
	return match, nil
}

func checkKernelAndArch() error {
	// Check for unsupported architectures
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("The Docker runtime currently only supports amd64 (not %s). This will change in the future. Aborting.", runtime.GOARCH)
	}
	// Check for unsupported kernel versions
	// FIXME: it would be cleaner to not test for specific versions, but rather
	// test for specific functionalities.
	// Unfortunately we can't test for the feature "does not cause a kernel panic"
	// without actually causing a kernel panic, so we need this workaround until
	// the circumstances of pre-3.8 crashes are clearer.
	// For details see http://github.com/docker/docker/issues/407
	if k, err := kernel.GetKernelVersion(); err != nil {
		log.Infof("WARNING: %s", err)
	} else {
		if kernel.CompareKernelVersion(k, &kernel.KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0}) < 0 {
			if os.Getenv("DOCKER_NOWARN_KERNEL_VERSION") == "" {
				log.Infof("WARNING: You are running linux kernel version %s, which might be unstable running docker. Please upgrade your kernel to 3.8.0.", k.String())
			}
		}
	}
	return nil
}
