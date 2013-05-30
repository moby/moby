package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
)

type BuildFile interface {
	Build(io.Reader, io.Reader) (string, error)
	CmdFrom(string) error
	CmdRun(string) error
}

type buildFile struct {
	runtime *Runtime
	builder *Builder
	srv     *Server

	image      string
	maintainer string
	config     *Config
	context    string

	tmpContainers map[string]struct{}
	tmpImages     map[string]struct{}

	out io.Writer
}

func (b *buildFile) clearTmp(containers, images map[string]struct{}) {
	for c := range containers {
		tmp := b.runtime.Get(c)
		b.runtime.Destroy(tmp)
		utils.Debugf("Removing container %s", c)
	}
	for i := range images {
		b.runtime.graph.Delete(i)
		utils.Debugf("Removing image %s", i)
	}
}

func (b *buildFile) CmdFrom(name string) error {
	image, err := b.runtime.repositories.LookupImage(name)
	if err != nil {
		if b.runtime.graph.IsNotExist(err) {

			var tag, remote string
			if strings.Contains(name, ":") {
				remoteParts := strings.Split(name, ":")
				tag = remoteParts[1]
				remote = remoteParts[0]
			} else {
				remote = name
			}

			if err := b.srv.ImagePull(remote, tag, "", b.out, false); err != nil {
				return err
			}

			image, err = b.runtime.repositories.LookupImage(name)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	b.image = image.Id
	b.config = &Config{}
	return nil
}

func (b *buildFile) CmdMaintainer(name string) error {
	b.maintainer = name
	return b.commit("", b.config.Cmd, fmt.Sprintf("MAINTAINER %s", name))
}

func (b *buildFile) CmdRun(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	config, _, err := ParseRun([]string{b.image, "/bin/sh", "-c", args}, nil)
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = nil
	MergeConfig(b.config, config)

	utils.Debugf("Command to be executed: %v", b.config.Cmd)

	if cache, err := b.srv.ImageGetCached(b.image, b.config); err != nil {
		return err
	} else if cache != nil {
		utils.Debugf("[BUILDER] Use cached version")
		b.image = cache.Id
		return nil
	} else {
		utils.Debugf("[BUILDER] Cache miss")
	}

	cid, err := b.run()
	if err != nil {
		return err
	}
	if err := b.commit(cid, cmd, "run"); err != nil {
		return err
	}
	b.config.Cmd = cmd
	return nil
}

func (b *buildFile) CmdEnv(args string) error {
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ENV format")
	}
	key := strings.Trim(tmp[0], " ")
	value := strings.Trim(tmp[1], " ")

	for i, elem := range b.config.Env {
		if strings.HasPrefix(elem, key+"=") {
			b.config.Env[i] = key + "=" + value
			return nil
		}
	}
	b.config.Env = append(b.config.Env, key+"="+value)
	return b.commit("", b.config.Cmd, fmt.Sprintf("ENV %s=%s", key, value))
}

func (b *buildFile) CmdCmd(args string) error {
	var cmd []string
	if err := json.Unmarshal([]byte(args), &cmd); err != nil {
		utils.Debugf("Error unmarshalling: %s, using /bin/sh -c", err)
		cmd = []string{"/bin/sh", "-c", args}
	}
	if err := b.commit("", cmd, fmt.Sprintf("CMD %v", cmd)); err != nil {
		return err
	}
	b.config.Cmd = cmd
	return nil
}

func (b *buildFile) CmdExpose(args string) error {
	ports := strings.Split(args, " ")
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return b.commit("", b.config.Cmd, fmt.Sprintf("EXPOSE %v", ports))
}

func (b *buildFile) CmdInsert(args string) error {
	return fmt.Errorf("INSERT has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdCopy(args string) error {
	return fmt.Errorf("COPY has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdAdd(args string) error {
	if b.context == "" {
		return fmt.Errorf("No context given. Impossible to use ADD")
	}
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ADD format")
	}
	orig := strings.Trim(tmp[0], " ")
	dest := strings.Trim(tmp[1], " ")

	cmd := b.config.Cmd
	b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) ADD %s in %s", orig, dest)}
	cid, err := b.run()
	if err != nil {
		return err
	}

	container := b.runtime.Get(cid)
	if container == nil {
		return fmt.Errorf("Error while creating the container (CmdAdd)")
	}
	if err := container.EnsureMounted(); err != nil {
		return err
	}
	defer container.Unmount()

	origPath := path.Join(b.context, orig)
	destPath := path.Join(container.RootfsPath(), dest)

	fi, err := os.Stat(origPath)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.MkdirAll(destPath, 0700); err != nil {
			return err
		}

		files, err := ioutil.ReadDir(path.Join(b.context, orig))
		if err != nil {
			return err
		}
		for _, fi := range files {
			if err := utils.CopyDirectory(path.Join(origPath, fi.Name()), path.Join(destPath, fi.Name())); err != nil {
				return err
			}
		}
	} else {
		if err := os.MkdirAll(path.Dir(destPath), 0700); err != nil {
			return err
		}
		if err := utils.CopyDirectory(origPath, destPath); err != nil {
			return err
		}
	}
	if err := b.commit(cid, cmd, fmt.Sprintf("ADD %s in %s", orig, dest)); err != nil {
		return err
	}
	b.config.Cmd = cmd
	return nil
}

func (b *buildFile) run() (string, error) {
	if b.image == "" {
		return "", fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	b.config.Image = b.image

	// Create the container and start it
	c, err := b.builder.Create(b.config)
	if err != nil {
		return "", err
	}
	b.tmpContainers[c.Id] = struct{}{}

	//start the container
	if err := c.Start(); err != nil {
		return "", err
	}

	// Wait for it to finish
	if ret := c.Wait(); ret != 0 {
		return "", fmt.Errorf("The command %v returned a non-zero code: %d", b.config.Cmd, ret)
	}

	return c.Id, nil
}

// Commit the container <id> with the autorun command <autoCmd>
func (b *buildFile) commit(id string, autoCmd []string, comment string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to commit")
	}
	b.config.Image = b.image
	if id == "" {
		b.config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + comment}

		if cache, err := b.srv.ImageGetCached(b.image, b.config); err != nil {
			return err
		} else if cache != nil {
			utils.Debugf("[BUILDER] Use cached version")
			b.image = cache.Id
			return nil
		} else {
			utils.Debugf("[BUILDER] Cache miss")
		}

		if cid, err := b.run(); err != nil {
			return err
		} else {
			id = cid
		}
	}

	container := b.runtime.Get(id)
	if container == nil {
		return fmt.Errorf("An error occured while creating the container")
	}

	// Note: Actually copy the struct
	autoConfig := *b.config
	autoConfig.Cmd = autoCmd
	// Commit the container
	image, err := b.builder.Commit(container, "", "", "", b.maintainer, &autoConfig)
	if err != nil {
		return err
	}
	b.tmpImages[image.Id] = struct{}{}
	b.image = image.Id
	return nil
}

func (b *buildFile) Build(dockerfile, context io.Reader) (string, error) {
	if context != nil {
		name, err := ioutil.TempDir("/tmp", "docker-build")
		if err != nil {
			return "", err
		}
		if err := Untar(context, name); err != nil {
			return "", err
		}
		defer os.RemoveAll(name)
		b.context = name
	}
	file := bufio.NewReader(dockerfile)
	for {
		line, err := file.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		line = strings.Replace(strings.TrimSpace(line), "	", " ", 1)
		// Skip comments and empty line
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		tmp := strings.SplitN(line, " ", 2)
		if len(tmp) != 2 {
			return "", fmt.Errorf("Invalid Dockerfile format")
		}
		instruction := strings.ToLower(strings.Trim(tmp[0], " "))
		arguments := strings.Trim(tmp[1], " ")

		fmt.Fprintf(b.out, "%s %s (%s)\n", strings.ToUpper(instruction), arguments, b.image)

		method, exists := reflect.TypeOf(b).MethodByName("Cmd" + strings.ToUpper(instruction[:1]) + strings.ToLower(instruction[1:]))
		if !exists {
			fmt.Fprintf(b.out, "Skipping unknown instruction %s\n", strings.ToUpper(instruction))
			continue
		}
		ret := method.Func.Call([]reflect.Value{reflect.ValueOf(b), reflect.ValueOf(arguments)})[0].Interface()
		if ret != nil {
			return "", ret.(error)
		}

		fmt.Fprintf(b.out, "===> %v\n", b.image)
	}
	if b.image != "" {
		fmt.Fprintf(b.out, "Build successful.\n===> %s\n", b.image)
		return b.image, nil
	}
	return "", fmt.Errorf("An error occured during the build\n")
}

func NewBuildFile(srv *Server, out io.Writer) BuildFile {
	return &buildFile{
		builder:       NewBuilder(srv.runtime),
		runtime:       srv.runtime,
		srv:           srv,
		config:        &Config{},
		out:           out,
		tmpContainers: make(map[string]struct{}),
		tmpImages:     make(map[string]struct{}),
	}
}
