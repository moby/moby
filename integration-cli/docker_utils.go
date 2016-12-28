package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/httputils"
	icmd "github.com/docker/docker/pkg/integration/cmd"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/go-connections/tlsconfig"
	units "github.com/docker/go-units"
	"github.com/go-check/check"
)

func init() {
	cmd := exec.Command(dockerBinary, "images", "-f", "dangling=false", "--format", "{{.Repository}}:{{.Tag}}")
	cmd.Env = appendBaseEnv(true)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("err=%v\nout=%s\n", err, out))
	}
	images := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, img := range images {
		protectedImages[img] = struct{}{}
	}

	res, body, err := sockRequestRaw("GET", "/info", nil, "application/json")
	if err != nil {
		panic(fmt.Errorf("Init failed to get /info: %v", err))
	}
	defer body.Close()
	if res.StatusCode != http.StatusOK {
		panic(fmt.Errorf("Init failed to get /info. Res=%v", res))
	}

	svrHeader, _ := httputils.ParseServerHeader(res.Header.Get("Server"))
	daemonPlatform = svrHeader.OS
	if daemonPlatform != "linux" && daemonPlatform != "windows" {
		panic("Cannot run tests against platform: " + daemonPlatform)
	}

	// Now we know the daemon platform, can set paths used by tests.
	var info types.Info
	err = json.NewDecoder(body).Decode(&info)
	if err != nil {
		panic(fmt.Errorf("Init failed to unmarshal docker info: %v", err))
	}

	daemonStorageDriver = info.Driver
	dockerBasePath = info.DockerRootDir
	volumesConfigPath = filepath.Join(dockerBasePath, "volumes")
	containerStoragePath = filepath.Join(dockerBasePath, "containers")
	// Make sure in context of daemon, not the local platform. Note we can't
	// use filepath.FromSlash or ToSlash here as they are a no-op on Unix.
	if daemonPlatform == "windows" {
		volumesConfigPath = strings.Replace(volumesConfigPath, `/`, `\`, -1)
		containerStoragePath = strings.Replace(containerStoragePath, `/`, `\`, -1)
		// On Windows, extract out the version as we need to make selective
		// decisions during integration testing as and when features are implemented.
		// eg in "10.0 10550 (10550.1000.amd64fre.branch.date-time)" we want 10550
		windowsDaemonKV, _ = strconv.Atoi(strings.Split(info.KernelVersion, " ")[1])
	} else {
		volumesConfigPath = strings.Replace(volumesConfigPath, `\`, `/`, -1)
		containerStoragePath = strings.Replace(containerStoragePath, `\`, `/`, -1)
	}
	isolation = info.Isolation
}

func convertBasesize(basesizeBytes int64) (int64, error) {
	basesize := units.HumanSize(float64(basesizeBytes))
	basesize = strings.Trim(basesize, " ")[:len(basesize)-3]
	basesizeFloat, err := strconv.ParseFloat(strings.Trim(basesize, " "), 64)
	if err != nil {
		return 0, err
	}
	return int64(basesizeFloat) * 1024 * 1024 * 1024, nil
}

func daemonHost() string {
	daemonURLStr := "unix://" + opts.DefaultUnixSocket
	if daemonHostVar := os.Getenv("DOCKER_HOST"); daemonHostVar != "" {
		daemonURLStr = daemonHostVar
	}
	return daemonURLStr
}

func getTLSConfig() (*tls.Config, error) {
	dockerCertPath := os.Getenv("DOCKER_CERT_PATH")

	if dockerCertPath == "" {
		return nil, fmt.Errorf("DOCKER_TLS_VERIFY specified, but no DOCKER_CERT_PATH environment variable")
	}

	option := &tlsconfig.Options{
		CAFile:   filepath.Join(dockerCertPath, "ca.pem"),
		CertFile: filepath.Join(dockerCertPath, "cert.pem"),
		KeyFile:  filepath.Join(dockerCertPath, "key.pem"),
	}
	tlsConfig, err := tlsconfig.Client(*option)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

func sockConn(timeout time.Duration, daemon string) (net.Conn, error) {
	if daemon == "" {
		daemon = daemonHost()
	}
	daemonURL, err := url.Parse(daemon)
	if err != nil {
		return nil, fmt.Errorf("could not parse url %q: %v", daemon, err)
	}

	var c net.Conn
	switch daemonURL.Scheme {
	case "npipe":
		return npipeDial(daemonURL.Path, timeout)
	case "unix":
		return net.DialTimeout(daemonURL.Scheme, daemonURL.Path, timeout)
	case "tcp":
		if os.Getenv("DOCKER_TLS_VERIFY") != "" {
			// Setup the socket TLS configuration.
			tlsConfig, err := getTLSConfig()
			if err != nil {
				return nil, err
			}
			dialer := &net.Dialer{Timeout: timeout}
			return tls.DialWithDialer(dialer, daemonURL.Scheme, daemonURL.Host, tlsConfig)
		}
		return net.DialTimeout(daemonURL.Scheme, daemonURL.Host, timeout)
	default:
		return c, fmt.Errorf("unknown scheme %v (%s)", daemonURL.Scheme, daemon)
	}
}

func sockRequest(method, endpoint string, data interface{}) (int, []byte, error) {
	jsonData := bytes.NewBuffer(nil)
	if err := json.NewEncoder(jsonData).Encode(data); err != nil {
		return -1, nil, err
	}

	res, body, err := sockRequestRaw(method, endpoint, jsonData, "application/json")
	if err != nil {
		return -1, nil, err
	}
	b, err := readBody(body)
	return res.StatusCode, b, err
}

func sockRequestRaw(method, endpoint string, data io.Reader, ct string) (*http.Response, io.ReadCloser, error) {
	return sockRequestRawToDaemon(method, endpoint, data, ct, "")
}

func sockRequestRawToDaemon(method, endpoint string, data io.Reader, ct, daemon string) (*http.Response, io.ReadCloser, error) {
	req, client, err := newRequestClient(method, endpoint, data, ct, daemon)
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	body := ioutils.NewReadCloserWrapper(resp.Body, func() error {
		defer resp.Body.Close()
		return client.Close()
	})

	return resp, body, nil
}

func sockRequestHijack(method, endpoint string, data io.Reader, ct string) (net.Conn, *bufio.Reader, error) {
	req, client, err := newRequestClient(method, endpoint, data, ct, "")
	if err != nil {
		return nil, nil, err
	}

	client.Do(req)
	conn, br := client.Hijack()
	return conn, br, nil
}

func newRequestClient(method, endpoint string, data io.Reader, ct, daemon string) (*http.Request, *httputil.ClientConn, error) {
	c, err := sockConn(time.Duration(10*time.Second), daemon)
	if err != nil {
		return nil, nil, fmt.Errorf("could not dial docker daemon: %v", err)
	}

	client := httputil.NewClientConn(c, nil)

	req, err := http.NewRequest(method, endpoint, data)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("could not create new request: %v", err)
	}

	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	return req, client, nil
}

func readBody(b io.ReadCloser) ([]byte, error) {
	defer b.Close()
	return ioutil.ReadAll(b)
}

func deleteContainer(container ...string) error {
	result := icmd.RunCommand(dockerBinary, append([]string{"rm", "-fv"}, container...)...)
	return result.Compare(icmd.Success)
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
	if containers == "" {
		return nil
	}

	err = deleteContainer(strings.Split(strings.TrimSpace(containers), "\n")...)
	if err != nil {
		fmt.Println(err.Error())
	}
	return err
}

func deleteAllNetworks() error {
	networks, err := getAllNetworks()
	if err != nil {
		return err
	}
	var errors []string
	for _, n := range networks {
		if n.Name == "bridge" || n.Name == "none" || n.Name == "host" {
			continue
		}
		if daemonPlatform == "windows" && strings.ToLower(n.Name) == "nat" {
			// nat is a pre-defined network on Windows and cannot be removed
			continue
		}
		status, b, err := sockRequest("DELETE", "/networks/"+n.Name, nil)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		if status != http.StatusNoContent {
			errors = append(errors, fmt.Sprintf("error deleting network %s: %s", n.Name, string(b)))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}

func getAllNetworks() ([]types.NetworkResource, error) {
	var networks []types.NetworkResource
	_, b, err := sockRequest("GET", "/networks", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

func deleteAllPlugins() error {
	plugins, err := getAllPlugins()
	if err != nil {
		return err
	}
	var errors []string
	for _, p := range plugins {
		status, b, err := sockRequest("DELETE", "/plugins/"+p.Name+"?force=1", nil)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		if status != http.StatusNoContent {
			errors = append(errors, fmt.Sprintf("error deleting plugin %s: %s", p.Name, string(b)))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}

func getAllPlugins() (types.PluginsListResponse, error) {
	var plugins types.PluginsListResponse
	_, b, err := sockRequest("GET", "/plugins", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

func deleteAllVolumes() error {
	volumes, err := getAllVolumes()
	if err != nil {
		return err
	}
	var errors []string
	for _, v := range volumes {
		status, b, err := sockRequest("DELETE", "/volumes/"+v.Name, nil)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		if status != http.StatusNoContent {
			errors = append(errors, fmt.Sprintf("error deleting volume %s: %s", v.Name, string(b)))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}

func getAllVolumes() ([]*types.Volume, error) {
	var volumes volumetypes.VolumesListOKBody
	_, b, err := sockRequest("GET", "/volumes", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &volumes); err != nil {
		return nil, err
	}
	return volumes.Volumes, nil
}

var protectedImages = map[string]struct{}{}

func deleteAllImages() error {
	cmd := exec.Command(dockerBinary, "images")
	cmd.Env = appendBaseEnv(true)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	lines := strings.Split(string(out), "\n")[1:]
	var imgs []string
	for _, l := range lines {
		if l == "" {
			continue
		}
		fields := strings.Fields(l)
		imgTag := fields[0] + ":" + fields[1]
		if _, ok := protectedImages[imgTag]; !ok {
			if fields[0] == "<none>" {
				imgs = append(imgs, fields[2])
				continue
			}
			imgs = append(imgs, imgTag)
		}
	}
	if len(imgs) == 0 {
		return nil
	}
	args := append([]string{"rmi", "-f"}, imgs...)
	if err := exec.Command(dockerBinary, args...).Run(); err != nil {
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

func getSliceOfPausedContainers() ([]string, error) {
	out, err := getPausedContainers()
	if err == nil {
		if len(out) == 0 {
			return nil, err
		}
		slice := strings.Split(strings.TrimSpace(out), "\n")
		return slice, err
	}
	return []string{out}, err
}

func unpauseContainer(container string) error {
	return icmd.RunCommand(dockerBinary, "unpause", container).Error
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
	args := []string{dockerBinary, "rmi", "-f"}
	return icmd.RunCmd(icmd.Cmd{Command: append(args, images...)}).Error
}

func imageExists(image string) error {
	return icmd.RunCommand(dockerBinary, "inspect", image).Error
}

func pullImageIfNotExist(image string) error {
	if err := imageExists(image); err != nil {
		pullCmd := exec.Command(dockerBinary, "pull", image)
		_, exitCode, err := runCommandWithOutput(pullCmd)

		if err != nil || exitCode != 0 {
			return fmt.Errorf("image %q wasn't found locally and it couldn't be pulled: %s", image, err)
		}
	}
	return nil
}

func dockerCmdWithError(args ...string) (string, int, error) {
	if err := validateArgs(args...); err != nil {
		return "", 0, err
	}
	result := icmd.RunCommand(dockerBinary, args...)
	if result.Error != nil {
		return result.Combined(), result.ExitCode, result.Compare(icmd.Success)
	}
	return result.Combined(), result.ExitCode, result.Error
}

func dockerCmdWithStdoutStderr(c *check.C, args ...string) (string, string, int) {
	if err := validateArgs(args...); err != nil {
		c.Fatalf(err.Error())
	}

	result := icmd.RunCommand(dockerBinary, args...)
	// TODO: why is c ever nil?
	if c != nil {
		c.Assert(result, icmd.Matches, icmd.Success)
	}
	return result.Stdout(), result.Stderr(), result.ExitCode
}

func dockerCmd(c *check.C, args ...string) (string, int) {
	if err := validateArgs(args...); err != nil {
		c.Fatalf(err.Error())
	}
	result := icmd.RunCommand(dockerBinary, args...)
	c.Assert(result, icmd.Matches, icmd.Success)
	return result.Combined(), result.ExitCode
}

func dockerCmdWithResult(args ...string) *icmd.Result {
	return icmd.RunCommand(dockerBinary, args...)
}

func binaryWithArgs(args ...string) []string {
	return append([]string{dockerBinary}, args...)
}

// execute a docker command with a timeout
func dockerCmdWithTimeout(timeout time.Duration, args ...string) *icmd.Result {
	if err := validateArgs(args...); err != nil {
		return &icmd.Result{Error: err}
	}
	return icmd.RunCmd(icmd.Cmd{Command: binaryWithArgs(args...), Timeout: timeout})
}

// execute a docker command in a directory
func dockerCmdInDir(c *check.C, path string, args ...string) (string, int, error) {
	if err := validateArgs(args...); err != nil {
		c.Fatalf(err.Error())
	}
	result := icmd.RunCmd(icmd.Cmd{Command: binaryWithArgs(args...), Dir: path})
	return result.Combined(), result.ExitCode, result.Error
}

// execute a docker command in a directory with a timeout
func dockerCmdInDirWithTimeout(timeout time.Duration, path string, args ...string) *icmd.Result {
	if err := validateArgs(args...); err != nil {
		return &icmd.Result{Error: err}
	}
	return icmd.RunCmd(icmd.Cmd{
		Command: binaryWithArgs(args...),
		Timeout: timeout,
		Dir:     path,
	})
}

// validateArgs is a checker to ensure tests are not running commands which are
// not supported on platforms. Specifically on Windows this is 'busybox top'.
func validateArgs(args ...string) error {
	if daemonPlatform != "windows" {
		return nil
	}
	foundBusybox := -1
	for key, value := range args {
		if strings.ToLower(value) == "busybox" {
			foundBusybox = key
		}
		if (foundBusybox != -1) && (key == foundBusybox+1) && (strings.ToLower(value) == "top") {
			return errors.New("cannot use 'busybox top' in tests on Windows. Use runSleepingContainer()")
		}
	}
	return nil
}

// find the State.ExitCode in container metadata
func findContainerExitCode(c *check.C, name string, vargs ...string) string {
	args := append(vargs, "inspect", "--format='{{ .State.ExitCode }} {{ .State.Error }}'", name)
	cmd := exec.Command(dockerBinary, args...)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	return out
}

func findContainerIP(c *check.C, id string, network string) string {
	out, _ := dockerCmd(c, "inspect", fmt.Sprintf("--format='{{ .NetworkSettings.Networks.%s.IPAddress }}'", network), id)
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
			output := strings.TrimSpace(line)
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

// FakeContext creates directories that can be used as a build context
type FakeContext struct {
	Dir string
}

// Add a file at a path, creating directories where necessary
func (f *FakeContext) Add(file, content string) error {
	return f.addFile(file, []byte(content))
}

func (f *FakeContext) addFile(file string, content []byte) error {
	fp := filepath.Join(f.Dir, filepath.FromSlash(file))
	dirpath := filepath.Dir(fp)
	if dirpath != "." {
		if err := os.MkdirAll(dirpath, 0755); err != nil {
			return err
		}
	}
	return ioutil.WriteFile(fp, content, 0644)

}

// Delete a file at a path
func (f *FakeContext) Delete(file string) error {
	fp := filepath.Join(f.Dir, filepath.FromSlash(file))
	return os.RemoveAll(fp)
}

// Close deletes the context
func (f *FakeContext) Close() error {
	return os.RemoveAll(f.Dir)
}

func fakeContextFromNewTempDir() (*FakeContext, error) {
	tmp, err := ioutil.TempDir("", "fake-context")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		return nil, err
	}
	return fakeContextFromDir(tmp), nil
}

func fakeContextFromDir(dir string) *FakeContext {
	return &FakeContext{dir}
}

func fakeContextWithFiles(files map[string]string) (*FakeContext, error) {
	ctx, err := fakeContextFromNewTempDir()
	if err != nil {
		return nil, err
	}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	return ctx, nil
}

func fakeContextAddDockerfile(ctx *FakeContext, dockerfile string) error {
	if err := ctx.Add("Dockerfile", dockerfile); err != nil {
		ctx.Close()
		return err
	}
	return nil
}

func fakeContext(dockerfile string, files map[string]string) (*FakeContext, error) {
	ctx, err := fakeContextWithFiles(files)
	if err != nil {
		return nil, err
	}
	if err := fakeContextAddDockerfile(ctx, dockerfile); err != nil {
		return nil, err
	}
	return ctx, nil
}

// FakeStorage is a static file server. It might be running locally or remotely
// on test host.
type FakeStorage interface {
	Close() error
	URL() string
	CtxDir() string
}

func fakeBinaryStorage(archives map[string]*bytes.Buffer) (FakeStorage, error) {
	ctx, err := fakeContextFromNewTempDir()
	if err != nil {
		return nil, err
	}
	for name, content := range archives {
		if err := ctx.addFile(name, content.Bytes()); err != nil {
			return nil, err
		}
	}
	return fakeStorageWithContext(ctx)
}

// fakeStorage returns either a local or remote (at daemon machine) file server
func fakeStorage(files map[string]string) (FakeStorage, error) {
	ctx, err := fakeContextWithFiles(files)
	if err != nil {
		return nil, err
	}
	return fakeStorageWithContext(ctx)
}

// fakeStorageWithContext returns either a local or remote (at daemon machine) file server
func fakeStorageWithContext(ctx *FakeContext) (FakeStorage, error) {
	if isLocalDaemon {
		return newLocalFakeStorage(ctx)
	}
	return newRemoteFileServer(ctx)
}

// localFileStorage is a file storage on the running machine
type localFileStorage struct {
	*FakeContext
	*httptest.Server
}

func (s *localFileStorage) URL() string {
	return s.Server.URL
}

func (s *localFileStorage) CtxDir() string {
	return s.FakeContext.Dir
}

func (s *localFileStorage) Close() error {
	defer s.Server.Close()
	return s.FakeContext.Close()
}

func newLocalFakeStorage(ctx *FakeContext) (*localFileStorage, error) {
	handler := http.FileServer(http.Dir(ctx.Dir))
	server := httptest.NewServer(handler)
	return &localFileStorage{
		FakeContext: ctx,
		Server:      server,
	}, nil
}

// remoteFileServer is a containerized static file server started on the remote
// testing machine to be used in URL-accepting docker build functionality.
type remoteFileServer struct {
	host      string // hostname/port web server is listening to on docker host e.g. 0.0.0.0:43712
	container string
	image     string
	ctx       *FakeContext
}

func (f *remoteFileServer) URL() string {
	u := url.URL{
		Scheme: "http",
		Host:   f.host}
	return u.String()
}

func (f *remoteFileServer) CtxDir() string {
	return f.ctx.Dir
}

func (f *remoteFileServer) Close() error {
	defer func() {
		if f.ctx != nil {
			f.ctx.Close()
		}
		if f.image != "" {
			deleteImages(f.image)
		}
	}()
	if f.container == "" {
		return nil
	}
	return deleteContainer(f.container)
}

func newRemoteFileServer(ctx *FakeContext) (*remoteFileServer, error) {
	var (
		image     = fmt.Sprintf("fileserver-img-%s", strings.ToLower(stringutils.GenerateRandomAlphaOnlyString(10)))
		container = fmt.Sprintf("fileserver-cnt-%s", strings.ToLower(stringutils.GenerateRandomAlphaOnlyString(10)))
	)

	if err := ensureHTTPServerImage(); err != nil {
		return nil, err
	}

	// Build the image
	if err := fakeContextAddDockerfile(ctx, `FROM httpserver
COPY . /static`); err != nil {
		return nil, fmt.Errorf("Cannot add Dockerfile to context: %v", err)
	}
	if _, err := buildImageFromContext(image, ctx, false); err != nil {
		return nil, fmt.Errorf("failed building file storage container image: %v", err)
	}

	// Start the container
	runCmd := exec.Command(dockerBinary, "run", "-d", "-P", "--name", container, image)
	if out, ec, err := runCommandWithOutput(runCmd); err != nil {
		return nil, fmt.Errorf("failed to start file storage container. ec=%v\nout=%s\nerr=%v", ec, out, err)
	}

	// Find out the system assigned port
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "port", container, "80/tcp"))
	if err != nil {
		return nil, fmt.Errorf("failed to find container port: err=%v\nout=%s", err, out)
	}

	fileserverHostPort := strings.Trim(out, "\n")
	_, port, err := net.SplitHostPort(fileserverHostPort)
	if err != nil {
		return nil, fmt.Errorf("unable to parse file server host:port: %v", err)
	}

	dockerHostURL, err := url.Parse(daemonHost())
	if err != nil {
		return nil, fmt.Errorf("unable to parse daemon host URL: %v", err)
	}

	host, _, err := net.SplitHostPort(dockerHostURL.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to parse docker daemon host:port: %v", err)
	}

	return &remoteFileServer{
		container: container,
		image:     image,
		host:      fmt.Sprintf("%s:%s", host, port),
		ctx:       ctx}, nil
}

func inspectFieldAndMarshall(c *check.C, name, field string, output interface{}) {
	str := inspectFieldJSON(c, name, field)
	err := json.Unmarshal([]byte(str), output)
	if c != nil {
		c.Assert(err, check.IsNil, check.Commentf("failed to unmarshal: %v", err))
	}
}

func inspectFilter(name, filter string) (string, error) {
	format := fmt.Sprintf("{{%s}}", filter)
	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", format, name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func inspectFieldWithError(name, field string) (string, error) {
	return inspectFilter(name, fmt.Sprintf(".%s", field))
}

func inspectField(c *check.C, name, field string) string {
	out, err := inspectFilter(name, fmt.Sprintf(".%s", field))
	if c != nil {
		c.Assert(err, check.IsNil)
	}
	return out
}

func inspectFieldJSON(c *check.C, name, field string) string {
	out, err := inspectFilter(name, fmt.Sprintf("json .%s", field))
	if c != nil {
		c.Assert(err, check.IsNil)
	}
	return out
}

func inspectFieldMap(c *check.C, name, path, field string) string {
	out, err := inspectFilter(name, fmt.Sprintf("index .%s %q", path, field))
	if c != nil {
		c.Assert(err, check.IsNil)
	}
	return out
}

func inspectMountSourceField(name, destination string) (string, error) {
	m, err := inspectMountPoint(name, destination)
	if err != nil {
		return "", err
	}
	return m.Source, nil
}

func inspectMountPoint(name, destination string) (types.MountPoint, error) {
	out, err := inspectFilter(name, "json .Mounts")
	if err != nil {
		return types.MountPoint{}, err
	}

	return inspectMountPointJSON(out, destination)
}

var errMountNotFound = errors.New("mount point not found")

func inspectMountPointJSON(j, destination string) (types.MountPoint, error) {
	var mp []types.MountPoint
	if err := json.Unmarshal([]byte(j), &mp); err != nil {
		return types.MountPoint{}, err
	}

	var m *types.MountPoint
	for _, c := range mp {
		if c.Destination == destination {
			m = &c
			break
		}
	}

	if m == nil {
		return types.MountPoint{}, errMountNotFound
	}

	return *m, nil
}

func inspectImage(name, filter string) (string, error) {
	args := []string{"inspect", "--type", "image"}
	if filter != "" {
		format := fmt.Sprintf("{{%s}}", filter)
		args = append(args, "-f", format)
	}
	args = append(args, name)
	inspectCmd := exec.Command(dockerBinary, args...)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func getIDByName(name string) (string, error) {
	return inspectFieldWithError(name, "Id")
}

// getContainerState returns the exit code of the container
// and true if it's running
// the exit code should be ignored if it's running
func getContainerState(c *check.C, id string) (int, bool, error) {
	var (
		exitStatus int
		running    bool
	)
	out, exitCode := dockerCmd(c, "inspect", "--format={{.State.Running}} {{.State.ExitCode}}", id)
	if exitCode != 0 {
		return 0, false, fmt.Errorf("%q doesn't exist: %s", id, out)
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

func buildImageCmd(name, dockerfile string, useCache bool, buildFlags ...string) *exec.Cmd {
	return buildImageCmdWithHost(name, dockerfile, "", useCache, buildFlags...)
}

func buildImageCmdWithHost(name, dockerfile, host string, useCache bool, buildFlags ...string) *exec.Cmd {
	args := []string{}
	if host != "" {
		args = append(args, "--host", host)
	}
	args = append(args, "build", "-t", name)
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, buildFlags...)
	args = append(args, "-")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Stdin = strings.NewReader(dockerfile)
	return buildCmd
}

func buildImageWithOut(name, dockerfile string, useCache bool, buildFlags ...string) (string, string, error) {
	buildCmd := buildImageCmd(name, dockerfile, useCache, buildFlags...)
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

func buildImageWithStdoutStderr(name, dockerfile string, useCache bool, buildFlags ...string) (string, string, string, error) {
	buildCmd := buildImageCmd(name, dockerfile, useCache, buildFlags...)
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

func buildImage(name, dockerfile string, useCache bool, buildFlags ...string) (string, error) {
	id, _, err := buildImageWithOut(name, dockerfile, useCache, buildFlags...)
	return id, err
}

func buildImageFromContext(name string, ctx *FakeContext, useCache bool, buildFlags ...string) (string, error) {
	id, _, err := buildImageFromContextWithOut(name, ctx, useCache, buildFlags...)
	if err != nil {
		return "", err
	}
	return id, nil
}

func buildImageFromContextWithOut(name string, ctx *FakeContext, useCache bool, buildFlags ...string) (string, string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, buildFlags...)
	args = append(args, ".")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Dir = ctx.Dir
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", "", fmt.Errorf("failed to build the image: %s", out)
	}
	id, err := getIDByName(name)
	if err != nil {
		return "", "", err
	}
	return id, out, nil
}

func buildImageFromContextWithStdoutStderr(name string, ctx *FakeContext, useCache bool, buildFlags ...string) (string, string, string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, buildFlags...)
	args = append(args, ".")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Dir = ctx.Dir

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

func buildImageFromGitWithStdoutStderr(name string, ctx *fakeGit, useCache bool, buildFlags ...string) (string, string, string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, buildFlags...)
	args = append(args, ctx.RepoURL)
	buildCmd := exec.Command(dockerBinary, args...)

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

func buildImageFromPath(name, path string, useCache bool, buildFlags ...string) (string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, buildFlags...)
	args = append(args, path)
	buildCmd := exec.Command(dockerBinary, args...)
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to build the image: %s", out)
	}
	return getIDByName(name)
}

type gitServer interface {
	URL() string
	Close() error
}

type localGitServer struct {
	*httptest.Server
}

func (r *localGitServer) Close() error {
	r.Server.Close()
	return nil
}

func (r *localGitServer) URL() string {
	return r.Server.URL
}

type fakeGit struct {
	root    string
	server  gitServer
	RepoURL string
}

func (g *fakeGit) Close() {
	g.server.Close()
	os.RemoveAll(g.root)
}

func newFakeGit(name string, files map[string]string, enforceLocalServer bool) (*fakeGit, error) {
	ctx, err := fakeContextWithFiles(files)
	if err != nil {
		return nil, err
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
	if output, err := exec.Command("git", "config", "user.name", "Fake User").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error trying to set 'user.name': %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "config", "user.email", "fake.user@example.com").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error trying to set 'user.email': %s (%s)", err, output)
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

	var server gitServer
	if !enforceLocalServer {
		// use fakeStorage server, which might be local or remote (at test daemon)
		server, err = fakeStorageWithContext(fakeContextFromDir(root))
		if err != nil {
			return nil, fmt.Errorf("cannot start fake storage: %v", err)
		}
	} else {
		// always start a local http server on CLI test machine
		httpServer := httptest.NewServer(http.FileServer(http.Dir(root)))
		server = &localGitServer{httpServer}
	}
	return &fakeGit{
		root:    root,
		server:  server,
		RepoURL: fmt.Sprintf("%s/%s.git", server.URL(), name),
	}, nil
}

// Write `content` to the file at path `dst`, creating it if necessary,
// as well as any missing directories.
// The file is truncated if it already exists.
// Fail the test when error occurs.
func writeFile(dst, content string, c *check.C) {
	// Create subdirectories if necessary
	c.Assert(os.MkdirAll(path.Dir(dst), 0700), check.IsNil)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	c.Assert(err, check.IsNil)
	defer f.Close()
	// Write content (truncate if it exists)
	_, err = io.Copy(f, strings.NewReader(content))
	c.Assert(err, check.IsNil)
}

// Return the contents of file at path `src`.
// Fail the test when error occurs.
func readFile(src string, c *check.C) (content string) {
	data, err := ioutil.ReadFile(src)
	c.Assert(err, check.IsNil)

	return string(data)
}

func containerStorageFile(containerID, basename string) string {
	return filepath.Join(containerStoragePath, containerID, basename)
}

// docker commands that use this function must be run with the '-d' switch.
func runCommandAndReadContainerFile(filename string, cmd *exec.Cmd) ([]byte, error) {
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("%v: %q", err, out)
	}

	contID := strings.TrimSpace(out)

	if err := waitRun(contID); err != nil {
		return nil, fmt.Errorf("%v: %q", contID, err)
	}

	return readContainerFile(contID, filename)
}

func readContainerFile(containerID, filename string) ([]byte, error) {
	f, err := os.Open(containerStorageFile(containerID, filename))
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

func readContainerFileWithExec(containerID, filename string) ([]byte, error) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "exec", containerID, "cat", filename))
	return []byte(out), err
}

// daemonTime provides the current time on the daemon host
func daemonTime(c *check.C) time.Time {
	if isLocalDaemon {
		return time.Now()
	}

	status, body, err := sockRequest("GET", "/info", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	type infoJSON struct {
		SystemTime string
	}
	var info infoJSON
	err = json.Unmarshal(body, &info)
	c.Assert(err, check.IsNil, check.Commentf("unable to unmarshal GET /info response"))

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	c.Assert(err, check.IsNil, check.Commentf("invalid time format in GET /info response"))
	return dt
}

// daemonUnixTime returns the current time on the daemon host with nanoseconds precision.
// It return the time formatted how the client sends timestamps to the server.
func daemonUnixTime(c *check.C) string {
	return parseEventTime(daemonTime(c))
}

func parseEventTime(t time.Time) string {
	return fmt.Sprintf("%d.%09d", t.Unix(), int64(t.Nanosecond()))
}

func setupRegistry(c *check.C, schema1 bool, auth, tokenURL string) *testRegistryV2 {
	reg, err := newTestRegistryV2(c, schema1, auth, tokenURL)
	c.Assert(err, check.IsNil)

	// Wait for registry to be ready to serve requests.
	for i := 0; i != 50; i++ {
		if err = reg.Ping(); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	c.Assert(err, check.IsNil, check.Commentf("Timeout waiting for test registry to become available: %v", err))
	return reg
}

func setupNotary(c *check.C) *testNotary {
	ts, err := newTestNotary(c)
	c.Assert(err, check.IsNil)

	return ts
}

// appendBaseEnv appends the minimum set of environment variables to exec the
// docker cli binary for testing with correct configuration to the given env
// list.
func appendBaseEnv(isTLS bool, env ...string) []string {
	preserveList := []string{
		// preserve remote test host
		"DOCKER_HOST",

		// windows: requires preserving SystemRoot, otherwise dial tcp fails
		// with "GetAddrInfoW: A non-recoverable error occurred during a database lookup."
		"SystemRoot",

		// testing help text requires the $PATH to dockerd is set
		"PATH",
	}
	if isTLS {
		preserveList = append(preserveList, "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH")
	}

	for _, key := range preserveList {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return env
}

func createTmpFile(c *check.C, content string) string {
	f, err := ioutil.TempFile("", "testfile")
	c.Assert(err, check.IsNil)

	filename := f.Name()

	err = ioutil.WriteFile(filename, []byte(content), 0644)
	c.Assert(err, check.IsNil)

	return filename
}

func buildImageWithOutInDamon(socket string, name, dockerfile string, useCache bool) (string, error) {
	args := []string{"--host", socket}
	buildCmd := buildImageCmdArgs(args, name, dockerfile, useCache)
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return out, fmt.Errorf("failed to build the image: %s, error: %v", out, err)
	}
	return out, nil
}

func buildImageCmdArgs(args []string, name, dockerfile string, useCache bool) *exec.Cmd {
	args = append(args, []string{"-D", "build", "-t", name}...)
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Stdin = strings.NewReader(dockerfile)
	return buildCmd

}

func waitForContainer(contID string, args ...string) error {
	args = append([]string{dockerBinary, "run", "--name", contID}, args...)
	result := icmd.RunCmd(icmd.Cmd{Command: args})
	if result.Error != nil {
		return result.Error
	}
	return waitRun(contID)
}

// waitRestart will wait for the specified container to restart once
func waitRestart(contID string, duration time.Duration) error {
	return waitInspect(contID, "{{.RestartCount}}", "1", duration)
}

// waitRun will wait for the specified container to be running, maximum 5 seconds.
func waitRun(contID string) error {
	return waitInspect(contID, "{{.State.Running}}", "true", 5*time.Second)
}

// waitExited will wait for the specified container to state exit, subject
// to a maximum time limit in seconds supplied by the caller
func waitExited(contID string, duration time.Duration) error {
	return waitInspect(contID, "{{.State.Status}}", "exited", duration)
}

// waitInspect will wait for the specified container to have the specified string
// in the inspect output. It will wait until the specified timeout (in seconds)
// is reached.
func waitInspect(name, expr, expected string, timeout time.Duration) error {
	return waitInspectWithArgs(name, expr, expected, timeout)
}

func waitInspectWithArgs(name, expr, expected string, timeout time.Duration, arg ...string) error {
	after := time.After(timeout)

	args := append(arg, "inspect", "-f", expr, name)
	for {
		result := icmd.RunCommand(dockerBinary, args...)
		if result.Error != nil {
			if !strings.Contains(result.Stderr(), "No such") {
				return fmt.Errorf("error executing docker inspect: %v\n%s",
					result.Stderr(), result.Stdout())
			}
			select {
			case <-after:
				return result.Error
			default:
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		out := strings.TrimSpace(result.Stdout())
		if out == expected {
			break
		}

		select {
		case <-after:
			return fmt.Errorf("condition \"%q == %q\" not true in time", out, expected)
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func getInspectBody(c *check.C, version, id string) []byte {
	endpoint := fmt.Sprintf("/%s/containers/%s/json", version, id)
	status, body, err := sockRequest("GET", endpoint, nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)
	return body
}

// Run a long running idle task in a background container using the
// system-specific default image and command.
func runSleepingContainer(c *check.C, extraArgs ...string) (string, int) {
	return runSleepingContainerInImage(c, defaultSleepImage, extraArgs...)
}

// Run a long running idle task in a background container using the specified
// image and the system-specific command.
func runSleepingContainerInImage(c *check.C, image string, extraArgs ...string) (string, int) {
	args := []string{"run", "-d"}
	args = append(args, extraArgs...)
	args = append(args, image)
	args = append(args, sleepCommandForDaemonPlatform()...)
	return dockerCmd(c, args...)
}

func getRootUIDGID() (int, int, error) {
	uidgid := strings.Split(filepath.Base(dockerBasePath), ".")
	if len(uidgid) == 1 {
		//user namespace remapping is not turned on; return 0
		return 0, 0, nil
	}
	uid, err := strconv.Atoi(uidgid[0])
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(uidgid[1])
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}

// minimalBaseImage returns the name of the minimal base image for the current
// daemon platform.
func minimalBaseImage() string {
	if daemonPlatform == "windows" {
		return WindowsBaseImage
	}
	return "scratch"
}

func getGoroutineNumber() (int, error) {
	i := struct {
		NGoroutines int
	}{}
	status, b, err := sockRequest("GET", "/info", nil)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("http status code: %d", status)
	}
	if err := json.Unmarshal(b, &i); err != nil {
		return 0, err
	}
	return i.NGoroutines, nil
}

func waitForGoroutines(expected int) error {
	t := time.After(30 * time.Second)
	for {
		select {
		case <-t:
			n, err := getGoroutineNumber()
			if err != nil {
				return err
			}
			if n > expected {
				return fmt.Errorf("leaked goroutines: expected less than or equal to %d, got: %d", expected, n)
			}
		default:
			n, err := getGoroutineNumber()
			if err != nil {
				return err
			}
			if n <= expected {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// getErrorMessage returns the error message from an error API response
func getErrorMessage(c *check.C, body []byte) string {
	var resp types.ErrorResponse
	c.Assert(json.Unmarshal(body, &resp), check.IsNil)
	return strings.TrimSpace(resp.Message)
}

func waitAndAssert(c *check.C, timeout time.Duration, f checkF, checker check.Checker, args ...interface{}) {
	after := time.After(timeout)
	for {
		v, comment := f(c)
		assert, _ := checker.Check(append([]interface{}{v}, args...), checker.Info().Params)
		select {
		case <-after:
			assert = true
		default:
		}
		if assert {
			if comment != nil {
				args = append(args, comment)
			}
			c.Assert(v, checker, args...)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type checkF func(*check.C) (interface{}, check.CommentInterface)
type reducer func(...interface{}) interface{}

func reducedCheck(r reducer, funcs ...checkF) checkF {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		var values []interface{}
		var comments []string
		for _, f := range funcs {
			v, comment := f(c)
			values = append(values, v)
			if comment != nil {
				comments = append(comments, comment.CheckCommentString())
			}
		}
		return r(values...), check.Commentf("%v", strings.Join(comments, ", "))
	}
}

func sumAsIntegers(vals ...interface{}) interface{} {
	var s int
	for _, v := range vals {
		s += v.(int)
	}
	return s
}
