package docker

import (
	"fmt"
	"os"
	"path"
	"time"
)

type Builder struct {
	runtime      *Runtime
	repositories *TagStore
	graph        *Graph

	config *Config
	image  *Image
}

func NewBuilder(runtime *Runtime) *Builder {
	return &Builder{
		runtime:      runtime,
		graph:        runtime.graph,
		repositories: runtime.repositories,
	}
}

func (builder *Builder) Create(config *Config) (*Container, error) {
	// Lookup image
	img, err := builder.repositories.LookupImage(config.Image)
	if err != nil {
		return nil, err
	}

	if img.Config != nil {
		MergeConfig(config, img.Config)
	}

	if config.Cmd == nil || len(config.Cmd) == 0 {
		return nil, fmt.Errorf("No command specified")
	}

	// Generate id
	id := GenerateId()
	// Generate default hostname
	// FIXME: the lxc template no longer needs to set a default hostname
	if config.Hostname == "" {
		config.Hostname = id[:12]
	}

	container := &Container{
		// FIXME: we should generate the ID here instead of receiving it as an argument
		Id:              id,
		Created:         time.Now(),
		Path:            config.Cmd[0],
		Args:            config.Cmd[1:], //FIXME: de-duplicate from config
		Config:          config,
		Image:           img.Id, // Always use the resolved image id
		NetworkSettings: &NetworkSettings{},
		// FIXME: do we need to store this in the container?
		SysInitPath: sysInitPath,
	}
	container.root = builder.runtime.containerRoot(container.Id)
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return nil, err
	}

	// If custom dns exists, then create a resolv.conf for the container
	if len(config.Dns) > 0 {
		container.ResolvConfPath = path.Join(container.root, "resolv.conf")
		f, err := os.Create(container.ResolvConfPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		for _, dns := range config.Dns {
			if _, err := f.Write([]byte("nameserver " + dns + "\n")); err != nil {
				return nil, err
			}
		}
	} else {
		container.ResolvConfPath = "/etc/resolv.conf"
	}

	// Step 2: save the container json
	if err := container.ToDisk(); err != nil {
		return nil, err
	}
	// Step 3: register the container
	if err := builder.runtime.Register(container); err != nil {
		return nil, err
	}
	return container, nil
}

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository
func (builder *Builder) Commit(container *Container, repository, tag, comment, author string, config *Config) (*Image, error) {
	// FIXME: freeze the container before copying it to avoid data corruption?
	// FIXME: this shouldn't be in commands.
	rwTar, err := container.ExportRw()
	if err != nil {
		return nil, err
	}
	// Create a new image from the container's base layers + a new layer from container changes
	img, err := builder.graph.Create(rwTar, container, comment, author, config)
	if err != nil {
		return nil, err
	}
	// Register the image if needed
	if repository != "" {
		if err := builder.repositories.Set(repository, tag, img.Id, true); err != nil {
			return img, err
		}
	}
	return img, nil
}
