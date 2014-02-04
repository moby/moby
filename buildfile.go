package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/registry"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrDockerfileEmpty = errors.New("Dockerfile cannot be empty")
)

type BuildFile interface {
	Build(io.Reader) (string, error)
	CmdFrom(string) error
	CmdRun(string) error
}

type buildFile struct {
	runtime *Runtime
	srv     *Server

	image      string
	maintainer string
	config     *Config

	contextPath string
	context     *utils.TarSum

	verbose      bool
	utilizeCache bool
	rm           bool

	authConfig *auth.AuthConfig
	configFile *auth.ConfigFile

	tmpContainers map[string]struct{}
	tmpImages     map[string]struct{}

	outStream io.Writer
	errStream io.Writer

	// Deprecated, original writer used for ImagePull. To be removed.
	outOld io.Writer
	sf     *utils.StreamFormatter
}

func (b *buildFile) clearTmp(containers map[string]struct{}) {
	for c := range containers {
		tmp := b.runtime.Get(c)
		b.runtime.Destroy(tmp)
		fmt.Fprintf(b.outStream, "Removing intermediate container %s\n", utils.TruncateID(c))
	}
}

func (b *buildFile) CmdFrom(name string) error {
	image, err := b.runtime.repositories.LookupImage(name)
	if err != nil {
		if b.runtime.graph.IsNotExist(err) {
			remote, tag := utils.ParseRepositoryTag(name)
			pullRegistryAuth := b.authConfig
			if len(b.configFile.Configs) > 0 {
				// The request came with a full auth config file, we prefer to use that
				endpoint, _, err := registry.ResolveRepositoryName(remote)
				if err != nil {
					return err
				}
				resolvedAuth := b.configFile.ResolveAuthConfig(endpoint)
				pullRegistryAuth = &resolvedAuth
			}
			job := b.srv.Eng.Job("pull", remote, tag)
			job.SetenvBool("json", b.sf.Json())
			job.SetenvBool("parallel", true)
			job.SetenvJson("authConfig", pullRegistryAuth)
			job.Stdout.Add(b.outOld)
			if err := job.Run(); err != nil {
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
	b.image = image.ID
	b.config = &Config{}
	if image.Config != nil {
		b.config = image.Config
	}
	if b.config.Env == nil || len(b.config.Env) == 0 {
		b.config.Env = append(b.config.Env, "HOME=/", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}
	// Process ONBUILD triggers if they exist
	if nTriggers := len(b.config.OnBuild); nTriggers != 0 {
		fmt.Fprintf(b.errStream, "# Executing %d build triggers\n", nTriggers)
	}
	for n, step := range b.config.OnBuild {
		if err := b.BuildStep(fmt.Sprintf("onbuild-%d", n), step); err != nil {
			return err
		}
	}
	b.config.OnBuild = []string{}
	return nil
}

// The ONBUILD command declares a build instruction to be executed in any future build
// using the current image as a base.
func (b *buildFile) CmdOnbuild(trigger string) error {
	b.config.OnBuild = append(b.config.OnBuild, trigger)
	return b.commit("", b.config.Cmd, fmt.Sprintf("ONBUILD %s", trigger))
}

func (b *buildFile) CmdMaintainer(name string) error {
	b.maintainer = name
	return b.commit("", b.config.Cmd, fmt.Sprintf("MAINTAINER %s", name))
}

// probeCache checks to see if image-caching is enabled (`b.utilizeCache`)
// and if so attempts to look up the current `b.image` and `b.config` pair
// in the current server `b.srv`. If an image is found, probeCache returns
// `(true, nil)`. If no image is found, it returns `(false, nil)`. If there
// is any error, it returns `(false, err)`.
func (b *buildFile) probeCache() (bool, error) {
	if b.utilizeCache {
		if cache, err := b.srv.ImageGetCached(b.image, b.config); err != nil {
			return false, err
		} else if cache != nil {
			fmt.Fprintf(b.outStream, " ---> Using cache\n")
			utils.Debugf("[BUILDER] Use cached version")
			b.image = cache.ID
			return true, nil
		} else {
			utils.Debugf("[BUILDER] Cache miss")
		}
	}
	return false, nil
}

func (b *buildFile) CmdRun(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	config, _, _, err := ParseRun(append([]string{b.image}, b.buildCmdFromJson(args)...), nil)
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = nil
	MergeConfig(b.config, config)

	defer func(cmd []string) { b.config.Cmd = cmd }(cmd)

	utils.Debugf("Command to be executed: %v", b.config.Cmd)

	hit, err := b.probeCache()
	if err != nil {
		return err
	}
	if hit {
		return nil
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

func (b *buildFile) CmdEnv(args string) error {
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ENV format")
	}
	key := strings.Trim(tmp[0], " \t")
	value := strings.Trim(tmp[1], " \t")

	envKey := b.FindEnvKey(key)
	replacedValue, err := b.ReplaceEnvMatches(value)
	if err != nil {
		return err
	}
	replacedVar := fmt.Sprintf("%s=%s", key, replacedValue)

	if envKey >= 0 {
		b.config.Env[envKey] = replacedVar
	} else {
		b.config.Env = append(b.config.Env, replacedVar)
	}
	return b.commit("", b.config.Cmd, fmt.Sprintf("ENV %s", replacedVar))
}

func (b *buildFile) buildCmdFromJson(args string) []string {
	var cmd []string
	if err := json.Unmarshal([]byte(args), &cmd); err != nil {
		utils.Debugf("Error unmarshalling: %s, setting to /bin/sh -c", err)
		cmd = []string{"/bin/sh", "-c", args}
	}
	return cmd
}

func (b *buildFile) CmdCmd(args string) error {
	cmd := b.buildCmdFromJson(args)
	b.config.Cmd = cmd
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("CMD %v", cmd)); err != nil {
		return err
	}
	return nil
}

func (b *buildFile) CmdEntrypoint(args string) error {
	entrypoint := b.buildCmdFromJson(args)
	b.config.Entrypoint = entrypoint
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("ENTRYPOINT %v", entrypoint)); err != nil {
		return err
	}
	return nil
}

func (b *buildFile) CmdExpose(args string) error {
	ports := strings.Split(args, " ")
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return b.commit("", b.config.Cmd, fmt.Sprintf("EXPOSE %v", ports))
}

func (b *buildFile) CmdUser(args string) error {
	b.config.User = args
	return b.commit("", b.config.Cmd, fmt.Sprintf("USER %v", args))
}

func (b *buildFile) CmdInsert(args string) error {
	return fmt.Errorf("INSERT has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdCopy(args string) error {
	return fmt.Errorf("COPY has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdWorkdir(workdir string) error {
	b.config.WorkingDir = workdir
	return b.commit("", b.config.Cmd, fmt.Sprintf("WORKDIR %v", workdir))
}

func (b *buildFile) CmdVolume(args string) error {
	if args == "" {
		return fmt.Errorf("Volume cannot be empty")
	}

	var volume []string
	if err := json.Unmarshal([]byte(args), &volume); err != nil {
		volume = []string{args}
	}
	if b.config.Volumes == nil {
		b.config.Volumes = map[string]struct{}{}
	}
	for _, v := range volume {
		b.config.Volumes[v] = struct{}{}
	}
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("VOLUME %s", args)); err != nil {
		return err
	}
	return nil
}

func (b *buildFile) checkPathForAddition(orig string) error {
	origPath := path.Join(b.contextPath, orig)
	if p, err := filepath.EvalSymlinks(origPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	} else {
		origPath = p
	}
	if !strings.HasPrefix(origPath, b.contextPath) {
		return fmt.Errorf("Forbidden path outside the build context: %s (%s)", orig, origPath)
	}
	_, err := os.Stat(origPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}
	return nil
}

func (b *buildFile) addContext(container *Container, orig, dest string) error {
	var (
		origPath = path.Join(b.contextPath, orig)
		destPath = path.Join(container.BasefsPath(), dest)
	)
	// Preserve the trailing '/'
	if strings.HasSuffix(dest, "/") {
		destPath = destPath + "/"
	}
	fi, err := os.Stat(origPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}
	if fi.IsDir() {
		if err := archive.CopyWithTar(origPath, destPath); err != nil {
			return err
		}
		// First try to unpack the source as an archive
	} else if err := archive.UntarPath(origPath, destPath); err != nil {
		utils.Debugf("Couldn't untar %s to %s: %s", origPath, destPath, err)
		// If that fails, just copy it as a regular file
		if err := os.MkdirAll(path.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := archive.CopyWithTar(origPath, destPath); err != nil {
			return err
		}
	}
	return nil
}

func (b *buildFile) CmdAdd(args string) error {
	if b.context == nil {
		return fmt.Errorf("No context given. Impossible to use ADD")
	}
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ADD format")
	}

	orig, err := b.ReplaceEnvMatches(strings.Trim(tmp[0], " \t"))
	if err != nil {
		return err
	}

	dest, err := b.ReplaceEnvMatches(strings.Trim(tmp[1], " \t"))
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) ADD %s in %s", orig, dest)}
	b.config.Image = b.image

	// FIXME: do we really need this?
	var (
		origPath   = orig
		destPath   = dest
		remoteHash string
	)

	if utils.IsURL(orig) {
		resp, err := utils.Download(orig)
		if err != nil {
			return err
		}
		tmpDirName, err := ioutil.TempDir(b.contextPath, "docker-remote")
		if err != nil {
			return err
		}
		tmpFileName := path.Join(tmpDirName, "tmp")
		tmpFile, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDirName)
		if _, err = io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			return err
		}
		origPath = path.Join(filepath.Base(tmpDirName), filepath.Base(tmpFileName))
		tmpFile.Close()

		// Process the checksum
		r, err := archive.Tar(tmpFileName, archive.Uncompressed)
		if err != nil {
			return err
		}
		tarSum := utils.TarSum{Reader: r, DisableCompression: true}
		remoteHash = tarSum.Sum(nil)

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
			destPath = dest + filename
		}
	}

	if err := b.checkPathForAddition(origPath); err != nil {
		return err
	}

	// Hash path and check the cache
	if b.utilizeCache {
		var (
			hash string
			sums = b.context.GetSums()
		)

		if remoteHash != "" {
			hash = remoteHash
		} else if fi, err := os.Stat(path.Join(b.contextPath, origPath)); err != nil {
			return err
		} else if fi.IsDir() {
			var subfiles []string
			for file, sum := range sums {
				absFile := path.Join(b.contextPath, file)
				absOrigPath := path.Join(b.contextPath, origPath)
				if strings.HasPrefix(absFile, absOrigPath) {
					subfiles = append(subfiles, sum)
				}
			}
			sort.Strings(subfiles)
			hasher := sha256.New()
			hasher.Write([]byte(strings.Join(subfiles, ",")))
			hash = "dir:" + hex.EncodeToString(hasher.Sum(nil))
		} else {
			if origPath[0] == '/' && len(origPath) > 1 {
				origPath = origPath[1:]
			}
			origPath = strings.TrimPrefix(origPath, "./")
			if h, ok := sums[origPath]; ok {
				hash = "file:" + h
			}
		}
		b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) ADD %s in %s", hash, dest)}
		hit, err := b.probeCache()
		if err != nil {
			return err
		}
		// If we do not have a hash, never use the cache
		if hit && hash != "" {
			return nil
		}
	}

	// Create the container and start it
	container, _, err := b.runtime.Create(b.config, "")
	if err != nil {
		return err
	}
	b.tmpContainers[container.ID] = struct{}{}

	if err := container.Mount(); err != nil {
		return err
	}
	defer container.Unmount()

	if err := b.addContext(container, origPath, destPath); err != nil {
		return err
	}

	if err := b.commit(container.ID, cmd, fmt.Sprintf("ADD %s in %s", orig, dest)); err != nil {
		return err
	}
	b.config.Cmd = cmd
	return nil
}

type StdoutFormater struct {
	io.Writer
	*utils.StreamFormatter
}

func (sf *StdoutFormater) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream(string(buf))
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}

type StderrFormater struct {
	io.Writer
	*utils.StreamFormatter
}

func (sf *StderrFormater) Write(buf []byte) (int, error) {
	formattedBuf := sf.StreamFormatter.FormatStream("\033[91m" + string(buf) + "\033[0m")
	n, err := sf.Writer.Write(formattedBuf)
	if n != len(formattedBuf) {
		return n, io.ErrShortWrite
	}
	return len(buf), err
}

func (b *buildFile) run() (string, error) {
	if b.image == "" {
		return "", fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	b.config.Image = b.image

	// Create the container and start it
	c, _, err := b.runtime.Create(b.config, "")
	if err != nil {
		return "", err
	}
	b.tmpContainers[c.ID] = struct{}{}
	fmt.Fprintf(b.outStream, " ---> Running in %s\n", utils.TruncateID(c.ID))

	// override the entry point that may have been picked up from the base image
	c.Path = b.config.Cmd[0]
	c.Args = b.config.Cmd[1:]

	var errCh chan error

	if b.verbose {
		errCh = utils.Go(func() error {
			return <-c.Attach(nil, nil, b.outStream, b.errStream)
		})
	}

	//start the container
	if err := c.Start(); err != nil {
		return "", err
	}

	if errCh != nil {
		if err := <-errCh; err != nil {
			return "", err
		}
	}

	// Wait for it to finish
	if ret := c.Wait(); ret != 0 {
		err := &utils.JSONError{
			Message: fmt.Sprintf("The command %v returned a non-zero code: %d", b.config.Cmd, ret),
			Code:    ret,
		}
		return "", err
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

		hit, err := b.probeCache()
		if err != nil {
			return err
		}
		if hit {
			return nil
		}

		container, warnings, err := b.runtime.Create(b.config, "")
		if err != nil {
			return err
		}
		for _, warning := range warnings {
			fmt.Fprintf(b.outStream, " ---> [Warning] %s\n", warning)
		}
		b.tmpContainers[container.ID] = struct{}{}
		fmt.Fprintf(b.outStream, " ---> Running in %s\n", utils.TruncateID(container.ID))
		id = container.ID
		if err := container.Mount(); err != nil {
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

// Long lines can be split with a backslash
var lineContinuation = regexp.MustCompile(`\s*\\\s*\n`)

func (b *buildFile) Build(context io.Reader) (string, error) {
	tmpdirPath, err := ioutil.TempDir("", "docker-build")
	if err != nil {
		return "", err
	}

	decompressedStream, err := archive.DecompressStream(context)
	if err != nil {
		return "", err
	}

	b.context = &utils.TarSum{Reader: decompressedStream, DisableCompression: true}
	if err := archive.Untar(b.context, tmpdirPath, nil); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpdirPath)

	b.contextPath = tmpdirPath
	filename := path.Join(tmpdirPath, "Dockerfile")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", fmt.Errorf("Can't build a directory with no Dockerfile")
	}
	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	if len(fileBytes) == 0 {
		return "", ErrDockerfileEmpty
	}
	dockerfile := string(fileBytes)
	dockerfile = lineContinuation.ReplaceAllString(dockerfile, "")
	stepN := 0
	for _, line := range strings.Split(dockerfile, "\n") {
		line = strings.Trim(strings.Replace(line, "\t", " ", -1), " \t\r\n")
		// Skip comments and empty line
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if err := b.BuildStep(fmt.Sprintf("%d", stepN), line); err != nil {
			return "", err
		}
		stepN += 1

	}
	if b.image != "" {
		fmt.Fprintf(b.outStream, "Successfully built %s\n", utils.TruncateID(b.image))
		if b.rm {
			b.clearTmp(b.tmpContainers)
		}
		return b.image, nil
	}
	return "", fmt.Errorf("No image was generated. This may be because the Dockerfile does not, like, do anything.\n")
}

// BuildStep parses a single build step from `instruction` and executes it in the current context.
func (b *buildFile) BuildStep(name, expression string) error {
	fmt.Fprintf(b.outStream, "Step %s : %s\n", name, expression)
	tmp := strings.SplitN(expression, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid Dockerfile format")
	}
	instruction := strings.ToLower(strings.Trim(tmp[0], " "))
	arguments := strings.Trim(tmp[1], " ")

	method, exists := reflect.TypeOf(b).MethodByName("Cmd" + strings.ToUpper(instruction[:1]) + strings.ToLower(instruction[1:]))
	if !exists {
		fmt.Fprintf(b.errStream, "# Skipping unknown instruction %s\n", strings.ToUpper(instruction))
		return nil
	}

	ret := method.Func.Call([]reflect.Value{reflect.ValueOf(b), reflect.ValueOf(arguments)})[0].Interface()
	if ret != nil {
		return ret.(error)
	}

	fmt.Fprintf(b.outStream, " ---> %s\n", utils.TruncateID(b.image))
	return nil
}

func NewBuildFile(srv *Server, outStream, errStream io.Writer, verbose, utilizeCache, rm bool, outOld io.Writer, sf *utils.StreamFormatter, auth *auth.AuthConfig, authConfigFile *auth.ConfigFile) BuildFile {
	return &buildFile{
		runtime:       srv.runtime,
		srv:           srv,
		config:        &Config{},
		outStream:     outStream,
		errStream:     errStream,
		tmpContainers: make(map[string]struct{}),
		tmpImages:     make(map[string]struct{}),
		verbose:       verbose,
		utilizeCache:  utilizeCache,
		rm:            rm,
		sf:            sf,
		authConfig:    auth,
		configFile:    authConfigFile,
		outOld:        outOld,
	}
}
