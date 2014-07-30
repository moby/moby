package builder

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

var (
	// Long lines can be split with a backslash
	lineContinuation = regexp.MustCompile(`\\\s*\n`)
)

type BuildFile struct {
	daemon *daemon.Daemon
	eng    *engine.Engine

	image      string
	maintainer string
	config     *runconfig.Config

	contextPath string
	context     *utils.TarSum

	verbose      bool
	utilizeCache bool
	rm           bool
	forceRm      bool

	authConfig *registry.AuthConfig
	configFile *registry.ConfigFile

	tmpContainers map[string]struct{}
	tmpImages     map[string]struct{}

	outStream io.Writer
	errStream io.Writer

	// Deprecated, original writer used for ImagePull. To be removed.
	outOld io.Writer
	sf     *utils.StreamFormatter
}

func (b *BuildFile) clearTmp(containers map[string]struct{}) {
	for c := range containers {
		tmp := b.daemon.Get(c)
		if err := b.daemon.Destroy(tmp); err != nil {
			fmt.Fprintf(b.outStream, "Error removing intermediate container %s: %s\n", utils.TruncateID(c), err.Error())
		} else {
			delete(containers, c)
			fmt.Fprintf(b.outStream, "Removing intermediate container %s\n", utils.TruncateID(c))
		}
	}
}

func (b *BuildFile) checkPathForAddition(orig string) error {
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

func (b *BuildFile) addContext(container *daemon.Container, orig, dest string, decompress bool) error {
	var (
		err        error
		destExists = true
		origPath   = path.Join(b.contextPath, orig)
		destPath   = path.Join(container.RootfsPath(), dest)
	)

	if destPath != container.RootfsPath() {
		destPath, err = symlink.FollowSymlinkInScope(destPath, container.RootfsPath())
		if err != nil {
			return err
		}
	}

	// Preserve the trailing '/'
	if strings.HasSuffix(dest, "/") || dest == "." {
		destPath = destPath + "/"
	}

	destStat, err := os.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		destExists = false
	}

	fi, err := os.Stat(origPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}

	if fi.IsDir() {
		return copyAsDirectory(origPath, destPath, destExists)
	}

	// If we are adding a remote file (or we've been told not to decompress), do not try to untar it
	if decompress {
		// First try to unpack the source as an archive
		// to support the untar feature we need to clean up the path a little bit
		// because tar is very forgiving.  First we need to strip off the archive's
		// filename from the path but this is only added if it does not end in / .
		tarDest := destPath
		if strings.HasSuffix(tarDest, "/") {
			tarDest = filepath.Dir(destPath)
		}

		// try to successfully untar the orig
		if err := archive.UntarPath(origPath, tarDest); err == nil {
			return nil
		} else if err != io.EOF {
			utils.Debugf("Couldn't untar %s to %s: %s", origPath, tarDest, err)
		}
	}

	if err := os.MkdirAll(path.Dir(destPath), 0755); err != nil {
		return err
	}
	if err := archive.CopyWithTar(origPath, destPath); err != nil {
		return err
	}

	resPath := destPath
	if destExists && destStat.IsDir() {
		resPath = path.Join(destPath, path.Base(origPath))
	}

	return fixPermissions(resPath, 0, 0)
}

func (b *BuildFile) Build(context io.Reader) (string, error) {
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
		return "", fmt.Errorf("Dockerfile cannot be empty")
	}
	var (
		dockerfile = lineContinuation.ReplaceAllString(stripComments(fileBytes), "")
		stepN      = 0
	)
	for _, line := range strings.Split(dockerfile, "\n") {
		line = strings.Trim(strings.Replace(line, "\t", " ", -1), " \t\r\n")
		if len(line) == 0 {
			continue
		}
		if err := b.BuildStep(fmt.Sprintf("%d", stepN), line); err != nil {
			if b.forceRm {
				b.clearTmp(b.tmpContainers)
			}
			return "", err
		} else if b.rm {
			b.clearTmp(b.tmpContainers)
		}
		stepN += 1
	}
	if b.image != "" {
		fmt.Fprintf(b.outStream, "Successfully built %s\n", utils.TruncateID(b.image))
		return b.image, nil
	}
	return "", fmt.Errorf("No image was generated. This may be because the Dockerfile does not, like, do anything.\n")
}

// BuildStep parses a single build step from `instruction` and executes it in the current context.
func (b *BuildFile) BuildStep(name, expression string) error {
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

func stripComments(raw []byte) string {
	var (
		out   []string
		lines = strings.Split(string(raw), "\n")
	)
	for _, l := range lines {
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func copyAsDirectory(source, destination string, destinationExists bool) error {
	if err := archive.CopyWithTar(source, destination); err != nil {
		return err
	}

	if destinationExists {
		files, err := ioutil.ReadDir(source)
		if err != nil {
			return err
		}

		for _, file := range files {
			if err := fixPermissions(filepath.Join(destination, file.Name()), 0, 0); err != nil {
				return err
			}
		}
		return nil
	}

	return fixPermissions(destination, 0, 0)
}

func fixPermissions(destination string, uid, gid int) error {
	return filepath.Walk(destination, func(path string, info os.FileInfo, err error) error {
		if err := os.Lchown(path, uid, gid); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
}

func NewBuildFile(d *daemon.Daemon, eng *engine.Engine, outStream, errStream io.Writer, verbose, utilizeCache, rm bool, forceRm bool, outOld io.Writer, sf *utils.StreamFormatter, auth *registry.AuthConfig, authConfigFile *registry.ConfigFile) *BuildFile {
	return &BuildFile{
		daemon:        d,
		eng:           eng,
		config:        &runconfig.Config{},
		outStream:     outStream,
		errStream:     errStream,
		tmpContainers: make(map[string]struct{}),
		tmpImages:     make(map[string]struct{}),
		verbose:       verbose,
		utilizeCache:  utilizeCache,
		rm:            rm,
		forceRm:       forceRm,
		sf:            sf,
		authConfig:    auth,
		configFile:    authConfigFile,
		outOld:        outOld,
	}
}
