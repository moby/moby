package docker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

type Builder struct {
	runtime      *Runtime
	repositories *TagStore
	graph        *Graph
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
func (builder *Builder) Commit(container *Container, repository, tag, comment, author string) (*Image, error) {
	// FIXME: freeze the container before copying it to avoid data corruption?
	// FIXME: this shouldn't be in commands.
	rwTar, err := container.ExportRw()
	if err != nil {
		return nil, err
	}
	// Create a new image from the container's base layers + a new layer from container changes
	img, err := builder.graph.Create(rwTar, container, comment, author)
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

func (builder *Builder) clearTmp(containers, images map[string]struct{}) {
	for c := range containers {
		tmp := builder.runtime.Get(c)
		builder.runtime.Destroy(tmp)
		Debugf("Removing container %s", c)
	}
	for i := range images {
		builder.runtime.graph.Delete(i)
		Debugf("Removing image %s", i)
	}
}

func (builder *Builder) Build(dockerfile io.Reader, stdout io.Writer) (*Image, error) {
	var (
		image, base   *Image
		tmpContainers map[string]struct{} = make(map[string]struct{})
		tmpImages     map[string]struct{} = make(map[string]struct{})
	)
	defer builder.clearTmp(tmpContainers, tmpImages)

	file := bufio.NewReader(dockerfile)
	for {
		line, err := file.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line = strings.TrimSpace(line)
		// Skip comments and empty line
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		tmp := strings.SplitN(line, "	", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("Invalid Dockerfile format")
		}
		switch tmp[0] {
		case "from":
			fmt.Fprintf(stdout, "FROM %s\n", tmp[1])
			image, err = builder.runtime.repositories.LookupImage(tmp[1])
			if err != nil {
				return nil, err
			}
			break
		case "run":
			fmt.Fprintf(stdout, "RUN %s\n", tmp[1])
			if image == nil {
				return nil, fmt.Errorf("Please provide a source image with `from` prior to run")
			}
			config, err := ParseRun([]string{image.Id, "/bin/sh", "-c", tmp[1]}, nil, builder.runtime.capabilities)
			if err != nil {
				return nil, err
			}

			// Create the container and start it
			c, err := builder.Create(config)
			if err != nil {
				return nil, err
			}
			if err := c.Start(); err != nil {
				return nil, err
			}
			tmpContainers[c.Id] = struct{}{}

			// Wait for it to finish
			if result := c.Wait(); result != 0 {
				return nil, fmt.Errorf("!!! '%s' return non-zero exit code '%d'. Aborting.", tmp[1], result)
			}

			// Commit the container
			base, err = builder.Commit(c, "", "", "", "")
			if err != nil {
				return nil, err
			}
			tmpImages[base.Id] = struct{}{}

			fmt.Fprintf(stdout, "===> %s\n", base.ShortId())

			// use the base as the new image
			image = base

			break
		case "copy":
			if image == nil {
				return nil, fmt.Errorf("Please provide a source image with `from` prior to copy")
			}
			tmp2 := strings.SplitN(tmp[1], " ", 2)
			if len(tmp) != 2 {
				return nil, fmt.Errorf("Invalid COPY format")
			}
			fmt.Fprintf(stdout, "COPY %s to %s in %s\n", tmp2[0], tmp2[1], base.ShortId())

			file, err := Download(tmp2[0], stdout)
			if err != nil {
				return nil, err
			}
			defer file.Body.Close()

			config, err := ParseRun([]string{base.Id, "echo", "insert", tmp2[0], tmp2[1]}, nil, builder.runtime.capabilities)
			if err != nil {
				return nil, err
			}
			c, err := builder.Create(config)
			if err != nil {
				return nil, err
			}

			if err := c.Start(); err != nil {
				return nil, err
			}

			// Wait for echo to finish
			if result := c.Wait(); result != 0 {
				return nil, fmt.Errorf("!!! '%s' return non-zero exit code '%d'. Aborting.", tmp[1], result)
			}

			if err := c.Inject(file.Body, tmp2[1]); err != nil {
				return nil, err
			}

			base, err = builder.Commit(c, "", "", "", "")
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(stdout, "===> %s\n", base.ShortId())

			image = base

			break
		default:
			fmt.Fprintf(stdout, "Skipping unknown op %s\n", tmp[0])
		}
	}
	if base != nil {
		// The build is successful, keep the temporary containers and images
		for i := range tmpImages {
			delete(tmpImages, i)
		}
		for i := range tmpContainers {
			delete(tmpContainers, i)
		}
		fmt.Fprintf(stdout, "Build finished. image id: %s\n", base.ShortId())
	} else {
		fmt.Fprintf(stdout, "An error occured during the build\n")
	}
	return base, nil
}
