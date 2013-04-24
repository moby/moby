package docker

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Builder struct {
	runtime *Runtime
}

func NewBuilder(runtime *Runtime) *Builder {
	return &Builder{
		runtime: runtime,
	}
}

func (builder *Builder) Run(image *Image, cmd ...string) (*Container, error) {
	// FIXME: pass a NopWriter instead of nil
	config, err := ParseRun(append([]string{"-d", image.Id}, cmd...), nil, builder.runtime.capabilities)
	if config.Image == "" {
		return nil, fmt.Errorf("Image not specified")
	}
	if len(config.Cmd) == 0 {
		return nil, fmt.Errorf("Command not specified")
	}
	if config.Tty {
		return nil, fmt.Errorf("The tty mode is not supported within the builder")
	}

	// Create new container
	container, err := builder.runtime.Create(config)
	if err != nil {
		return nil, err
	}
	if err := container.Start(); err != nil {
		return nil, err
	}
	return container, nil
}

func (builder *Builder) Commit(container *Container, repository, tag, comment, author string) (*Image, error) {
	return builder.runtime.Commit(container.Id, repository, tag, comment, author)
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

func (builder *Builder) Build(dockerfile io.Reader, stdout io.Writer) error {
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
				return err
			}
			break
		case "run":
			fmt.Fprintf(stdout, "RUN %s\n", tmp[1])
			if image == nil {
				return fmt.Errorf("Please provide a source image with `from` prior to run")
			}

			// Create the container and start it
			c, err := builder.Run(image, "/bin/sh", "-c", tmp[1])
			if err != nil {
				return err
			}
			tmpContainers[c.Id] = struct{}{}

			// Wait for it to finish
			if result := c.Wait(); result != 0 {
				return fmt.Errorf("!!! '%s' return non-zero exit code '%d'. Aborting.", tmp[1], result)
			}

			// Commit the container
			base, err = builder.Commit(c, "", "", "", "")
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

			c, err := builder.Run(base, "echo", "insert", tmp2[0], tmp2[1])
			if err != nil {
				return err
			}

			if err := c.Inject(file.Body, tmp2[1]); err != nil {
				return err
			}

			base, err = builder.Commit(c, "", "", "", "")
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "===> %s\n", base.ShortId())
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
	return nil
}
