package docker

import (
	"container/list"
	"fmt"
	"github.com/dotcloud/docker/graph"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"sync"
	"time"
)

type Runtime struct {
	root           string
	repository     string
	containers     *list.List
	networkManager *NetworkManager
	graph          *graph.Graph
	repositories   *graph.TagStore
}

var sysInitPath string

func init() {
	sysInitPath = SelfPath()
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
		if container.Id == id {
			return e
		}
	}
	return nil
}

func (runtime *Runtime) Get(id string) *Container {
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

func (runtime *Runtime) Create(command string, args []string, image string, config *Config) (*Container, error) {
	container := &Container{
		// FIXME: we should generate the ID here instead of receiving it as an argument
		Id:              GenerateId(),
		Created:         time.Now(),
		Path:            command,
		Args:            args,
		Config:          config,
		Image:           image,
		NetworkSettings: &NetworkSettings{},
		// FIXME: do we need to store this in the container?
		SysInitPath: sysInitPath,
	}
	container.root = runtime.containerRoot(container.Id)
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return nil, err
	}
	// Step 2: save the container json
	if err := container.ToDisk(); err != nil {
		return nil, err
	}
	// Step 3: register the container
	if err := runtime.Register(container); err != nil {
		return nil, err
	}
	return container, nil
}

func (runtime *Runtime) Load(id string) (*Container, error) {
	container := &Container{root: runtime.containerRoot(id)}
	if err := container.FromDisk(); err != nil {
		return nil, err
	}
	if container.Id != id {
		return container, fmt.Errorf("Container %s is stored at %s", container.Id, id)
	}
	if err := runtime.Register(container); err != nil {
		return nil, err
	}
	return container, nil
}

// Register makes a container object usable by the runtime as <container.Id>
func (runtime *Runtime) Register(container *Container) error {
	if container.runtime != nil || runtime.Exists(container.Id) {
		return fmt.Errorf("Container is already loaded")
	}
	if err := validateId(container.Id); err != nil {
		return err
	}
	container.runtime = runtime
	// Setup state lock (formerly in newState()
	lock := new(sync.Mutex)
	container.State.stateChangeLock = lock
	container.State.stateChangeCond = sync.NewCond(lock)
	// Attach to stdout and stderr
	container.stderr = newWriteBroadcaster()
	container.stdout = newWriteBroadcaster()
	// Attach to stdin
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	} else {
		container.stdinPipe = NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}
	// Setup logging of stdout and stderr to disk
	if err := runtime.LogToDisk(container.stdout, container.logPath("stdout")); err != nil {
		return err
	}
	if err := runtime.LogToDisk(container.stderr, container.logPath("stderr")); err != nil {
		return err
	}
	// done
	runtime.containers.PushBack(container)
	return nil
}

func (runtime *Runtime) LogToDisk(src *writeBroadcaster, dst string) error {
	log, err := os.OpenFile(dst, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	src.AddWriter(NopWriteCloser(log))
	return nil
}

func (runtime *Runtime) Destroy(container *Container) error {
	element := runtime.getContainerElement(container.Id)
	if element == nil {
		return fmt.Errorf("Container %v not found - maybe it was already destroyed?", container.Id)
	}

	if err := container.Stop(); err != nil {
		return err
	}
	if mounted, err := container.Mounted(); err != nil {
		return err
	} else if mounted {
		if err := container.Unmount(); err != nil {
			return fmt.Errorf("Unable to unmount container %v: %v", container.Id, err)
		}
	}
	// Deregister the container before removing its directory, to avoid race conditions
	runtime.containers.Remove(element)
	if err := os.RemoveAll(container.root); err != nil {
		return fmt.Errorf("Unable to remove filesystem for %v: %v", container.Id, err)
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
			log.Printf("Failed to load container %v: %v", id, err)
			continue
		}
		log.Printf("Loaded container %v", container.Id)
	}
	return nil
}

func New() (*Runtime, error) {
	return NewFromDirectory("/var/lib/docker")
}

func NewFromDirectory(root string) (*Runtime, error) {
	runtime_repo := path.Join(root, "containers")

	if err := os.MkdirAll(runtime_repo, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	g, err := graph.New(path.Join(root, "graph"))
	if err != nil {
		return nil, err
	}
	repositories, err := graph.NewTagStore(path.Join(root, "repositories"), g)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Tag store: %s", err)
	}
	netManager, err := newNetworkManager(networkBridgeIface)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		root:           root,
		repository:     runtime_repo,
		containers:     list.New(),
		networkManager: netManager,
		graph:          g,
		repositories:   repositories,
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
