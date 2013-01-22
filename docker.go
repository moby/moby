package docker

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type Docker struct {
	root       string
	repository string
	containers *list.List
}

func (docker *Docker) List() []*Container {
	containers := []*Container{}
	for e := docker.containers.Front(); e != nil; e = e.Next() {
		containers = append(containers, e.Value.(*Container))
	}
	return containers
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

func (docker *Docker) Create(id string, command string, args []string, layers []string, config *Config) (*Container, error) {
	if docker.Exists(id) {
		return nil, fmt.Errorf("Container %v already exists", id)
	}
	root := path.Join(docker.repository, id)
	container, err := createContainer(id, root, command, args, layers, config)
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
	if err := os.RemoveAll(container.Root); err != nil {
		return err
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
		container, err := loadContainer(path.Join(docker.repository, v.Name()))
		if err != nil {
			fmt.Errorf("Failed to load %v: %v", v.Name(), err)
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
	docker := &Docker{
		root:       root,
		repository: path.Join(root, "containers"),
		containers: list.New(),
	}

	if err := os.Mkdir(docker.repository, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if err := docker.restore(); err != nil {
		return nil, err
	}
	return docker, nil
}
