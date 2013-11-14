package docker

import (
	"archive/tar"
	"bytes"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

// This file contains utility functions for docker's unit test suite.
// It has to be named XXX_test.go, apparently, in other to access private functions
// from other XXX_test.go functions.

// Create a temporary runtime suitable for unit testing.
// Call t.Fatal() at the first error.
func mkRuntime(f utils.Fataler) *docker.Runtime {
	root, err := newTestDirectory(unitTestStoreBase)
	if err != nil {
		f.Fatal(err)
	}
	config := &docker.DaemonConfig{
		Root:        root,
		AutoRestart: false,
	}
	r, err := docker.NewRuntimeFromDirectory(config)
	if err != nil {
		f.Fatal(err)
	}
	r.UpdateCapabilities(true)
	return r
}

func createNamedTestContainer(eng *engine.Engine, config *docker.Config, f utils.Fataler, name string) (shortId string) {
	job := eng.Job("create", name)
	if err := job.ImportEnv(config); err != nil {
		f.Fatal(err)
	}
	job.StdoutParseString(&shortId)
	if err := job.Run(); err != nil {
		f.Fatal(err)
	}
	return
}

func createTestContainer(eng *engine.Engine, config *docker.Config, f utils.Fataler) (shortId string) {
	return createNamedTestContainer(eng, config, f, "")
}

func mkServerFromEngine(eng *engine.Engine, t utils.Fataler) *docker.Server {
	iSrv := eng.Hack_GetGlobalVar("httpapi.server")
	if iSrv == nil {
		panic("Legacy server field not set in engine")
	}
	srv, ok := iSrv.(*docker.Server)
	if !ok {
		panic("Legacy server field in engine does not cast to *docker.Server")
	}
	return srv
}

func mkRuntimeFromEngine(eng *engine.Engine, t utils.Fataler) *docker.Runtime {
	iRuntime := eng.Hack_GetGlobalVar("httpapi.runtime")
	if iRuntime == nil {
		panic("Legacy runtime field not set in engine")
	}
	runtime, ok := iRuntime.(*docker.Runtime)
	if !ok {
		panic("Legacy runtime field in engine does not cast to *docker.Runtime")
	}
	return runtime
}

func NewTestEngine(t utils.Fataler) *engine.Engine {
	root, err := newTestDirectory(unitTestStoreBase)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(root)
	if err != nil {
		t.Fatal(err)
	}
	// Load default plugins
	// (This is manually copied and modified from main() until we have a more generic plugin system)
	job := eng.Job("initapi")
	job.Setenv("Root", root)
	job.SetenvBool("AutoRestart", false)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	return eng
}

func newTestDirectory(templateDir string) (dir string, err error) {
	return utils.TestDirectory(templateDir)
}

func getCallerName(depth int) string {
	return utils.GetCallerName(depth)
}

// Write `content` to the file at path `dst`, creating it if necessary,
// as well as any missing directories.
// The file is truncated if it already exists.
// Call t.Fatal() at the first error.
func writeFile(dst, content string, t *testing.T) {
	// Create subdirectories if necessary
	if err := os.MkdirAll(path.Dir(dst), 0700); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		t.Fatal(err)
	}
	// Write content (truncate if it exists)
	if _, err := io.Copy(f, strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
}

// Return the contents of file at path `src`.
// Call t.Fatal() at the first error (including if the file doesn't exist)
func readFile(src string, t *testing.T) (content string) {
	f, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Create a test container from the given runtime `r` and run arguments `args`.
// If the image name is "_", (eg. []string{"-i", "-t", "_", "bash"}, it is
// dynamically replaced by the current test image.
// The caller is responsible for destroying the container.
// Call t.Fatal() at the first error.
func mkContainer(r *docker.Runtime, args []string, t *testing.T) (*docker.Container, error) {
	config, _, _, err := docker.ParseRun(args, nil)
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	if err != nil {
		return nil, err
	}
	if config.Image == "_" {
		config.Image = GetTestImage(r).ID
	}
	c, _, err := r.Create(config, "")
	if err != nil {
		return nil, err
	}
	// NOTE: hostConfig is ignored.
	// If `args` specify privileged mode, custom lxc conf, external mount binds,
	// port redirects etc. they will be ignored.
	// This is because the correct way to set these things is to pass environment
	// to the `start` job.
	// FIXME: this helper function should be deprecated in favor of calling
	// `create` and `start` jobs directly.
	return c, nil
}

// Create a test container, start it, wait for it to complete, destroy it,
// and return its standard output as a string.
// The image name (eg. the XXX in []string{"-i", "-t", "XXX", "bash"}, is dynamically replaced by the current test image.
// If t is not nil, call t.Fatal() at the first error. Otherwise return errors normally.
func runContainer(r *docker.Runtime, args []string, t *testing.T) (output string, err error) {
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	container, err := mkContainer(r, args, t)
	if err != nil {
		return "", err
	}
	defer r.Destroy(container)
	stdout, err := container.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer stdout.Close()
	if err := container.Start(); err != nil {
		return "", err
	}
	container.Wait()
	data, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}
	output = string(data)
	return
}

// FIXME: this is duplicated from graph_test.go in the docker package.
func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}
