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

	needCommit bool

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
	b.needCommit = true
	b.maintainer = name
	return nil
}

func (b *buildFile) CmdRun(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	config, _, err := ParseRun([]string{b.image, "/bin/sh", "-c", args}, nil)
	if err != nil {
		return err
	}

	cmd, env := b.config.Cmd, b.config.Env
	b.config.Cmd = nil
	MergeConfig(b.config, config)

	if cache, err := b.srv.ImageGetCached(b.image, config); err != nil {
		return err
	} else if cache != nil {
		utils.Debugf("Use cached version")
		b.image = cache.Id
		return nil
	}

	cid, err := b.run()
	if err != nil {
		return err
	}
	b.config.Cmd, b.config.Env = cmd, env
	return b.commit(cid)
}

func (b *buildFile) CmdEnv(args string) error {
	b.needCommit = true
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
	return nil
}

func (b *buildFile) CmdCmd(args string) error {
	b.needCommit = true
	var cmd []string
	if err := json.Unmarshal([]byte(args), &cmd); err != nil {
		utils.Debugf("Error unmarshalling: %s, using /bin/sh -c", err)
		b.config.Cmd = []string{"/bin/sh", "-c", args}
	} else {
		b.config.Cmd = cmd
	}
	return nil
}

func (b *buildFile) CmdExpose(args string) error {
	ports := strings.Split(args, " ")
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return nil
}

func (b *buildFile) CmdInsert(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to insert")
	}
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid INSERT format")
	}
	sourceUrl := strings.Trim(tmp[0], " ")
	destPath := strings.Trim(tmp[1], " ")

	file, err := utils.Download(sourceUrl, b.out)
	if err != nil {
		return err
	}
	defer file.Body.Close()

	b.config.Cmd = []string{"echo", "INSERT", sourceUrl, "in", destPath}
	cid, err := b.run()
	if err != nil {
		return err
	}

	container := b.runtime.Get(cid)
	if container == nil {
		return fmt.Errorf("An error occured while creating the container")
	}

	if err := container.Inject(file.Body, destPath); err != nil {
		return err
	}

	return b.commit(cid)
}

func (b *buildFile) CmdAdd(args string) error {
	if b.context == "" {
		return fmt.Errorf("No context given. Impossible to use ADD")
	}
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid INSERT format")
	}
	orig := strings.Trim(tmp[0], " ")
	dest := strings.Trim(tmp[1], " ")

	b.config.Cmd = []string{"echo", "PUSH", orig, "in", dest}
	cid, err := b.run()
	if err != nil {
		return err
	}

	container := b.runtime.Get(cid)
	if container == nil {
		return fmt.Errorf("Error while creating the container (CmdAdd)")
	}

	if err := os.MkdirAll(path.Join(container.rwPath(), dest), 0700); err != nil {
		return err
	}

	if err := utils.CopyDirectory(path.Join(b.context, orig), path.Join(container.rwPath(), dest)); err != nil {
		return err
	}

	return b.commit(cid)
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

func (b *buildFile) commit(id string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to commit")
	}
	b.config.Image = b.image
	if id == "" {
		cmd := b.config.Cmd
		b.config.Cmd = []string{"true"}
		if cid, err := b.run(); err != nil {
			return err
		} else {
			id = cid
		}
		b.config.Cmd = cmd
	}

	container := b.runtime.Get(id)
	if container == nil {
		return fmt.Errorf("An error occured while creating the container")
	}

	// Commit the container
	image, err := b.builder.Commit(container, "", "", "", b.maintainer, nil)
	if err != nil {
		return err
	}
	b.tmpImages[image.Id] = struct{}{}
	b.image = image.Id
	b.needCommit = false
	return nil
}

func (b *buildFile) Build(dockerfile, context io.Reader) (string, error) {
	defer b.clearTmp(b.tmpContainers, b.tmpImages)

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
		}
		ret := method.Func.Call([]reflect.Value{reflect.ValueOf(b), reflect.ValueOf(arguments)})[0].Interface()
		if ret != nil {
			return "", ret.(error)
		}

		fmt.Fprintf(b.out, "===> %v\n", b.image)
	}
	if b.needCommit {
		if err := b.commit(""); err != nil {
			return "", err
		}
	}
	if b.image != "" {
		// The build is successful, keep the temporary containers and images
		for i := range b.tmpImages {
			delete(b.tmpImages, i)
		}
		fmt.Fprintf(b.out, "Build finished. image id: %s\n", b.image)
		return b.image, nil
	}
	for i := range b.tmpContainers {
		delete(b.tmpContainers, i)
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
