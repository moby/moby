package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/docker/go-connections/sockets"
	"github.com/go-check/check"
	"golang.org/x/crypto/ssh/terminal"
)

var daemonSockRoot = filepath.Join(os.TempDir(), "docker-integration")

// Daemon represents a Docker daemon for the testing framework.
type Daemon struct {
	GlobalFlags []string

	id                string
	c                 *check.C
	logFile           *os.File
	folder            string
	root              string
	stdin             io.WriteCloser
	stdout, stderr    io.ReadCloser
	storageDriver     string
	wait              chan error
	userlandProxy     bool
	useDefaultTLSHost bool
	containerID       string
	dockerdPID        string
	ip                string
	tlsAddr           string
}

type clientConfig struct {
	transport *http.Transport
	scheme    string
	addr      string
}

// NewDaemon returns a Daemon instance to be used for testing.
// This will create a directory such as d123456789 in the folder specified by $DEST.
// The daemon will not automatically start.
func NewDaemon(c *check.C) *Daemon {
	dest := os.Getenv("DEST")
	c.Assert(dest, check.Not(check.Equals), "", check.Commentf("Please set the DEST environment variable"))

	err := os.MkdirAll(daemonSockRoot, 0700)
	c.Assert(err, checker.IsNil, check.Commentf("could not create daemon socket root"))

	id := fmt.Sprintf("d%d", time.Now().UnixNano()%100000000)
	dir := filepath.Join(dest, id)
	daemonFolder, err := filepath.Abs(dir)
	c.Assert(err, check.IsNil, check.Commentf("Could not make %q an absolute path", dir))
	daemonRoot := filepath.Join(daemonFolder, "root")

	c.Assert(os.MkdirAll(daemonRoot, 0755), check.IsNil, check.Commentf("Could not create daemon root %q", dir))

	userlandProxy := true
	if env := os.Getenv("DOCKER_USERLANDPROXY"); env != "" {
		if val, err := strconv.ParseBool(env); err != nil {
			userlandProxy = val
		}
	}

	args := []string{"run", "-p", strconv.Itoa(opts.DefaultTLSHTTPPort), "-itd", "--privileged", "--pid=host", "-e", "PATH", "-e", "DOCKER_SERVICE_PREFER_OFFLINE_IMAGE=1", "debian:jessie", "nsenter", "-t", "1", "-m", "sh"}
	cmd := exec.Command(dockerBinary, args...)
	out, err := cmd.CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("error starting daemon for container: %s", string(out)))
	containerID := strings.TrimSpace(string(out))

	cmd = exec.Command(dockerBinary, "inspect", "-f", "{{.NetworkSettings.Networks.bridge.IPAddress}}", containerID)
	out, err = cmd.CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("error getting daemon ip: %s", string(out)))
	ip := strings.TrimSpace(string(out))

	cmd = exec.Command(dockerBinary, "port", containerID, strconv.Itoa(opts.DefaultTLSHTTPPort))
	out, err = cmd.CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("error getting tls port: %s", string(out)))
	_, port, err := net.SplitHostPort(strings.TrimSpace(string(out)))
	c.Assert(err, check.IsNil)
	tlsAddr := net.JoinHostPort(opts.DefaultHTTPHost, port)

	c.Assert(ip, check.Not(check.Equals), "")

	return &Daemon{
		id:            id,
		containerID:   containerID,
		tlsAddr:       tlsAddr,
		ip:            ip,
		c:             c,
		folder:        daemonFolder,
		root:          daemonRoot,
		storageDriver: os.Getenv("DOCKER_GRAPHDRIVER"),
		userlandProxy: userlandProxy,
	}
}

func (d *Daemon) getClientConfig() (*clientConfig, error) {
	var (
		transport *http.Transport
		scheme    string
		addr      string
		proto     string
	)
	if d.useDefaultTLSHost {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		option := &tlsconfig.Options{
			CAFile:   filepath.Join(wd, "fixtures/https/ca.pem"),
			CertFile: filepath.Join(wd, "fixtures/https/client-cert.pem"),
			KeyFile:  filepath.Join(wd, "fixtures/https/client-key.pem"),
		}
		tlsConfig, err := tlsconfig.Client(*option)
		if err != nil {
			return nil, err
		}
		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		addr = d.tlsAddr
		scheme = "https"
		proto = "tcp"
	} else {
		addr = fmt.Sprintf("%s:%d", d.ip, opts.DefaultHTTPPort)
		proto = "tcp"
		scheme = "http"
		transport = &http.Transport{}
	}

	d.c.Assert(sockets.ConfigureTransport(transport, proto, addr), check.IsNil)

	return &clientConfig{
		transport: transport,
		scheme:    scheme,
		addr:      addr,
	}, nil
}

// Start will start the daemon and return once it is ready to receive requests.
// You can specify additional daemon flags.
func (d *Daemon) Start(args ...string) error {
	logFile, err := os.OpenFile(filepath.Join(d.folder, "docker.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	d.c.Assert(err, check.IsNil, check.Commentf("[%s] Could not create %s/docker.log", d.id, d.folder))

	return d.StartWithLogFile(logFile, args...)
}

// StartWithLogFile will start the daemon and attach its streams to a given file.
func (d *Daemon) StartWithLogFile(out *os.File, providedArgs ...string) error {
	dockerdBinary, err := exec.LookPath(dockerdBinary)
	d.c.Assert(err, check.IsNil, check.Commentf("[%s] could not find docker binary in $PATH", d.id))

	args := append(d.GlobalFlags,
		"--cgroup-parent", "/docker/"+d.containerID+"/docker",
		"--containerd", "/var/run/docker/libcontainerd/docker-containerd.sock",
		"--graph", d.root,
		"--exec-root", filepath.Join(d.folder, "exec-root"),
		"--pidfile", fmt.Sprintf("%s/docker.pid", d.folder),
		fmt.Sprintf("--userland-proxy=%t", d.userlandProxy),
	)
	if d.useDefaultTLSHost {
		args = append(args, []string{"--host", fmt.Sprintf("tcp://0.0.0.0:%d", opts.DefaultTLSHTTPPort)}...)
	} else {
		args = append(args, []string{"--host", fmt.Sprintf("tcp://%s:%d", d.ip, opts.DefaultHTTPPort)}...)
	}
	if root := os.Getenv("DOCKER_REMAP_ROOT"); root != "" {
		args = append(args, []string{"--userns-remap", root}...)
	}

	// If we don't explicitly set the log-level or debug flag(-D) then
	// turn on debug mode
	foundLog := false
	foundSd := false
	for _, a := range providedArgs {
		if strings.Contains(a, "--log-level") || strings.Contains(a, "-D") || strings.Contains(a, "--debug") {
			foundLog = true
		}
		if strings.Contains(a, "--storage-driver") {
			foundSd = true
		}
	}
	if !foundLog {
		args = append(args, "--debug")
	}
	if d.storageDriver != "" && !foundSd {
		args = append(args, "--storage-driver", d.storageDriver)
	}

	args = append([]string{"exec", d.containerID, dockerdBinary}, append(args, providedArgs...)...)
	if terminal.IsTerminal(int(out.Fd())) {
		args = append([]string{args[0], "-t"}, args[1:]...)
	}
	cmd := exec.Command(dockerBinary, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	d.logFile = out

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[%s] could not start daemon container: %v", d.id, err)
	}

	wait := make(chan error)

	go func() {
		wait <- cmd.Wait()
		d.c.Logf("[%s] exiting daemon", d.id)
		close(wait)
	}()

	d.wait = wait

	tick := time.Tick(500 * time.Millisecond)
	// make sure daemon is ready to receive requests
	startTime := time.Now().Unix()
	for {
		d.c.Logf("[%s] waiting for daemon to start", d.id)
		if time.Now().Unix()-startTime > 5 {
			// After 5 seconds, give up
			return fmt.Errorf("[%s] Daemon exited and never started", d.id)
		}
		select {
		case <-time.After(2 * time.Second):
			return fmt.Errorf("[%s] timeout: daemon does not respond", d.id)
		case <-tick:
			clientConfig, err := d.getClientConfig()
			if err != nil {
				return err
			}

			client := &http.Client{
				Transport: clientConfig.transport,
			}

			req, err := http.NewRequest("GET", "/_ping", nil)
			d.c.Assert(err, check.IsNil, check.Commentf("[%s] could not create new request", d.id))
			req.URL.Host = clientConfig.addr
			req.URL.Scheme = clientConfig.scheme
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode != http.StatusOK {
				d.c.Logf("[%s] received status != 200 OK: %s", d.id, resp.Status)
			}
			d.c.Logf("[%s] daemon started", d.id)
			d.dockerdPID, err = d.getDockerdPID()
			if err != nil {
				return err
			}
			d.root, err = d.queryRootDir()
			if err != nil {
				return fmt.Errorf("[%s] error querying daemon for root directory: %v", d.id, err)
			}
			return nil
		case <-d.wait:
			return fmt.Errorf("[%s] Daemon exited during startup", d.id)
		}
	}
}

// StartWithBusybox will first start the daemon with Daemon.Start()
// then save the busybox image from the main daemon and load it into this Daemon instance.
func (d *Daemon) StartWithBusybox(arg ...string) error {
	if err := d.Start(arg...); err != nil {
		return err
	}
	return d.LoadBusybox()
}

// Kill will send a SIGKILL to the daemon
func (d *Daemon) Kill() error {
	if d.wait == nil {
		return errors.New("daemon not started")
	}

	defer func() {
		d.logFile.Close()
	}()

	if err := d.Signal(syscall.SIGKILL); err != nil {
		d.c.Logf("Could not kill daemon: %v", err)
		return err
	}

	if err := os.Remove(fmt.Sprintf("%s/docker.pid", d.folder)); err != nil {
		return err
	}

	return nil
}

// Signal sends a specified signal to the current daemon process
func (d *Daemon) Signal(sig syscall.Signal) error {
	if d.wait == nil {
		return errors.New("daemon not started")
	}
	if _, err := d.runInSandbox("kill", fmt.Sprintf("-%d", sig), d.dockerdPID); err != nil {
		return fmt.Errorf("could not send signal: %v", err)
	}
	return nil
}

// Stop will send a SIGINT every second and wait for the daemon to stop.
// If it timeouts, a SIGKILL is sent.
// Stop will not delete the daemon directory. If a purged daemon is needed,
// instantiate a new one with NewDaemon.
func (d *Daemon) Stop() error {
	if d.wait == nil {
		return errors.New("daemon not started")
	}

	defer func() {
		d.logFile.Close()
		// remove pid file even if daemon isn't running any more
		os.Remove(fmt.Sprintf("%s/docker.pid", d.folder))
	}()

	i := 1
	tick := time.Tick(time.Second)

	if err := d.Signal(syscall.SIGINT); err != nil {
		return err
	}
out1:
	for {
		select {
		case err := <-d.wait:
			return err
		case <-time.After(20 * time.Second):
			// time for stopping jobs and run onShutdown hooks
			d.c.Logf("timeout: %v", d.id)
			break out1
		}
	}

out2:
	for {
		select {
		case err := <-d.wait:
			return err
		case <-tick:
			i++
			if i > 5 {
				d.c.Logf("tried to interrupt daemon for %d times, now try to kill it", i)
				break out2
			}
			d.c.Logf("Attempt #%d: daemon is still running", i)
			if err := d.Signal(syscall.SIGINT); err != nil {
				d.c.Logf("could not send signal: %v", err)
			}
		}
	}

	if err := d.Signal(syscall.SIGKILL); err != nil {
		d.c.Logf("Could not kill daemon: %v", err)
		return err
	}

	return nil
}

// Restart will restart the daemon by first stopping it and then starting it.
func (d *Daemon) Restart(arg ...string) error {
	d.Stop()
	// in the case of tests running a user namespace-enabled daemon, we have resolved
	// d.root to be the actual final path of the graph dir after the "uid.gid" of
	// remapped root is added--we need to subtract it from the path before calling
	// start or else we will continue making subdirectories rather than truly restarting
	// with the same location/root:
	if root := os.Getenv("DOCKER_REMAP_ROOT"); root != "" {
		d.root = filepath.Dir(d.root)
	}
	return d.Start(arg...)
}

// LoadBusybox will load the stored busybox into a newly started daemon
func (d *Daemon) LoadBusybox() error {
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
	if out, err := d.Cmd("load", "--input", bb); err != nil {
		return fmt.Errorf("could not load busybox image: %s", out)
	}
	if err := os.Remove(bb); err != nil {
		d.c.Logf("could not remove %s: %v", bb, err)
	}
	return nil
}

// runInSandbox executes a command in the container where the daemon is running.
func (d *Daemon) runInSandbox(args ...string) (string, error) {
	cmd := exec.Command(dockerBinary, append([]string{"exec", d.containerID}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// getDockerdPID returns the PID of current dockerd process
func (d *Daemon) getDockerdPID() (string, error) {
	cmd := exec.Command(dockerBinary, "top", d.containerID, "x", "-o", "pid,comm")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			return "", fmt.Errorf("invalid ps output")
		}
		if f[1] == dockerdBinary {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("dockerd pid not found")
}

func (d *Daemon) queryRootDir() (string, error) {
	// update daemon root by asking /info endpoint (to support user
	// namespaced daemon with root remapped uid.gid directory)
	clientConfig, err := d.getClientConfig()
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Transport: clientConfig.transport,
	}

	req, err := http.NewRequest("GET", "/info", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.URL.Host = clientConfig.addr
	req.URL.Scheme = clientConfig.scheme

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body := ioutils.NewReadCloserWrapper(resp.Body, func() error {
		return resp.Body.Close()
	})

	type Info struct {
		DockerRootDir string
	}
	var b []byte
	var i Info
	b, err = readBody(body)
	if err == nil && resp.StatusCode == http.StatusOK {
		// read the docker root dir
		if err = json.Unmarshal(b, &i); err == nil {
			return i.DockerRootDir, nil
		}
	}
	return "", err
}

func (d *Daemon) sock() string {
	if d.useDefaultTLSHost {
		return fmt.Sprintf("tcp://%s", d.tlsAddr)
	}
	return fmt.Sprintf("tcp://%s:%d", d.ip, opts.DefaultHTTPPort)
}

func (d *Daemon) waitRun(contID string) error {
	args := []string{"--host", d.sock()}
	return waitInspectWithArgs(contID, "{{.State.Running}}", "true", 10*time.Second, args...)
}

func (d *Daemon) getBaseDeviceSize(c *check.C) int64 {
	infoCmdOutput, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "-H", d.sock(), "info"),
		exec.Command("grep", "Base Device Size"),
	)
	c.Assert(err, checker.IsNil)
	basesizeSlice := strings.Split(infoCmdOutput, ":")
	basesize := strings.Trim(basesizeSlice[1], " ")
	basesize = strings.Trim(basesize, "\n")[:len(basesize)-3]
	basesizeFloat, err := strconv.ParseFloat(strings.Trim(basesize, " "), 64)
	c.Assert(err, checker.IsNil)
	basesizeBytes := int64(basesizeFloat) * (1024 * 1024 * 1024)
	return basesizeBytes
}

// Cmd will execute a docker CLI command against this Daemon.
// Example: d.Cmd("version") will run docker -H unix://path/to/unix.sock version
func (d *Daemon) Cmd(args ...string) (string, error) {
	c := exec.Command(dockerBinary, d.prependHostArg(args)...)
	b, err := c.CombinedOutput()
	return string(b), err
}

func (d *Daemon) prependHostArg(args []string) []string {
	for _, arg := range args {
		if arg == "--host" || arg == "-H" {
			return args
		}
	}
	newargs := []string{"--host", d.sock()}
	if d.useDefaultTLSHost {
		newargs = append(newargs, "--tlsverify", "--tlscacert", "fixtures/https/ca.pem", "--tlscert", "fixtures/https/client-cert.pem", "--tlskey", "fixtures/https/client-key.pem")
	}
	return append(newargs, args...)
}

// SockRequest executes a socket request on a daemon and returns statuscode and output.
func (d *Daemon) SockRequest(method, endpoint string, data interface{}) (int, []byte, error) {
	jsonData := bytes.NewBuffer(nil)
	if err := json.NewEncoder(jsonData).Encode(data); err != nil {
		return -1, nil, err
	}

	res, body, err := d.SockRequestRaw(method, endpoint, jsonData, "application/json")
	if err != nil {
		return -1, nil, err
	}
	b, err := readBody(body)
	return res.StatusCode, b, err
}

// SockRequestRaw executes a socket request on a daemon and returns a http
// response and a reader for the output data.
func (d *Daemon) SockRequestRaw(method, endpoint string, data io.Reader, ct string) (*http.Response, io.ReadCloser, error) {
	return sockRequestRawToDaemon(method, endpoint, data, ct, d.sock())
}

// LogFileName returns the path the the daemon's log file
func (d *Daemon) LogFileName() string {
	return d.logFile.Name()
}

func (d *Daemon) getIDByName(name string) (string, error) {
	return d.inspectFieldWithError(name, "Id")
}

func (d *Daemon) activeContainers() (ids []string) {
	out, _ := d.Cmd("ps", "-q")
	for _, id := range strings.Split(out, "\n") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return
}

func (d *Daemon) inspectFilter(name, filter string) (string, error) {
	format := fmt.Sprintf("{{%s}}", filter)
	out, err := d.Cmd("inspect", "-f", format, name)
	if err != nil {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func (d *Daemon) inspectFieldWithError(name, field string) (string, error) {
	return d.inspectFilter(name, fmt.Sprintf(".%s", field))
}

func (d *Daemon) findContainerIP(id string) string {
	out, err := d.Cmd("inspect", fmt.Sprintf("--format='{{ .NetworkSettings.Networks.bridge.IPAddress }}'"), id)
	if err != nil {
		d.c.Log(err)
	}
	return strings.Trim(out, " \r\n'")
}

func (d *Daemon) buildImageWithOut(name, dockerfile string, useCache bool, buildFlags ...string) (string, int, error) {
	buildCmd := buildImageCmdWithHost(name, dockerfile, d.sock(), useCache, buildFlags...)
	return runCommandWithOutput(buildCmd)
}

func (d *Daemon) checkActiveContainerCount(c *check.C) (interface{}, check.CommentInterface) {
	out, err := d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	if len(strings.TrimSpace(out)) == 0 {
		return 0, nil
	}
	return len(strings.Split(strings.TrimSpace(out), "\n")), check.Commentf("output: %q", string(out))
}

// newHTTPTestServer starts a new http test server on the gateway interface so
// all container daemons can access it
func (d *Daemon) newHTTPTestServer(handler http.Handler) *httptest.Server {
	out, err := exec.Command(dockerBinary, "inspect", "-f", "{{.NetworkSettings.Networks.bridge.Gateway}}", d.containerID).Output()
	d.c.Assert(err, checker.IsNil)
	gw := strings.TrimSpace(string(out))
	d.c.Assert(gw, checker.Not(checker.Equals), "")
	l, err := net.Listen("tcp", gw+":0")
	d.c.Assert(err, checker.IsNil)
	ts := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: handler},
	}
	ts.Start()
	return ts
}
