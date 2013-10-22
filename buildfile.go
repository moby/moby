package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type BuildFile interface {
	Build(io.Reader) (string, error)
	CmdFrom(*fromArgs) error
	CmdRun(*runArgs) error
}

type buildFile struct {
	runtime *Runtime
	srv     *Server

	image        string
	maintainer   string
	config       *Config
	context      string
	verbose      bool
	utilizeCache bool
	rm           bool

	tmpContainers map[string]struct{}
	tmpImages     map[string]struct{}

	out io.Writer
}

func (b *buildFile) clearTmp(containers map[string]struct{}) {
	for c := range containers {
		tmp := b.runtime.Get(c)
		b.runtime.Destroy(tmp)
		fmt.Fprintf(b.out, "Removing intermediate container %s\n", utils.TruncateID(c))
	}
}

func (b *buildFile) CmdFrom(args *fromArgs) error {
	image, err := b.runtime.repositories.LookupImage(args.name)
	if err != nil {
		if b.runtime.graph.IsNotExist(err) {
			remote, tag := utils.ParseRepositoryTag(args.name)
			if err := b.srv.ImagePull(remote, tag, b.out, utils.NewStreamFormatter(false), nil, nil, true); err != nil {
				return err
			}
			image, err = b.runtime.repositories.LookupImage(args.name)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	b.image = image.ID
	b.config = &Config{}
	if b.config.Env == nil || len(b.config.Env) == 0 {
		b.config.Env = append(b.config.Env, "HOME=/", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}
	return nil
}

func (b *buildFile) CmdMaintainer(args *maintainerArgs) error {
	b.maintainer = args.name
	return b.commit("", b.config.Cmd, args.String())
}

func (b *buildFile) CmdRun(args *runArgs) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	config, _, _, err := ParseRun(append([]string{b.image}, args.cmd...), nil)
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = nil
	MergeConfig(b.config, config)

	defer func(cmd []string) { b.config.Cmd = cmd }(cmd)

	utils.Debugf("Command to be executed: %v", b.config.Cmd)

	if b.utilizeCache {
		if cache, err := b.srv.ImageGetCached(b.image, b.config); err != nil {
			return err
		} else if cache != nil {
			fmt.Fprintf(b.out, " ---> Using cache\n")
			utils.Debugf("[BUILDER] Use cached version")
			b.image = cache.ID
			return nil
		} else {
			utils.Debugf("[BUILDER] Cache miss")
		}
	}

	cid, err := b.run()
	if err != nil {
		return err
	}
	if err := b.commit(cid, cmd, "run"); err != nil {
		return err
	}

	return nil
}

func (b *buildFile) FindEnvKey(key string) int {
	for k, envVar := range b.config.Env {
		envParts := strings.SplitN(envVar, "=", 2)
		if key == envParts[0] {
			return k
		}
	}
	return -1
}

func (b *buildFile) ReplaceEnvMatches(value string) (string, error) {
	exp, err := regexp.Compile("(\\\\\\\\+|[^\\\\]|\\b|\\A)\\$({?)([[:alnum:]_]+)(}?)")
	if err != nil {
		return value, err
	}
	matches := exp.FindAllString(value, -1)
	for _, match := range matches {
		match = match[strings.Index(match, "$"):]
		matchKey := strings.Trim(match, "${}")

		for _, envVar := range b.config.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			envKey := envParts[0]
			envValue := envParts[1]

			if envKey == matchKey {
				value = strings.Replace(value, match, envValue, -1)
				break
			}
		}
	}
	return value, nil
}

func (b *buildFile) CmdEnv(args *envArgs) error {
	envKey := b.FindEnvKey(args.key)
	replacedValue, err := b.ReplaceEnvMatches(args.value)
	if err != nil {
		return err
	}
	replacedVar := fmt.Sprintf("%s=%s", args.key, replacedValue)

	if envKey >= 0 {
		b.config.Env[envKey] = replacedVar
	} else {
		b.config.Env = append(b.config.Env, replacedVar)
	}
	return b.commit("", b.config.Cmd, fmt.Sprintf("ENV %s", replacedVar))
}

func (b *buildFile) CmdCmd(args *cmdArgs) error {
	if err := b.commit("", args.cmd, args.String()); err != nil {
		return err
	}
	b.config.Cmd = args.cmd
	return nil
}

func (b *buildFile) CmdExpose(args *exposeArgs) error {
	ports := make([]string, len(args.ports))
	for i := 0; i < len(args.ports); i++ {
		ports[i] = strconv.Itoa(args.ports[i])
	}
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return b.commit("", b.config.Cmd, args.String())
}

func (b *buildFile) CmdUser(args *userArgs) error {
	b.config.User = args.name
	return b.commit("", b.config.Cmd, args.String())
}

func (b *buildFile) CmdInsert(args string) error {
	return fmt.Errorf("INSERT has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdCopy(args string) error {
	return fmt.Errorf("COPY has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdEntrypoint(args *entryPointArgs) error {
	if len(args.cmd) == 0 {
		return fmt.Errorf("Entrypoint cannot be empty")
	}
	b.config.Entrypoint = args.cmd
	if err := b.commit("", b.config.Cmd, args.String()); err != nil {
		return err
	}
	return nil
}

func (b *buildFile) CmdWorkdir(args *workDirArgs) error {
	b.config.WorkingDir = args.path
	return b.commit("", b.config.Cmd, args.String())
}

func (b *buildFile) CmdVolume(args *volumeArgs) error {
	if len(args.volumes) == 0 {
		return fmt.Errorf("Volume cannot be empty")
	}
	if b.config.Volumes == nil {
		b.config.Volumes = NewPathOpts()
	}
	for _, v := range args.volumes {
		b.config.Volumes[v] = struct{}{}
	}
	if err := b.commit("", b.config.Cmd, args.String()); err != nil {
		return err
	}
	return nil
}

func (b *buildFile) addRemote(container *Container, orig, dest string) error {
	file, err := utils.Download(orig, ioutil.Discard)
	if err != nil {
		return err
	}
	defer file.Body.Close()

	// If the destination is a directory, figure out the filename.
	if strings.HasSuffix(dest, "/") {
		u, err := url.Parse(orig)
		if err != nil {
			return err
		}
		path := u.Path
		if strings.HasSuffix(path, "/") {
			path = path[:len(path)-1]
		}
		parts := strings.Split(path, "/")
		filename := parts[len(parts)-1]
		if filename == "" {
			return fmt.Errorf("cannot determine filename from url: %s", u)
		}
		dest = dest + filename
	}

	return container.Inject(file.Body, dest)
}

func (b *buildFile) addContext(container *Container, orig, dest string) error {
	origPath := path.Join(b.context, orig)
	destPath := path.Join(container.RootfsPath(), dest)
	// Preserve the trailing '/'
	if strings.HasSuffix(dest, "/") {
		destPath = destPath + "/"
	}
	if !strings.HasPrefix(origPath, b.context) {
		return fmt.Errorf("Forbidden path: %s", origPath)
	}
	fi, err := os.Stat(origPath)
	if err != nil {
		return fmt.Errorf("%s: no such file or directory", orig)
	}
	if fi.IsDir() {
		if err := CopyWithTar(origPath, destPath); err != nil {
			return err
		}
		// First try to unpack the source as an archive
	} else if err := UntarPath(origPath, destPath); err != nil {
		utils.Debugf("Couldn't untar %s to %s: %s", origPath, destPath, err)
		// If that fails, just copy it as a regular file
		if err := os.MkdirAll(path.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := CopyWithTar(origPath, destPath); err != nil {
			return err
		}
	}
	return nil
}

func (b *buildFile) CmdAdd(args *addArgs) error {
	if b.context == "" {
		return fmt.Errorf("No context given. Impossible to use ADD")
	}
	orig, err := b.ReplaceEnvMatches(args.src)
	if err != nil {
		return err
	}
	dest, err := b.ReplaceEnvMatches(args.dst)
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) ADD %s in %s", orig, dest)}

	b.config.Image = b.image
	// Create the container and start it
	container, err := b.runtime.Create(b.config)
	if err != nil {
		return err
	}
	b.tmpContainers[container.ID] = struct{}{}

	if err := container.EnsureMounted(); err != nil {
		return err
	}
	defer container.Unmount()

	if utils.IsURL(orig) {
		if err := b.addRemote(container, orig, dest); err != nil {
			return err
		}
	} else {
		if err := b.addContext(container, orig, dest); err != nil {
			return err
		}
	}

	if err := b.commit(container.ID, cmd, args.String()); err != nil {
		return err
	}
	b.config.Cmd = cmd
	return nil
}

func (b *buildFile) CmdInclude(args *includeArgs) error {
	if args.filename == "" {
		return fmt.Errorf("Missing file name to include")
	}
	if err := b.processFile(args.filename); err != nil {
		return err
	}
	if err := b.commit("", b.config.Cmd, args.String()); err != nil {
		return err
	}
	return nil
}

func (b *buildFile) run() (string, error) {
	if b.image == "" {
		return "", fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	b.config.Image = b.image

	// Create the container and start it
	c, err := b.runtime.Create(b.config)
	if err != nil {
		return "", err
	}
	b.tmpContainers[c.ID] = struct{}{}
	fmt.Fprintf(b.out, " ---> Running in %s\n", utils.TruncateID(c.ID))

	// override the entry point that may have been picked up from the base image
	c.Path = b.config.Cmd[0]
	c.Args = b.config.Cmd[1:]

	//start the container
	hostConfig := &HostConfig{}
	if err := c.Start(hostConfig); err != nil {
		return "", err
	}

	if b.verbose {
		err = <-c.Attach(nil, nil, b.out, b.out)
		if err != nil {
			return "", err
		}
	}

	// Wait for it to finish
	if ret := c.Wait(); ret != 0 {
		return "", fmt.Errorf("The command %v returned a non-zero code: %d", b.config.Cmd, ret)
	}

	return c.ID, nil
}

// Commit the container <id> with the autorun command <autoCmd>
func (b *buildFile) commit(id string, autoCmd []string, comment string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to commit")
	}
	b.config.Image = b.image
	if id == "" {
		cmd := b.config.Cmd
		b.config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + comment}
		defer func(cmd []string) { b.config.Cmd = cmd }(cmd)

		if b.utilizeCache {
			if cache, err := b.srv.ImageGetCached(b.image, b.config); err != nil {
				return err
			} else if cache != nil {
				fmt.Fprintf(b.out, " ---> Using cache\n")
				utils.Debugf("[BUILDER] Use cached version")
				b.image = cache.ID
				return nil
			} else {
				utils.Debugf("[BUILDER] Cache miss")
			}
		}

		container, err := b.runtime.Create(b.config)
		if err != nil {
			return err
		}
		b.tmpContainers[container.ID] = struct{}{}
		fmt.Fprintf(b.out, " ---> Running in %s\n", utils.TruncateID(container.ID))
		id = container.ID
		if err := container.EnsureMounted(); err != nil {
			return err
		}
		defer container.Unmount()
	}

	container := b.runtime.Get(id)
	if container == nil {
		return fmt.Errorf("An error occured while creating the container")
	}

	// Note: Actually copy the struct
	autoConfig := *b.config
	autoConfig.Cmd = autoCmd
	// Commit the container
	image, err := b.runtime.Commit(container, "", "", "", b.maintainer, &autoConfig)
	if err != nil {
		return err
	}
	b.tmpImages[image.ID] = struct{}{}
	b.image = image.ID
	return nil
}

func (b *buildFile) Build(context io.Reader) (string, error) {
	// FIXME: @creack any reason for using /tmp instead of ""?
	// FIXME: @creack "name" is a terrible variable name
	name, err := ioutil.TempDir("/tmp", "docker-build")
	if err != nil {
		return "", err
	}
	if err := Untar(context, name); err != nil {
		return "", err
	}
	defer os.RemoveAll(name)
	b.context = name
	if err := b.processFile("Dockerfile"); err != nil {
		return "", err
	}
	if b.image == "" {
		return "", fmt.Errorf("No image, an error must have occurred during the build\n")
	}
	fmt.Fprintf(b.out, "Successfully built %s\n", utils.TruncateID(b.image))
	if b.rm {
		b.clearTmp(b.tmpContainers)
	}
	return b.image, nil
}

func (b *buildFile) processFile(filename string) error {
	filename = path.Join(b.context, filename)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("Can't build a directory with no '%s'", filename)
	}
	dockerfile, err := parseFile(filename)
	if err != nil {
		return err
	}
	if dockerfile == nil {
		fileBytes, err := ioutil.ReadFile(filename)
		if err == nil {
			s := string(fileBytes)
			utils.Debugf("Couldn't parse %s:\n%s", filename, s)
		}
		return fmt.Errorf("Invalid build file %s", filename)
	}
	stepN := 0
	for _, instr := range dockerfile.instructions {
		v := instr.(fmt.Stringer)
		fmt.Fprintf(b.out, "Step %d : %s %s\n", stepN, v.String())
		switch args := instr.(type) {
		case *fromArgs:
			err = b.CmdFrom(args)
		case *maintainerArgs:
			err = b.CmdMaintainer(args)
		case *runArgs:
			err = b.CmdRun(args)
		case *cmdArgs:
			err = b.CmdCmd(args)
		case *exposeArgs:
			err = b.CmdExpose(args)
		case *envArgs:
			err = b.CmdEnv(args)
		case *addArgs:
			err = b.CmdAdd(args)
		case *entryPointArgs:
			err = b.CmdEntrypoint(args)
		case *volumeArgs:
			err = b.CmdVolume(args)
		case *userArgs:
			err = b.CmdUser(args)
		case *workDirArgs:
			err = b.CmdWorkdir(args)
		case *includeArgs:
			err = b.CmdInclude(args)
		default:
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(b.out, " ---> %v\n", utils.TruncateID(b.image))
		stepN += 1
	}
	return nil
}

func NewBuildFile(srv *Server, out io.Writer, verbose, utilizeCache, rm bool) BuildFile {
	return &buildFile{
		runtime:       srv.runtime,
		srv:           srv,
		config:        &Config{},
		out:           out,
		tmpContainers: make(map[string]struct{}),
		tmpImages:     make(map[string]struct{}),
		verbose:       verbose,
		utilizeCache:  utilizeCache,
		rm:            rm,
	}
}
