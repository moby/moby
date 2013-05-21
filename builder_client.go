package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"net/url"
	"os"
	"reflect"
	"strings"
)

type BuilderClient interface {
	Build(io.Reader) (string, error)
	CmdFrom(string) error
	CmdRun(string) error
}

type builderClient struct {
	cli *DockerCli

	image      string
	maintainer string
	config     *Config

	tmpContainers map[string]struct{}
	tmpImages     map[string]struct{}

	needCommit bool
}

func (b *builderClient) clearTmp(containers, images map[string]struct{}) {
	for c := range containers {
		if _, _, err := b.cli.call("DELETE", "/containers/"+c, nil); err != nil {
			utils.Debugf("%s", err)
		}
		utils.Debugf("Removing container %s", c)
	}
	for i := range images {
		if _, _, err := b.cli.call("DELETE", "/images/"+i, nil); err != nil {
			utils.Debugf("%s", err)
		}
		utils.Debugf("Removing image %s", i)
	}
}

func (b *builderClient) CmdFrom(name string) error {
	obj, statusCode, err := b.cli.call("GET", "/images/"+name+"/json", nil)
	if statusCode == 404 {

		remote := name
		var tag string
		if strings.Contains(remote, ":") {
			remoteParts := strings.Split(remote, ":")
			tag = remoteParts[1]
			remote = remoteParts[0]
		}
		var out io.Writer
		if os.Getenv("DEBUG") != "" {
			out = os.Stdout
		} else {
			out = &utils.NopWriter{}
		}
		if err := b.cli.stream("POST", "/images/create?fromImage="+remote+"&tag="+tag, nil, out); err != nil {
			return err
		}
		obj, _, err = b.cli.call("GET", "/images/"+name+"/json", nil)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	img := &ApiId{}
	if err := json.Unmarshal(obj, img); err != nil {
		return err
	}
	b.image = img.Id
	utils.Debugf("Using image %s", b.image)
	return nil
}

func (b *builderClient) CmdMaintainer(name string) error {
	b.needCommit = true
	b.maintainer = name
	return nil
}

func (b *builderClient) CmdRun(args string) error {
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

	body, statusCode, err := b.cli.call("POST", "/images/getCache", &ApiImageConfig{Id: b.image, Config: b.config})
	if err != nil {
		if statusCode != 404 {
			return err
		}
	}
	if statusCode != 404 {
		apiId := &ApiId{}
		if err := json.Unmarshal(body, apiId); err != nil {
			return err
		}
		utils.Debugf("Use cached version")
		b.image = apiId.Id
		return nil
	}
	cid, err := b.run()
	if err != nil {
		return err
	}
	b.config.Cmd, b.config.Env = cmd, env
	return b.commit(cid)
}

func (b *builderClient) CmdEnv(args string) error {
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

func (b *builderClient) CmdCmd(args string) error {
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

func (b *builderClient) CmdExpose(args string) error {
	ports := strings.Split(args, " ")
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return nil
}

func (b *builderClient) CmdInsert(args string) error {
	// FIXME: Reimplement this once the remove_hijack branch gets merged.
	// We need to retrieve the resulting Id
	return fmt.Errorf("INSERT not implemented")
}

func (b *builderClient) run() (string, error) {
	if b.image == "" {
		return "", fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	b.config.Image = b.image
	body, _, err := b.cli.call("POST", "/containers/create", b.config)
	if err != nil {
		return "", err
	}

	apiRun := &ApiRun{}
	if err := json.Unmarshal(body, apiRun); err != nil {
		return "", err
	}
	for _, warning := range apiRun.Warnings {
		fmt.Fprintln(os.Stderr, "WARNING: ", warning)
	}

	//start the container
	_, _, err = b.cli.call("POST", "/containers/"+apiRun.Id+"/start", nil)
	if err != nil {
		return "", err
	}
	b.tmpContainers[apiRun.Id] = struct{}{}

	// Wait for it to finish
	body, _, err = b.cli.call("POST", "/containers/"+apiRun.Id+"/wait", nil)
	if err != nil {
		return "", err
	}
	apiWait := &ApiWait{}
	if err := json.Unmarshal(body, apiWait); err != nil {
		return "", err
	}
	if apiWait.StatusCode != 0 {
		return "", fmt.Errorf("The command %v returned a non-zero code: %d", b.config.Cmd, apiWait.StatusCode)
	}

	return apiRun.Id, nil
}

func (b *builderClient) commit(id string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
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

	// Commit the container
	v := url.Values{}
	v.Set("container", id)
	v.Set("author", b.maintainer)

	body, _, err := b.cli.call("POST", "/commit?"+v.Encode(), b.config)
	if err != nil {
		return err
	}
	apiId := &ApiId{}
	if err := json.Unmarshal(body, apiId); err != nil {
		return err
	}
	b.tmpImages[apiId.Id] = struct{}{}
	b.image = apiId.Id
	b.needCommit = false
	return nil
}

func (b *builderClient) Build(dockerfile io.Reader) (string, error) {
	defer b.clearTmp(b.tmpContainers, b.tmpImages)
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

		fmt.Printf("%s %s (%s)\n", strings.ToUpper(instruction), arguments, b.image)

		method, exists := reflect.TypeOf(b).MethodByName("Cmd" + strings.ToUpper(instruction[:1]) + strings.ToLower(instruction[1:]))
		if !exists {
			fmt.Printf("Skipping unknown instruction %s\n", strings.ToUpper(instruction))
		}
		ret := method.Func.Call([]reflect.Value{reflect.ValueOf(b), reflect.ValueOf(arguments)})[0].Interface()
		if ret != nil {
			return "", ret.(error)
		}

		fmt.Printf("===> %v\n", b.image)
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
		for i := range b.tmpContainers {
			delete(b.tmpContainers, i)
		}
		fmt.Printf("Build finished. image id: %s\n", b.image)
		return b.image, nil
	}
	return "", fmt.Errorf("An error occured during the build\n")
}

func NewBuilderClient(addr string, port int) BuilderClient {
	return &builderClient{
		cli:           NewDockerCli(addr, port),
		config:        &Config{},
		tmpContainers: make(map[string]struct{}),
		tmpImages:     make(map[string]struct{}),
	}
}
