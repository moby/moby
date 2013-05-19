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

type BuilderClient struct {
	builder *Builder
	cli     *DockerCli

	image      string
	maintainer string
	config     *Config

	tmpContainers map[string]struct{}
	tmpImages     map[string]struct{}

	needCommit bool
}

func (b *BuilderClient) clearTmp(containers, images map[string]struct{}) {
	for c := range containers {
		tmp := b.builder.runtime.Get(c)
		b.builder.runtime.Destroy(tmp)
		utils.Debugf("Removing container %s", c)
	}
	for i := range images {
		b.builder.runtime.graph.Delete(i)
		utils.Debugf("Removing image %s", i)
	}
}

func (b *BuilderClient) From(name string) error {
	obj, statusCode, err := b.cli.call("GET", "/images/"+name+"/json", nil)
	if statusCode == 404 {
		if err := b.cli.hijack("POST", "/images/create?fromImage="+name, false); err != nil {
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

	img := &ApiImages{}
	if err := json.Unmarshal(obj, img); err != nil {
		return err
	}
	b.image = img.Id
	return nil
}

func (b *BuilderClient) Maintainer(name string) error {
	b.needCommit = true
	b.maintainer = name
	return nil
}

func (b *BuilderClient) Run(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	config, _, err := ParseRun([]string{b.image, "/bin/sh", "-c", args}, b.builder.runtime.capabilities)
	if err != nil {
		return err
	}
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
		b.image = apiId.Id
		return nil
	}

	body, _, err = b.cli.call("POST", "/containers/create", b.config)
	if err != nil {
		return err
	}

	out := &ApiRun{}
	err = json.Unmarshal(body, out)
	if err != nil {
		return err
	}

	for _, warning := range out.Warnings {
		fmt.Fprintln(os.Stderr, "WARNING: ", warning)
	}

	//start the container
	_, _, err = b.cli.call("POST", "/containers/"+out.Id+"/start", nil)
	if err != nil {
		return err
	}
	b.tmpContainers[out.Id] = struct{}{}

	// Wait for it to finish
	_, _, err = b.cli.call("POST", "/containers/"+out.Id+"/wait", nil)
	if err != nil {
		return err
	}

	// Commit the container
	v := url.Values{}
	v.Set("container", out.Id)
	v.Set("author", b.maintainer)
	body, _, err = b.cli.call("POST", "/commit?"+v.Encode(), b.config)
	if err != nil {
		return err
	}
	apiId := &ApiId{}
	err = json.Unmarshal(body, apiId)
	if err != nil {
		return err
	}
	b.tmpImages[apiId.Id] = struct{}{}
	b.image = apiId.Id
	b.needCommit = false
	return nil
}

func (b *BuilderClient) Env(args string) error {
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

func (b *BuilderClient) Cmd(args string) error {
	b.needCommit = true
	b.config.Cmd = []string{"/bin/sh", "-c", args}
	return nil
}

func (b *BuilderClient) Expose(args string) error {
	ports := strings.Split(args, " ")
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return nil
}

func (b *BuilderClient) Insert(args string) error {
	// FIXME: Reimplement this once the remove_hijack branch gets merged.
	// We need to retrieve the resulting Id
	return fmt.Errorf("INSERT not implemented")
}

func NewBuilderClient(dockerfile io.Reader) (string, error) {
	//	defer b.clearTmp(tmpContainers, tmpImages)

	b := &BuilderClient{
		cli: NewDockerCli("0.0.0.0", 4243),
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

		fmt.Printf("%s %s\n", strings.ToUpper(instruction), arguments)

		method, exists := reflect.TypeOf(b).MethodByName(strings.ToUpper(instruction[:1]) + strings.ToLower(instruction[1:]))
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
		body, _, err = b.cli.call("POST", "/containers/create", b.config)
		if err != nil {
			return err
		}

		out := &ApiRun{}
		err = json.Unmarshal(body, out)
		if err != nil {
			return err
		}

		for _, warning := range out.Warnings {
			fmt.Fprintln(os.Stderr, "WARNING: ", warning)
		}

		//start the container
		_, _, err = b.cli.call("POST", "/containers/"+out.Id+"/start", nil)
		if err != nil {
			return err
		}
		b.tmpContainers[out.Id] = struct{}{}

		// Wait for it to finish
		_, _, err = b.cli.call("POST", "/containers/"+out.Id+"/wait", nil)
		if err != nil {
			return err
		}

		// Commit the container
		v := url.Values{}
		v.Set("container", out.Id)
		v.Set("author", b.maintainer)
		body, _, err = b.cli.call("POST", "/commit?"+v.Encode(), b.config)
		if err != nil {
			return err
		}
		apiId := &ApiId{}
		err = json.Unmarshal(body, apiId)
		if err != nil {
			return err
		}
		b.tmpImages[apiId.Id] = struct{}{}
		b.image = apiId.Id
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
