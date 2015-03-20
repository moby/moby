package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"

	"github.com/docker/docker/builtins"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

type Fataler interface {
	Fatal(...interface{})
}

// This file contains utility functions for docker's unit test suite.
// It has to be named XXX_test.go, apparently, in other to access private functions
// from other XXX_test.go functions.

// Create a temporary daemon suitable for unit testing.
// Call t.Fatal() at the first error.
func mkDaemon(f Fataler) *daemon.Daemon {
	eng := newTestEngine(f, false, "")
	return mkDaemonFromEngine(eng, f)
}

func createNamedTestContainer(eng *engine.Engine, config *runconfig.Config, f Fataler, name string) (shortId string) {
	job := eng.Job("create", name)
	if err := job.ImportEnv(config); err != nil {
		f.Fatal(err)
	}
	var outputBuffer = bytes.NewBuffer(nil)
	job.Stdout.Add(outputBuffer)
	if err := job.Run(); err != nil {
		f.Fatal(err)
	}
	return engine.Tail(outputBuffer, 1)
}

func createTestContainer(eng *engine.Engine, config *runconfig.Config, f Fataler) (shortId string) {
	return createNamedTestContainer(eng, config, f, "")
}

func startContainer(eng *engine.Engine, id string, t Fataler) {
	job := eng.Job("start", id)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
}

func containerRun(eng *engine.Engine, id string, t Fataler) {
	startContainer(eng, id, t)
	containerWait(eng, id, t)
}

func containerFileExists(eng *engine.Engine, id, dir string, t Fataler) bool {
	c := getContainer(eng, id, t)
	if err := c.Mount(); err != nil {
		t.Fatal(err)
	}
	defer c.Unmount()
	if _, err := os.Stat(path.Join(c.RootfsPath(), dir)); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatal(err)
	}
	return true
}

func containerAttach(eng *engine.Engine, id string, t Fataler) (io.WriteCloser, io.ReadCloser) {
	c := getContainer(eng, id, t)
	i := c.StdinPipe()
	o := c.StdoutPipe()
	return i, o
}

func containerWait(eng *engine.Engine, id string, t Fataler) int {
	ex, _ := getContainer(eng, id, t).WaitStop(-1 * time.Second)
	return ex
}

func containerWaitTimeout(eng *engine.Engine, id string, t Fataler) error {
	_, err := getContainer(eng, id, t).WaitStop(500 * time.Millisecond)
	return err
}

func containerKill(eng *engine.Engine, id string, t Fataler) {
	if err := eng.Job("kill", id).Run(); err != nil {
		t.Fatal(err)
	}
}

func containerRunning(eng *engine.Engine, id string, t Fataler) bool {
	return getContainer(eng, id, t).IsRunning()
}

func containerAssertExists(eng *engine.Engine, id string, t Fataler) {
	getContainer(eng, id, t)
}

func containerAssertNotExists(eng *engine.Engine, id string, t Fataler) {
	daemon := mkDaemonFromEngine(eng, t)
	if c, _ := daemon.Get(id); c != nil {
		t.Fatal(fmt.Errorf("Container %s should not exist", id))
	}
}

// assertHttpNotError expect the given response to not have an error.
// Otherwise the it causes the test to fail.
func assertHttpNotError(r *httptest.ResponseRecorder, t Fataler) {
	// Non-error http status are [200, 400)
	if r.Code < http.StatusOK || r.Code >= http.StatusBadRequest {
		t.Fatal(fmt.Errorf("Unexpected http error: %v", r.Code))
	}
}

// assertHttpError expect the given response to have an error.
// Otherwise the it causes the test to fail.
func assertHttpError(r *httptest.ResponseRecorder, t Fataler) {
	// Non-error http status are [200, 400)
	if !(r.Code < http.StatusOK || r.Code >= http.StatusBadRequest) {
		t.Fatal(fmt.Errorf("Unexpected http success code: %v", r.Code))
	}
}

func getContainer(eng *engine.Engine, id string, t Fataler) *daemon.Container {
	daemon := mkDaemonFromEngine(eng, t)
	c, err := daemon.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mkDaemonFromEngine(eng *engine.Engine, t Fataler) *daemon.Daemon {
	iDaemon := eng.Hack_GetGlobalVar("httpapi.daemon")
	if iDaemon == nil {
		panic("Legacy daemon field not set in engine")
	}
	daemon, ok := iDaemon.(*daemon.Daemon)
	if !ok {
		panic("Legacy daemon field in engine does not cast to *daemon.Daemon")
	}
	return daemon
}

func newTestEngine(t Fataler, autorestart bool, root string) *engine.Engine {
	if root == "" {
		if dir, err := newTestDirectory(unitTestStoreBase); err != nil {
			t.Fatal(err)
		} else {
			root = dir
		}
	}
	os.MkdirAll(root, 0700)

	eng := engine.New()
	eng.Logging = false
	// Load default plugins
	if err := builtins.Register(eng); err != nil {
		t.Fatal(err)
	}
	// load registry service
	if err := registry.NewService(nil).Install(eng); err != nil {
		t.Fatal(err)
	}

	// (This is manually copied and modified from main() until we have a more generic plugin system)
	cfg := &daemon.Config{
		Root:        root,
		AutoRestart: autorestart,
		ExecDriver:  "native",
		// Either InterContainerCommunication or EnableIptables must be set,
		// otherwise NewDaemon will fail because of conflicting settings.
		InterContainerCommunication: true,
		TrustKeyPath:                filepath.Join(root, "key.json"),
		LogConfig:                   runconfig.LogConfig{Type: "json-file"},
	}
	d, err := daemon.NewDaemon(cfg, eng)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Install(eng); err != nil {
		t.Fatal(err)
	}
	return eng
}

func NewTestEngine(t Fataler) *engine.Engine {
	return newTestEngine(t, false, "")
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

// Create a test container from the given daemon `r` and run arguments `args`.
// If the image name is "_", (eg. []string{"-i", "-t", "_", "bash"}, it is
// dynamically replaced by the current test image.
// The caller is responsible for destroying the container.
// Call t.Fatal() at the first error.
func mkContainer(r *daemon.Daemon, args []string, t *testing.T) (*daemon.Container, *runconfig.HostConfig, error) {
	config, hc, _, err := parseRun(args)
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
	c, _, err := r.Create(config, nil, "")
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
func runContainer(eng *engine.Engine, r *daemon.Daemon, args []string, t *testing.T) (output string, err error) {
	defer func() {
		if err != nil && t != nil {
			t.Fatal(err)
		}
	}()
	container, hc, err := mkContainer(r, args, t)
	if err != nil {
		return "", err
	}
	defer r.Rm(container)
	stdout := container.StdoutPipe()
	defer stdout.Close()

	job := eng.Job("start", container.ID)
	if err := job.ImportEnv(hc); err != nil {
		return "", err
	}
	if err := job.Run(); err != nil {
		return "", err
	}

	container.WaitStop(-1 * time.Second)
	data, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}
	output = string(data)
	return
}

// FIXME: this is duplicated from graph_test.go in the docker package.
func fakeTar() (io.ReadCloser, error) {
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
	return ioutil.NopCloser(buf), nil
}

func getAllImages(eng *engine.Engine, t *testing.T) *engine.Table {
	return getImages(eng, t, true, "")
}

func getImages(eng *engine.Engine, t *testing.T, all bool, filter string) *engine.Table {
	job := eng.Job("images")
	job.SetenvBool("all", all)
	job.Setenv("filter", filter)
	images, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	return images

}

func parseRun(args []string) (*runconfig.Config, *runconfig.HostConfig, *flag.FlagSet, error) {
	cmd := flag.NewFlagSet("run", flag.ContinueOnError)
	cmd.SetOutput(ioutil.Discard)
	cmd.Usage = nil
	return runconfig.Parse(cmd, args)
}
