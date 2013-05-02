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
}

func NewBuilder(runtime *Runtime) *Builder {
	return &Builder{
		runtime:      runtime,
		repositories: runtime.repositories,
	}
}

func (builder *Builder) mergeConfig(userConf, imageConf *Config) {
	if userConf.Hostname != "" {
		userConf.Hostname = imageConf.Hostname
	}
	if userConf.User != "" {
		userConf.User = imageConf.User
	}
	if userConf.Memory == 0 {
		userConf.Memory = imageConf.Memory
	}
	if userConf.MemorySwap == 0 {
		userConf.MemorySwap = imageConf.MemorySwap
	}
	if userConf.PortSpecs == nil || len(userConf.PortSpecs) == 0 {
		userConf.PortSpecs = imageConf.PortSpecs
	}
	if !userConf.Tty {
		userConf.Tty = userConf.Tty
	}
	if !userConf.OpenStdin {
		userConf.OpenStdin = imageConf.OpenStdin
	}
	if !userConf.StdinOnce {
		userConf.StdinOnce = imageConf.StdinOnce
	}
	if userConf.Env == nil || len(userConf.Env) == 0 {
		userConf.Env = imageConf.Env
	}
	if userConf.Cmd == nil || len(userConf.Cmd) == 0 {
		userConf.Cmd = imageConf.Cmd
	}
	if userConf.Dns == nil || len(userConf.Dns) == 0 {
		userConf.Dns = imageConf.Dns
	}
}

func (builder *Builder) Create(config *Config) (*Container, error) {
	// Lookup image
	img, err := builder.repositories.LookupImage(config.Image)
	if err != nil {
		return nil, err
	}

	if img.Config != nil {
		builder.mergeConfig(config, img.Config)
	}

	if config.Cmd == nil {
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

func (builder *Builder) getCachedImage(image *Image, config *Config) (*Image, error) {
	// Retrieve all images
	images, err := builder.graph.All()
	if err != nil {
		return nil, err
	}

	// Store the tree in a map of map (map[parentId][childId])
	imageMap := make(map[string]map[string]struct{})
	for _, img := range images {
		if _, exists := imageMap[img.Parent]; !exists {
			imageMap[img.Parent] = make(map[string]struct{})
		}
		imageMap[img.Parent][img.Id] = struct{}{}
	}

	// Loop on the children of the given image and check the config
	for elem := range imageMap[image.Id] {
		img, err := builder.graph.Get(elem)
		if err != nil {
			return nil, err
		}
		if CompareConfig(&img.ContainerConfig, config) {
			return img, nil
		}
	}
	return nil, nil
}

func (builder *Builder) Build(dockerfile io.Reader, stdout io.Writer) (*Image, error) {
	var (
		image, base   *Image
		maintainer    string
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
			return err
		}
		line = strings.TrimSpace(line)
		// Skip comments and empty line
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		tmp := strings.SplitN(line, "	", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("Invalid Dockerfile format")
		}
		switch tmp[0] {
		case "from":
			fmt.Fprintf(stdout, "FROM %s\n", tmp[1])
			image, err = builder.runtime.repositories.LookupImage(tmp[1])
			if err != nil {
				if builder.runtime.graph.IsNotExist(err) {

					var tag, remote string
					if strings.Contains(remote, ":") {
						remoteParts := strings.Split(remote, ":")
						tag = remoteParts[1]
						remote = remoteParts[0]
					}

					if err := builder.runtime.graph.PullRepository(stdout, remote, tag, builder.runtime.repositories, builder.runtime.authConfig); err != nil {
						return nil, err
					}

					image, err = builder.runtime.repositories.LookupImage(arguments)
					if err != nil {
						return nil, err
					}

				} else {
					return nil, err
				}
			}
			break
		case "run":
			fmt.Fprintf(stdout, "RUN %s\n", tmp[1])
			if image == nil {
				return fmt.Errorf("Please provide a source image with `from` prior to run")
			}
			config, err := ParseRun([]string{image.Id, "/bin/sh", "-c", tmp[1]}, nil, builder.runtime.capabilities)
			if err != nil {
				return err
			}

			if cache, err := builder.getCachedImage(image, config); err != nil {
				return nil, err
			} else if cache != nil {
				image = cache
				fmt.Fprintf(stdout, "===> %s\n", image.ShortId())
				break
			}

			// Create the container and start it
			c, err := builder.Create(config)
			if err != nil {
				return err
			}
			if err := c.Start(); err != nil {
				return err
			}
			tmpContainers[c.Id] = struct{}{}

			// Wait for it to finish
			if result := c.Wait(); result != 0 {
				return fmt.Errorf("!!! '%s' return non-zero exit code '%d'. Aborting.", tmp[1], result)
			}

			// Commit the container
			base, err = builder.Commit(c, "", "", "", "", nil)
			if err != nil {
				return err
			}
			tmpImages[base.Id] = struct{}{}

			fmt.Fprintf(stdout, "===> %s\n", base.ShortId())
			break
		case "copy":
			if image == nil {
				return fmt.Errorf("Please provide a source image with `from` prior to copy")
			}
			tmp2 := strings.SplitN(tmp[1], " ", 2)
			if len(tmp) != 2 {
				return fmt.Errorf("Invalid COPY format")
			}
			fmt.Fprintf(stdout, "COPY %s to %s in %s\n", tmp2[0], tmp2[1], base.ShortId())

			file, err := Download(tmp2[0], stdout)
			if err != nil {
				return err
			}
			defer file.Body.Close()

			config, err := ParseRun([]string{base.Id, "echo", "insert", tmp2[0], tmp2[1]}, nil, builder.runtime.capabilities)
			if err != nil {
				return err
			}
			c, err := builder.Create(config)
			if err != nil {
				return err
			}

			if err := c.Start(); err != nil {
				return err
			}

			// Wait for echo to finish
			if result := c.Wait(); result != 0 {
				return fmt.Errorf("!!! '%s' return non-zero exit code '%d'. Aborting.", tmp[1], result)
			}

			if err := c.Inject(file.Body, tmp2[1]); err != nil {
				return err
			}

			base, err = builder.Commit(c, "", "", "", "", nil)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "===> %s\n", base.ShortId())
			break
		default:
			fmt.Fprintf(stdout, "Skipping unknown op %s\n", tmp[0])
		}
	}
	if image != nil {
		// The build is successful, keep the temporary containers and images
		for i := range tmpImages {
			delete(tmpImages, i)
		}
		for i := range tmpContainers {
			delete(tmpContainers, i)
		}
		fmt.Fprintf(stdout, "Build finished. image id: %s\n", image.ShortId())
		return image, nil
	}
	return nil, fmt.Errorf("An error occured during the build\n")
}
