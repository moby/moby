package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"
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
	job.Stdout.AddString(&shortId)
	if err := job.Run(); err != nil {
		f.Fatal(err)
	}
	return
}

func createTestContainer(eng *engine.Engine, config *docker.Config, f utils.Fataler) (shortId string) {
	return createNamedTestContainer(eng, config, f, "")
}

func startContainer(eng *engine.Engine, id string, t utils.Fataler) {
	job := eng.Job("start", id)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
}

func containerRun(eng *engine.Engine, id string, t utils.Fataler) {
	startContainer(eng, id, t)
	containerWait(eng, id, t)
}

func containerFileExists(eng *engine.Engine, id, dir string, t utils.Fataler) bool {
	c := getContainer(eng, id, t)
	if err := c.EnsureMounted(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path.Join(c.RootfsPath(), dir)); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatal(err)
	}
	return true
}

func containerAttach(eng *engine.Engine, id string, t utils.Fataler) (io.WriteCloser, io.ReadCloser) {
	c := getContainer(eng, id, t)
	i, err := c.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	o, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	return i, o
}

func containerWait(eng *engine.Engine, id string, t utils.Fataler) int {
	return getContainer(eng, id, t).Wait()
}

func containerWaitTimeout(eng *engine.Engine, id string, t utils.Fataler) error {
	return getContainer(eng, id, t).WaitTimeout(500 * time.Millisecond)
}

func containerKill(eng *engine.Engine, id string, t utils.Fataler) {
	if err := getContainer(eng, id, t).Kill(); err != nil {
		t.Fatal(err)
	}
}

func containerRunning(eng *engine.Engine, id string, t utils.Fataler) bool {
	return getContainer(eng, id, t).State.IsRunning()
}

func containerAssertExists(eng *engine.Engine, id string, t utils.Fataler) {
	getContainer(eng, id, t)
}

func containerAssertNotExists(eng *engine.Engine, id string, t utils.Fataler) {
	runtime := mkRuntimeFromEngine(eng, t)
	if c := runtime.Get(id); c != nil {
		t.Fatal(fmt.Errorf("Container %s should not exist", id))
	}
}

// assertHttpNotError expect the given response to not have an error.
// Otherwise the it causes the test to fail.
func assertHttpNotError(r *httptest.ResponseRecorder, t utils.Fataler) {
	// Non-error http status are [200, 400)
	if r.Code < http.StatusOK || r.Code >= http.StatusBadRequest {
		t.Fatal(fmt.Errorf("Unexpected http error: %v", r.Code))
	}
}

// assertHttpError expect the given response to have an error.
// Otherwise the it causes the test to fail.
func assertHttpError(r *httptest.ResponseRecorder, t utils.Fataler) {
	// Non-error http status are [200, 400)
	if !(r.Code < http.StatusOK || r.Code >= http.StatusBadRequest) {
		t.Fatal(fmt.Errorf("Unexpected http success code: %v", r.Code))
	}
}

func getContainer(eng *engine.Engine, id string, t utils.Fataler) *docker.Container {
	runtime := mkRuntimeFromEngine(eng, t)
	c := runtime.Get(id)
	if c == nil {
		t.Fatal(fmt.Errorf("No such container: %s", id))
	}
	return c
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
	// TestGetEnabledCors and TestOptionsRoute require EnableCors=true
	job.SetenvBool("EnableCors", true)
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
func mkContainer(r *docker.Runtime, args []string, t *testing.T) (*docker.Container, *docker.HostConfig, error) {
	config, hc, _, err := docker.ParseRun(args, nil)
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	if err != nil {
		return nil, nil, err
	}
	if config.Image == "_" {
		config.Image = GetTestImage(r).ID
	}
	c, _, err := r.Create(config, "")
	if err != nil {
		return nil, nil, err
	}
	// NOTE: hostConfig is ignored.
	// If `args` specify privileged mode, custom lxc conf, external mount binds,
	// port redirects etc. they will be ignored.
	// This is because the correct way to set these things is to pass environment
	// to the `start` job.
	// FIXME: this helper function should be deprecated in favor of calling
	// `create` and `start` jobs directly.
	return c, hc, nil
}

// Create a test container, start it, wait for it to complete, destroy it,
// and return its standard output as a string.
// The image name (eg. the XXX in []string{"-i", "-t", "XXX", "bash"}, is dynamically replaced by the current test image.
// If t is not nil, call t.Fatal() at the first error. Otherwise return errors normally.
func runContainer(eng *engine.Engine, r *docker.Runtime, args []string, t *testing.T) (output string, err error) {
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	container, hc, err := mkContainer(r, args, t)
	if err != nil {
		return "", err
	}
	defer r.Destroy(container)
	stdout, err := container.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer stdout.Close()

	job := eng.Job("start", container.ID)
	if err := job.ImportEnv(hc); err != nil {
		return "", err
	}
	if err := job.Run(); err != nil {
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
