package docker

import (
	"./fs"
	"container/list"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
)

type Docker struct {
	root           string
	repository     string
	containers     *list.List
	networkManager *NetworkManager
	Store          *fs.Store
}

func (docker *Docker) List() []*Container {
	containers := new(History)
	for e := docker.containers.Front(); e != nil; e = e.Next() {
		containers.Add(e.Value.(*Container))
	}
	return *containers
}

func (docker *Docker) getContainerElement(id string) *list.Element {
	for e := docker.containers.Front(); e != nil; e = e.Next() {
		container := e.Value.(*Container)
		if container.Id == id {
			return e
		}
	}
	return nil
}

func (docker *Docker) Get(id string) *Container {
	e := docker.getContainerElement(id)
	if e == nil {
		return nil
	}
	return e.Value.(*Container)
}

func (docker *Docker) Exists(id string) bool {
	return docker.Get(id) != nil
}

func (docker *Docker) Create(id string, command string, args []string, image *fs.Image, config *Config) (*Container, error) {
	if docker.Exists(id) {
		return nil, fmt.Errorf("Container %v already exists", id)
	}
	root := path.Join(docker.repository, id)

	container, err := createContainer(id, root, command, args, image, config, docker.networkManager)
	if err != nil {
		return nil, err
	}
	docker.containers.PushBack(container)
	return container, nil
}

func (docker *Docker) Destroy(container *Container) error {
	element := docker.getContainerElement(container.Id)
	if element == nil {
		return fmt.Errorf("Container %v not found - maybe it was already destroyed?", container.Id)
	}

	if err := container.Stop(); err != nil {
		return err
	}
	if container.Mountpoint.Mounted() {
		if err := container.Mountpoint.Umount(); err != nil {
			log.Printf("Unable to umount container %v: %v", container.Id, err)
		}

		if err := container.Mountpoint.Deregister(); err != nil {
			log.Printf("Unable to deregiser mountpoint %v: %v", container.Mountpoint.Root, err)
		}
	}
	if err := os.RemoveAll(container.Root); err != nil {
		log.Printf("Unable to remove filesystem for %v: %v", container.Id, err)
	}
	docker.containers.Remove(element)
	return nil
}

func (docker *Docker) restore() error {
	dir, err := ioutil.ReadDir(docker.repository)
	if err != nil {
		return err
	}
	for _, v := range dir {
		container, err := loadContainer(docker.Store, path.Join(docker.repository, v.Name()), docker.networkManager)
		if err != nil {
			log.Printf("Failed to load container %v: %v", v.Name(), err)
			continue
		}
		docker.containers.PushBack(container)
	}
	return nil
}

func New() (*Docker, error) {
	return NewFromDirectory("/var/lib/docker")
}

func NewFromDirectory(root string) (*Docker, error) {
	store, err := fs.New(path.Join(root, "images"))
	if err != nil {
		return nil, err
	}
	netManager, err := newNetworkManager(networkBridgeIface)
	if err != nil {
		return nil, err
	}

	docker := &Docker{
		root:           root,
		repository:     path.Join(root, "containers"),
		containers:     list.New(),
		Store:          store,
		networkManager: netManager,
	}

	if err := os.MkdirAll(docker.repository, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if err := docker.restore(); err != nil {
		return nil, err
	}
	return docker, nil
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
