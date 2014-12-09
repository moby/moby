package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Daemon represents a Docker daemon for the testing framework.
type Daemon struct {
	t              *testing.T
	logFile        *os.File
	folder         string
	stdin          io.WriteCloser
	stdout, stderr io.ReadCloser
	cmd            *exec.Cmd
	storageDriver  string
	execDriver     string
	wait           chan error
}

// NewDaemon returns a Daemon instance to be used for testing.
// This will create a directory such as daemon123456789 in the folder specified by $DEST.
// The daemon will not automatically start.
func NewDaemon(t *testing.T) *Daemon {
	dest := os.Getenv("DEST")
	if dest == "" {
		t.Fatal("Please set the DEST environment variable")
	}

	dir := filepath.Join(dest, fmt.Sprintf("daemon%d", time.Now().UnixNano()%100000000))
	daemonFolder, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Could not make %q an absolute path: %v", dir, err)
	}

	if err := os.MkdirAll(filepath.Join(daemonFolder, "graph"), 0600); err != nil {
		t.Fatalf("Could not create %s/graph directory", daemonFolder)
	}

	return &Daemon{
		t:             t,
		folder:        daemonFolder,
		storageDriver: os.Getenv("DOCKER_GRAPHDRIVER"),
		execDriver:    os.Getenv("DOCKER_EXECDRIVER"),
	}
}

// Start will start the daemon and return once it is ready to receive requests.
// You can specify additional daemon flags.
func (d *Daemon) Start(arg ...string) error {
	dockerBinary, err := exec.LookPath(dockerBinary)
	if err != nil {
		d.t.Fatalf("could not find docker binary in $PATH: %v", err)
	}

	args := []string{
		"--host", d.sock(),
		"--daemon",
		"--graph", fmt.Sprintf("%s/graph", d.folder),
		"--pidfile", fmt.Sprintf("%s/docker.pid", d.folder),
	}

	// If we don't explicitly set the log-level or debug flag(-D) then
	// turn on debug mode
	foundIt := false
	for _, a := range arg {
		if strings.Contains(a, "--log-level") || strings.Contains(a, "-D") {
			foundIt = true
		}
	}
	if !foundIt {
		args = append(args, "--debug")
	}

	if d.storageDriver != "" {
		args = append(args, "--storage-driver", d.storageDriver)
	}
	if d.execDriver != "" {
		args = append(args, "--exec-driver", d.execDriver)
	}

	args = append(args, arg...)
	d.cmd = exec.Command(dockerBinary, args...)

	d.logFile, err = os.OpenFile(filepath.Join(d.folder, "docker.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		d.t.Fatalf("Could not create %s/docker.log: %v", d.folder, err)
	}

	d.cmd.Stdout = d.logFile
	d.cmd.Stderr = d.logFile

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("could not start daemon container: %v", err)
	}

	wait := make(chan error)

	go func() {
		wait <- d.cmd.Wait()
		d.t.Log("exiting daemon")
		close(wait)
	}()

	d.wait = wait

	tick := time.Tick(500 * time.Millisecond)
	// make sure daemon is ready to receive requests
	startTime := time.Now().Unix()
	for {
		d.t.Log("waiting for daemon to start")
		if time.Now().Unix()-startTime > 5 {
			// After 5 seconds, give up
			return errors.New("Daemon exited and never started")
		}
		select {
		case <-time.After(2 * time.Second):
			return errors.New("timeout: daemon does not respond")
		case <-tick:
			c, err := net.Dial("unix", filepath.Join(d.folder, "docker.sock"))
			if err != nil {
				continue
			}

			client := httputil.NewClientConn(c, nil)
			defer client.Close()

			req, err := http.NewRequest("GET", "/_ping", nil)
			if err != nil {
				d.t.Fatalf("could not create new request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode != http.StatusOK {
				d.t.Logf("received status != 200 OK: %s", resp.Status)
			}

			d.t.Log("daemon started")
			return nil
		}
	}
}

// StartWithBusybox will first start the daemon with Daemon.Start()
// then save the busybox image from the main daemon and load it into this Daemon instance.
func (d *Daemon) StartWithBusybox(arg ...string) error {
	if err := d.Start(arg...); err != nil {
		return err
	}
	bb := filepath.Join(d.folder, "busybox.tar")
	if _, err := os.Stat(bb); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unexpected error on busybox.tar stat: %v", err)
		}
		// saving busybox image from main daemon
		if err := exec.Command(dockerBinary, "save", "--output", bb, "busybox:latest").Run(); err != nil {
			return fmt.Errorf("could not save busybox image: %v", err)
		}
	}
	// loading busybox image to this daemon
	if _, err := d.Cmd("load", "--input", bb); err != nil {
		return fmt.Errorf("could not load busybox image: %v", err)
	}
	if err := os.Remove(bb); err != nil {
		d.t.Logf("Could not remove %s: %v", bb, err)
	}
	return nil
}

// Stop will send a SIGINT every second and wait for the daemon to stop.
// If it timeouts, a SIGKILL is sent.
// Stop will not delete the daemon directory. If a purged daemon is needed,
// instantiate a new one with NewDaemon.
func (d *Daemon) Stop() error {
	if d.cmd == nil || d.wait == nil {
		return errors.New("daemon not started")
	}

	defer func() {
		d.logFile.Close()
		d.cmd = nil
	}()

	i := 1
	tick := time.Tick(time.Second)

	if err := d.cmd.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("could not send signal: %v", err)
	}
out:
	for {
		select {
		case err := <-d.wait:
			return err
		case <-time.After(20 * time.Second):
			d.t.Log("timeout")
			break out
		case <-tick:
			d.t.Logf("Attempt #%d: daemon is still running with pid %d", i+1, d.cmd.Process.Pid)
			if err := d.cmd.Process.Signal(os.Interrupt); err != nil {
				return fmt.Errorf("could not send signal: %v", err)
			}
			i++
		}
	}

	if err := d.cmd.Process.Kill(); err != nil {
		d.t.Logf("Could not kill daemon: %v", err)
		return err
	}

	return nil
}

// Restart will restart the daemon by first stopping it and then starting it.
func (d *Daemon) Restart(arg ...string) error {
	d.Stop()
	return d.Start(arg...)
}

func (d *Daemon) sock() string {
	return fmt.Sprintf("unix://%s/docker.sock", d.folder)
}

// Cmd will execute a docker CLI command against this Daemon.
// Example: d.Cmd("version") will run docker -H unix://path/to/unix.sock version
func (d *Daemon) Cmd(name string, arg ...string) (string, error) {
	args := []string{"--host", d.sock(), name}
	args = append(args, arg...)
	c := exec.Command(dockerBinary, args...)
	b, err := c.CombinedOutput()
	return string(b), err
}

func sockRequest(method, endpoint string, data interface{}) ([]byte, error) {
	// FIX: the path to sock should not be hardcoded
	sock := filepath.Join("/", "var", "run", "docker.sock")
	c, err := net.DialTimeout("unix", sock, time.Duration(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("could not dial docker sock at %s: %v", sock, err)
	}

	client := httputil.NewClientConn(c, nil)
	defer client.Close()

	jsonData := bytes.NewBuffer(nil)
	if err := json.NewEncoder(jsonData).Encode(data); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, endpoint, jsonData)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, fmt.Errorf("could not create new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not perform request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return body, fmt.Errorf("received status != 200 OK: %s", resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

func deleteContainer(container string) error {
	container = strings.Replace(container, "\n", " ", -1)
	container = strings.Trim(container, " ")
	killArgs := fmt.Sprintf("kill %v", container)
	killSplitArgs := strings.Split(killArgs, " ")
	killCmd := exec.Command(dockerBinary, killSplitArgs...)
	runCommand(killCmd)
	rmArgs := fmt.Sprintf("rm -v %v", container)
	rmSplitArgs := strings.Split(rmArgs, " ")
	rmCmd := exec.Command(dockerBinary, rmSplitArgs...)
	exitCode, err := runCommand(rmCmd)
	// set error manually if not set
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to remove container: `docker rm` exit is non-zero")
	}

	return err
}

func getAllContainers() (string, error) {
	getContainersCmd := exec.Command(dockerBinary, "ps", "-q", "-a")
	out, exitCode, err := runCommandWithOutput(getContainersCmd)
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to get a list of containers: %v\n", out)
	}

	return out, err
}

func deleteAllContainers() error {
	containers, err := getAllContainers()
	if err != nil {
		fmt.Println(containers)
		return err
	}

	if err = deleteContainer(containers); err != nil {
		return err
	}
	return nil
}

func getPausedContainers() (string, error) {
	getPausedContainersCmd := exec.Command(dockerBinary, "ps", "-f", "status=paused", "-q", "-a")
	out, exitCode, err := runCommandWithOutput(getPausedContainersCmd)
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to get a list of paused containers: %v\n", out)
	}

	return out, err
}

func unpauseContainer(container string) error {
	unpauseCmd := exec.Command(dockerBinary, "unpause", container)
	exitCode, err := runCommand(unpauseCmd)
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to unpause container")
	}

	return nil
}

func unpauseAllContainers() error {
	containers, err := getPausedContainers()
	if err != nil {
		fmt.Println(containers)
		return err
	}

	containers = strings.Replace(containers, "\n", " ", -1)
	containers = strings.Trim(containers, " ")
	containerList := strings.Split(containers, " ")

	for _, value := range containerList {
		if err = unpauseContainer(value); err != nil {
			return err
		}
	}

	return nil
}

func deleteImages(images ...string) error {
	args := make([]string, 1, 2)
	args[0] = "rmi"
	args = append(args, images...)
	rmiCmd := exec.Command(dockerBinary, args...)
	exitCode, err := runCommand(rmiCmd)
	// set error manually if not set
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to remove image: `docker rmi` exit is non-zero")
	}

	return err
}

func imageExists(image string) error {
	inspectCmd := exec.Command(dockerBinary, "inspect", image)
	exitCode, err := runCommand(inspectCmd)
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("couldn't find image %q", image)
	}
	return err
}

func pullImageIfNotExist(image string) (err error) {
	if err := imageExists(image); err != nil {
		pullCmd := exec.Command(dockerBinary, "pull", image)
		_, exitCode, err := runCommandWithOutput(pullCmd)

		if err != nil || exitCode != 0 {
			err = fmt.Errorf("image %q wasn't found locally and it couldn't be pulled: %s", image, err)
		}
	}
	return
}

func dockerCmd(t *testing.T, args ...string) (string, int, error) {
	out, status, err := runCommandWithOutput(exec.Command(dockerBinary, args...))
	if err != nil {
		t.Fatalf("%q failed with errors: %s, %v", strings.Join(args, " "), out, err)
	}
	return out, status, err
}

// execute a docker ocmmand with a timeout
func dockerCmdWithTimeout(timeout time.Duration, args ...string) (string, int, error) {
	out, status, err := runCommandWithOutputAndTimeout(exec.Command(dockerBinary, args...), timeout)
	if err != nil {
		return out, status, fmt.Errorf("%q failed with errors: %v : %q)", strings.Join(args, " "), err, out)
	}
	return out, status, err
}

// execute a docker command in a directory
func dockerCmdInDir(t *testing.T, path string, args ...string) (string, int, error) {
	dockerCommand := exec.Command(dockerBinary, args...)
	dockerCommand.Dir = path
	out, status, err := runCommandWithOutput(dockerCommand)
	if err != nil {
		return out, status, fmt.Errorf("%q failed with errors: %v : %q)", strings.Join(args, " "), err, out)
	}
	return out, status, err
}

// execute a docker command in a directory with a timeout
func dockerCmdInDirWithTimeout(timeout time.Duration, path string, args ...string) (string, int, error) {
	dockerCommand := exec.Command(dockerBinary, args...)
	dockerCommand.Dir = path
	out, status, err := runCommandWithOutputAndTimeout(dockerCommand, timeout)
	if err != nil {
		return out, status, fmt.Errorf("%q failed with errors: %v : %q)", strings.Join(args, " "), err, out)
	}
	return out, status, err
}

func findContainerIP(t *testing.T, id string) string {
	cmd := exec.Command(dockerBinary, "inspect", "--format='{{ .NetworkSettings.IPAddress }}'", id)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	return strings.Trim(out, " \r\n'")
}

func getContainerCount() (int, error) {
	const containers = "Containers:"

	cmd := exec.Command(dockerBinary, "info")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, containers) {
			output := stripTrailingCharacters(line)
			output = strings.TrimLeft(output, containers)
			output = strings.Trim(output, " ")
			containerCount, err := strconv.Atoi(output)
			if err != nil {
				return 0, err
			}
			return containerCount, nil
		}
	}
	return 0, fmt.Errorf("couldn't find the Container count in the output")
}

type FakeContext struct {
	Dir string
}

func (f *FakeContext) Add(file, content string) error {
	filepath := path.Join(f.Dir, file)
	dirpath := path.Dir(filepath)
	if dirpath != "." {
		if err := os.MkdirAll(dirpath, 0755); err != nil {
			return err
		}
	}
	return ioutil.WriteFile(filepath, []byte(content), 0644)
}

func (f *FakeContext) Delete(file string) error {
	filepath := path.Join(f.Dir, file)
	return os.RemoveAll(filepath)
}

func (f *FakeContext) Close() error {
	return os.RemoveAll(f.Dir)
}

func fakeContext(dockerfile string, files map[string]string) (*FakeContext, error) {
	tmp, err := ioutil.TempDir("", "fake-context")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		return nil, err
	}
	ctx := &FakeContext{tmp}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	if err := ctx.Add("Dockerfile", dockerfile); err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

type FakeStorage struct {
	*FakeContext
	*httptest.Server
}

func (f *FakeStorage) Close() error {
	f.Server.Close()
	return f.FakeContext.Close()
}

func fakeStorage(files map[string]string) (*FakeStorage, error) {
	tmp, err := ioutil.TempDir("", "fake-storage")
	if err != nil {
		return nil, err
	}
	ctx := &FakeContext{tmp}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	handler := http.FileServer(http.Dir(ctx.Dir))
	server := httptest.NewServer(handler)
	return &FakeStorage{
		FakeContext: ctx,
		Server:      server,
	}, nil
}

func inspectField(name, field string) (string, error) {
	format := fmt.Sprintf("{{.%s}}", field)
	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", format, name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func inspectFieldJSON(name, field string) (string, error) {
	format := fmt.Sprintf("{{json .%s}}", field)
	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", format, name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func inspectFieldMap(name, path, field string) (string, error) {
	format := fmt.Sprintf("{{index .%s %q}}", path, field)
	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", format, name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func getIDByName(name string) (string, error) {
	return inspectField(name, "Id")
}

// getContainerState returns the exit code of the container
// and true if it's running
// the exit code should be ignored if it's running
func getContainerState(t *testing.T, id string) (int, bool, error) {
	var (
		exitStatus int
		running    bool
	)
	out, exitCode, err := dockerCmd(t, "inspect", "--format={{.State.Running}} {{.State.ExitCode}}", id)
	if err != nil || exitCode != 0 {
		return 0, false, fmt.Errorf("%q doesn't exist: %s", id, err)
	}

	out = strings.Trim(out, "\n")
	splitOutput := strings.Split(out, " ")
	if len(splitOutput) != 2 {
		return 0, false, fmt.Errorf("failed to get container state: output is broken")
	}
	if splitOutput[0] == "true" {
		running = true
	}
	if n, err := strconv.Atoi(splitOutput[1]); err == nil {
		exitStatus = n
	} else {
		return 0, false, fmt.Errorf("failed to get container state: couldn't parse integer")
	}

	return exitStatus, running, nil
}

func buildImageWithOut(name, dockerfile string, useCache bool) (string, string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Stdin = strings.NewReader(dockerfile)
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", out, fmt.Errorf("failed to build the image: %s", out)
	}
	id, err := getIDByName(name)
	if err != nil {
		return "", out, err
	}
	return id, out, nil
}

func buildImageWithStdoutStderr(name, dockerfile string, useCache bool) (string, string, string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Stdin = strings.NewReader(dockerfile)
	stdout, stderr, exitCode, err := runCommandWithStdoutStderr(buildCmd)
	if err != nil || exitCode != 0 {
		return "", stdout, stderr, fmt.Errorf("failed to build the image: %s", stdout)
	}
	id, err := getIDByName(name)
	if err != nil {
		return "", stdout, stderr, err
	}
	return id, stdout, stderr, nil
}

func buildImage(name, dockerfile string, useCache bool) (string, error) {
	id, _, err := buildImageWithOut(name, dockerfile, useCache)
	return id, err
}

func buildImageFromContext(name string, ctx *FakeContext, useCache bool) (string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, ".")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Dir = ctx.Dir
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to build the image: %s", out)
	}
	return getIDByName(name)
}

func buildImageFromPath(name, path string, useCache bool) (string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, path)
	buildCmd := exec.Command(dockerBinary, args...)
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to build the image: %s", out)
	}
	return getIDByName(name)
}

type FakeGIT struct {
	*httptest.Server
	Root    string
	RepoURL string
}

func (g *FakeGIT) Close() {
	g.Server.Close()
	os.RemoveAll(g.Root)
}

func fakeGIT(name string, files map[string]string) (*FakeGIT, error) {
	tmp, err := ioutil.TempDir("", "fake-git-repo")
	if err != nil {
		return nil, err
	}
	ctx := &FakeContext{tmp}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	defer ctx.Close()
	curdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer os.Chdir(curdir)

	if output, err := exec.Command("git", "init", ctx.Dir).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error trying to init repo: %s (%s)", err, output)
	}
	err = os.Chdir(ctx.Dir)
	if err != nil {
		return nil, err
	}
	if output, err := exec.Command("git", "add", "*").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error trying to add files to repo: %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "commit", "-a", "-m", "Initial commit").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error trying to commit to repo: %s (%s)", err, output)
	}

	root, err := ioutil.TempDir("", "docker-test-git-repo")
	if err != nil {
		return nil, err
	}
	repoPath := filepath.Join(root, name+".git")
	if output, err := exec.Command("git", "clone", "--bare", ctx.Dir, repoPath).CombinedOutput(); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("error trying to clone --bare: %s (%s)", err, output)
	}
	err = os.Chdir(repoPath)
	if err != nil {
		os.RemoveAll(root)
		return nil, err
	}
	if output, err := exec.Command("git", "update-server-info").CombinedOutput(); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("error trying to git update-server-info: %s (%s)", err, output)
	}
	err = os.Chdir(curdir)
	if err != nil {
		os.RemoveAll(root)
		return nil, err
	}
	handler := http.FileServer(http.Dir(root))
	server := httptest.NewServer(handler)
	return &FakeGIT{
		Server:  server,
		Root:    root,
		RepoURL: fmt.Sprintf("%s/%s.git", server.URL, name),
	}, nil
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

func containerStorageFile(containerId, basename string) string {
	return filepath.Join("/var/lib/docker/containers", containerId, basename)
}

// docker commands that use this function must be run with the '-d' switch.
func runCommandAndReadContainerFile(filename string, cmd *exec.Cmd) ([]byte, error) {
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("%v: %q", err, out)
	}

	time.Sleep(1 * time.Second)

	contID := strings.TrimSpace(out)

	return readContainerFile(contID, filename)
}

func readContainerFile(containerId, filename string) ([]byte, error) {
	f, err := os.Open(containerStorageFile(containerId, filename))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return content, nil
}
