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

func (builder *Builder) run(image *Image, cmd string) (*Container, error) {
	// FIXME: pass a NopWriter instead of nil
	config, err := ParseRun([]string{"-d", image.Id, "/bin/sh", "-c", cmd}, nil, builder.runtime.capabilities)
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

func (builder *Builder) runCommit(image *Image, cmd string) (*Image, error) {
	c, err := builder.run(image, cmd)
	if err != nil {
		return nil, err
	}
	if result := c.Wait(); result != 0 {
		return nil, fmt.Errorf("!!! '%s' return non-zero exit code '%d'. Aborting.", cmd, result)
	}
	img, err := builder.runtime.Commit(c.Id, "", "", "", "")
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (builder *Builder) Build(dockerfile io.Reader, stdout io.Writer) error {
	var image, base *Image

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
			base, err = builder.runCommit(image, tmp[1])
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "===> %s\n", base.Id)
			break
		case "copy":
			return fmt.Errorf("The copy operator has not yet been implemented")
		default:
			fmt.Fprintf(stdout, "Skipping unknown op %s\n", tmp[0])
		}
	}
	if base != nil {
		fmt.Fprintf(stdout, "Build finished. image id: %s\n", base.Id)
	} else {
		fmt.Fprintf(stdout, "An error occured during the build\n")
	}
	return nil
}
