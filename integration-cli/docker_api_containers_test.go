package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/volume"
	"github.com/docker/go-connections/nat"
	"github.com/go-check/check"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func (s *DockerSuite) TestContainerAPIGetAll(c *check.C) {
	startCount := getContainerCount(c)
	name := "getall"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.ContainerListOptions{
		All: true,
	}
	containers, err := cli.ContainerList(context.Background(), options)
	c.Assert(err, checker.IsNil)
	c.Assert(containers, checker.HasLen, startCount+1)
	actual := containers[0].Names[0]
	c.Assert(actual, checker.Equals, "/"+name)
}

// regression test for empty json field being omitted #13691
func (s *DockerSuite) TestContainerAPIGetJSONNoFieldsOmitted(c *check.C) {
	startCount := getContainerCount(c)
	dockerCmd(c, "run", "busybox", "true")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.ContainerListOptions{
		All: true,
	}
	containers, err := cli.ContainerList(context.Background(), options)
	c.Assert(err, checker.IsNil)
	c.Assert(containers, checker.HasLen, startCount+1)
	actual := fmt.Sprintf("%+v", containers[0])

	// empty Labels field triggered this bug, make sense to check for everything
	// cause even Ports for instance can trigger this bug
	// better safe than sorry..
	fields := []string{
		"ID",
		"Names",
		"Image",
		"Command",
		"Created",
		"Ports",
		"Labels",
		"Status",
		"NetworkSettings",
	}

	// decoding into types.Container do not work since it eventually unmarshal
	// and empty field to an empty go map, so we just check for a string
	for _, f := range fields {
		if !strings.Contains(actual, f) {
			c.Fatalf("Field %s is missing and it shouldn't", f)
		}
	}
}

type containerPs struct {
	Names []string
	Ports []types.Port
}

// regression test for non-empty fields from #13901
func (s *DockerSuite) TestContainerAPIPsOmitFields(c *check.C) {
	// Problematic for Windows porting due to networking not yet being passed back
	testRequires(c, DaemonIsLinux)
	name := "pstest"
	port := 80
	runSleepingContainer(c, "--name", name, "--expose", strconv.Itoa(port))

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.ContainerListOptions{
		All: true,
	}
	containers, err := cli.ContainerList(context.Background(), options)
	c.Assert(err, checker.IsNil)
	var foundContainer containerPs
	for _, c := range containers {
		for _, testName := range c.Names {
			if "/"+name == testName {
				foundContainer.Names = c.Names
				foundContainer.Ports = c.Ports
				break
			}
		}
	}

	c.Assert(foundContainer.Ports, checker.HasLen, 1)
	c.Assert(foundContainer.Ports[0].PrivatePort, checker.Equals, uint16(port))
	c.Assert(foundContainer.Ports[0].PublicPort, checker.NotNil)
	c.Assert(foundContainer.Ports[0].IP, checker.NotNil)
}

func (s *DockerSuite) TestContainerAPIGetExport(c *check.C) {
	// Not supported on Windows as Windows does not support docker export
	testRequires(c, DaemonIsLinux)
	name := "exportcontainer"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	body, err := cli.ContainerExport(context.Background(), name)
	c.Assert(err, checker.IsNil)
	defer body.Close()
	found := false
	for tarReader := tar.NewReader(body); ; {
		h, err := tarReader.Next()
		if err != nil && err == io.EOF {
			break
		}
		if h.Name == "test" {
			found = true
			break
		}
	}
	c.Assert(found, checker.True, check.Commentf("The created test file has not been found in the exported image"))
}

func (s *DockerSuite) TestContainerAPIGetChanges(c *check.C) {
	// Not supported on Windows as Windows does not support docker diff (/containers/name/changes)
	testRequires(c, DaemonIsLinux)
	name := "changescontainer"
	dockerCmd(c, "run", "--name", name, "busybox", "rm", "/etc/passwd")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	changes, err := cli.ContainerDiff(context.Background(), name)
	c.Assert(err, checker.IsNil)

	// Check the changelog for removal of /etc/passwd
	success := false
	for _, elem := range changes {
		if elem.Path == "/etc/passwd" && elem.Kind == 2 {
			success = true
		}
	}
	c.Assert(success, checker.True, check.Commentf("/etc/passwd has been removed but is not present in the diff"))
}

func (s *DockerSuite) TestGetContainerStats(c *check.C) {
	var (
		name = "statscontainer"
	)
	runSleepingContainer(c, "--name", name)

	type b struct {
		stats types.ContainerStats
		err   error
	}

	bc := make(chan b, 1)
	go func() {
		cli, err := client.NewEnvClient()
		c.Assert(err, checker.IsNil)
		defer cli.Close()

		stats, err := cli.ContainerStats(context.Background(), name, true)
		c.Assert(err, checker.IsNil)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		dec := json.NewDecoder(sr.stats.Body)
		defer sr.stats.Body.Close()
		var s *types.Stats
		// decode only one object from the stream
		c.Assert(dec.Decode(&s), checker.IsNil)
	}
}

func (s *DockerSuite) TestGetContainerStatsRmRunning(c *check.C) {
	out := runSleepingContainer(c)
	id := strings.TrimSpace(out)

	buf := &ChannelBuffer{C: make(chan []byte, 1)}
	defer buf.Close()

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	stats, err := cli.ContainerStats(context.Background(), id, true)
	c.Assert(err, checker.IsNil)
	defer stats.Body.Close()

	chErr := make(chan error, 1)
	go func() {
		_, err = io.Copy(buf, stats.Body)
		chErr <- err
	}()

	b := make([]byte, 32)
	// make sure we've got some stats
	_, err = buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.IsNil)

	// Now remove without `-f` and make sure we are still pulling stats
	_, _, err = dockerCmdWithError("rm", id)
	c.Assert(err, checker.Not(checker.IsNil), check.Commentf("rm should have failed but didn't"))
	_, err = buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "rm", "-f", id)
	c.Assert(<-chErr, checker.IsNil)
}

// ChannelBuffer holds a chan of byte array that can be populate in a goroutine.
type ChannelBuffer struct {
	C chan []byte
}

// Write implements Writer.
func (c *ChannelBuffer) Write(b []byte) (int, error) {
	c.C <- b
	return len(b), nil
}

// Close closes the go channel.
func (c *ChannelBuffer) Close() error {
	close(c.C)
	return nil
}

// ReadTimeout reads the content of the channel in the specified byte array with
// the specified duration as timeout.
func (c *ChannelBuffer) ReadTimeout(p []byte, n time.Duration) (int, error) {
	select {
	case b := <-c.C:
		return copy(p[0:], b), nil
	case <-time.After(n):
		return -1, fmt.Errorf("timeout reading from channel")
	}
}

// regression test for gh13421
// previous test was just checking one stat entry so it didn't fail (stats with
// stream false always return one stat)
func (s *DockerSuite) TestGetContainerStatsStream(c *check.C) {
	name := "statscontainer"
	runSleepingContainer(c, "--name", name)

	type b struct {
		stats types.ContainerStats
		err   error
	}

	bc := make(chan b, 1)
	go func() {
		cli, err := client.NewEnvClient()
		c.Assert(err, checker.IsNil)
		defer cli.Close()

		stats, err := cli.ContainerStats(context.Background(), name, true)
		c.Assert(err, checker.IsNil)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		b, err := ioutil.ReadAll(sr.stats.Body)
		defer sr.stats.Body.Close()
		c.Assert(err, checker.IsNil)
		s := string(b)
		// count occurrences of "read" of types.Stats
		if l := strings.Count(s, "read"); l < 2 {
			c.Fatalf("Expected more than one stat streamed, got %d", l)
		}
	}
}

func (s *DockerSuite) TestGetContainerStatsNoStream(c *check.C) {
	name := "statscontainer"
	runSleepingContainer(c, "--name", name)

	type b struct {
		stats types.ContainerStats
		err   error
	}

	bc := make(chan b, 1)

	go func() {
		cli, err := client.NewEnvClient()
		c.Assert(err, checker.IsNil)
		defer cli.Close()

		stats, err := cli.ContainerStats(context.Background(), name, false)
		c.Assert(err, checker.IsNil)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		b, err := ioutil.ReadAll(sr.stats.Body)
		defer sr.stats.Body.Close()
		c.Assert(err, checker.IsNil)
		s := string(b)
		// count occurrences of `"read"` of types.Stats
		c.Assert(strings.Count(s, `"read"`), checker.Equals, 1, check.Commentf("Expected only one stat streamed, got %d", strings.Count(s, `"read"`)))
	}
}

func (s *DockerSuite) TestGetStoppedContainerStats(c *check.C) {
	name := "statscontainer"
	dockerCmd(c, "create", "--name", name, "busybox", "ps")

	chResp := make(chan error)

	// We expect an immediate response, but if it's not immediate, the test would hang, so put it in a goroutine
	// below we'll check this on a timeout.
	go func() {
		cli, err := client.NewEnvClient()
		c.Assert(err, checker.IsNil)
		defer cli.Close()

		resp, err := cli.ContainerStats(context.Background(), name, false)
		defer resp.Body.Close()
		chResp <- err
	}()

	select {
	case err := <-chResp:
		c.Assert(err, checker.IsNil)
	case <-time.After(10 * time.Second):
		c.Fatal("timeout waiting for stats response for stopped container")
	}
}

func (s *DockerSuite) TestContainerAPIPause(c *check.C) {
	// Problematic on Windows as Windows does not support pause
	testRequires(c, DaemonIsLinux)

	getPaused := func(c *check.C) []string {
		return strings.Fields(cli.DockerCmd(c, "ps", "-f", "status=paused", "-q", "-a").Combined())
	}

	out := cli.DockerCmd(c, "run", "-d", "busybox", "sleep", "30").Combined()
	ContainerID := strings.TrimSpace(out)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerPause(context.Background(), ContainerID)
	c.Assert(err, checker.IsNil)

	pausedContainers := getPaused(c)

	if len(pausedContainers) != 1 || stringid.TruncateID(ContainerID) != pausedContainers[0] {
		c.Fatalf("there should be one paused container and not %d", len(pausedContainers))
	}

	err = cli.ContainerUnpause(context.Background(), ContainerID)
	c.Assert(err, checker.IsNil)

	pausedContainers = getPaused(c)
	c.Assert(pausedContainers, checker.HasLen, 0, check.Commentf("There should be no paused container."))
}

func (s *DockerSuite) TestContainerAPITop(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "top")
	id := strings.TrimSpace(string(out))
	c.Assert(waitRun(id), checker.IsNil)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	top, err := cli.ContainerTop(context.Background(), id, []string{"aux"})
	c.Assert(err, checker.IsNil)
	c.Assert(top.Titles, checker.HasLen, 11, check.Commentf("expected 11 titles, found %d: %v", len(top.Titles), top.Titles))

	if top.Titles[0] != "USER" || top.Titles[10] != "COMMAND" {
		c.Fatalf("expected `USER` at `Titles[0]` and `COMMAND` at Titles[10]: %v", top.Titles)
	}
	c.Assert(top.Processes, checker.HasLen, 2, check.Commentf("expected 2 processes, found %d: %v", len(top.Processes), top.Processes))
	c.Assert(top.Processes[0][10], checker.Equals, "/bin/sh -c top")
	c.Assert(top.Processes[1][10], checker.Equals, "top")
}

func (s *DockerSuite) TestContainerAPITopWindows(c *check.C) {
	testRequires(c, DaemonIsWindows)
	out := runSleepingContainer(c, "-d")
	id := strings.TrimSpace(string(out))
	c.Assert(waitRun(id), checker.IsNil)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	top, err := cli.ContainerTop(context.Background(), id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(top.Titles, checker.HasLen, 4, check.Commentf("expected 4 titles, found %d: %v", len(top.Titles), top.Titles))

	if top.Titles[0] != "Name" || top.Titles[3] != "Private Working Set" {
		c.Fatalf("expected `Name` at `Titles[0]` and `Private Working Set` at Titles[3]: %v", top.Titles)
	}
	c.Assert(len(top.Processes), checker.GreaterOrEqualThan, 2, check.Commentf("expected at least 2 processes, found %d: %v", len(top.Processes), top.Processes))

	foundProcess := false
	expectedProcess := "busybox.exe"
	for _, process := range top.Processes {
		if process[0] == expectedProcess {
			foundProcess = true
			break
		}
	}

	c.Assert(foundProcess, checker.Equals, true, check.Commentf("expected to find %s: %v", expectedProcess, top.Processes))
}

func (s *DockerSuite) TestContainerAPICommit(c *check.C) {
	cName := "testapicommit"
	dockerCmd(c, "run", "--name="+cName, "busybox", "/bin/sh", "-c", "touch /test")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.ContainerCommitOptions{
		Reference: "testcontainerapicommit:testtag",
	}

	img, err := cli.ContainerCommit(context.Background(), cName, options)
	c.Assert(err, checker.IsNil)

	cmd := inspectField(c, img.ID, "Config.Cmd")
	c.Assert(cmd, checker.Equals, "[/bin/sh -c touch /test]", check.Commentf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	dockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerSuite) TestContainerAPICommitWithLabelInConfig(c *check.C) {
	cName := "testapicommitwithconfig"
	dockerCmd(c, "run", "--name="+cName, "busybox", "/bin/sh", "-c", "touch /test")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	config := containertypes.Config{
		Labels: map[string]string{"key1": "value1", "key2": "value2"}}

	options := types.ContainerCommitOptions{
		Reference: "testcontainerapicommitwithconfig",
		Config:    &config,
	}

	img, err := cli.ContainerCommit(context.Background(), cName, options)
	c.Assert(err, checker.IsNil)

	label1 := inspectFieldMap(c, img.ID, "Config.Labels", "key1")
	c.Assert(label1, checker.Equals, "value1")

	label2 := inspectFieldMap(c, img.ID, "Config.Labels", "key2")
	c.Assert(label2, checker.Equals, "value2")

	cmd := inspectField(c, img.ID, "Config.Cmd")
	c.Assert(cmd, checker.Equals, "[/bin/sh -c touch /test]", check.Commentf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	dockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerSuite) TestContainerAPIBadPort(c *check.C) {
	// TODO Windows to Windows CI - Port this test
	testRequires(c, DaemonIsLinux)

	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", "echo test"},
	}

	hostConfig := containertypes.HostConfig{
		PortBindings: nat.PortMap{
			"8080/tcp": []nat.PortBinding{
				{
					HostIP:   "",
					HostPort: "aa80"},
			},
		},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "")
	c.Assert(err.Error(), checker.Contains, `invalid port specification: "aa80"`)
}

func (s *DockerSuite) TestContainerAPICreate(c *check.C) {
	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", "touch /test && ls /test"},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "start", "-a", container.ID)
	c.Assert(strings.TrimSpace(out), checker.Equals, "/test")
}

func (s *DockerSuite) TestContainerAPICreateEmptyConfig(c *check.C) {

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &containertypes.Config{}, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")

	expected := "No command specified"
	c.Assert(err.Error(), checker.Contains, expected)
}

func (s *DockerSuite) TestContainerAPICreateMultipleNetworksConfig(c *check.C) {
	// Container creation must fail if client specified configurations for more than one network
	config := containertypes.Config{
		Image: "busybox",
	}

	networkingConfig := networktypes.NetworkingConfig{
		EndpointsConfig: map[string]*networktypes.EndpointSettings{
			"net1": {},
			"net2": {},
			"net3": {},
		},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networkingConfig, "")
	msg := err.Error()
	// network name order in error message is not deterministic
	c.Assert(msg, checker.Contains, "Container cannot be connected to network endpoints")
	c.Assert(msg, checker.Contains, "net1")
	c.Assert(msg, checker.Contains, "net2")
	c.Assert(msg, checker.Contains, "net3")
}

func (s *DockerSuite) TestContainerAPICreateWithHostName(c *check.C) {
	domainName := "test-domain"
	hostName := "test-hostname"
	config := containertypes.Config{
		Image:      "busybox",
		Hostname:   hostName,
		Domainname: domainName,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, checker.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, checker.IsNil)

	c.Assert(containerJSON.Config.Hostname, checker.Equals, hostName, check.Commentf("Mismatched Hostname"))
	c.Assert(containerJSON.Config.Domainname, checker.Equals, domainName, check.Commentf("Mismatched Domainname"))
}

func (s *DockerSuite) TestContainerAPICreateBridgeNetworkMode(c *check.C) {
	// Windows does not support bridge
	testRequires(c, DaemonIsLinux)
	UtilCreateNetworkMode(c, "bridge")
}

func (s *DockerSuite) TestContainerAPICreateOtherNetworkModes(c *check.C) {
	// Windows does not support these network modes
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	UtilCreateNetworkMode(c, "host")
	UtilCreateNetworkMode(c, "container:web1")
}

func UtilCreateNetworkMode(c *check.C, networkMode containertypes.NetworkMode) {
	config := containertypes.Config{
		Image: "busybox",
	}

	hostConfig := containertypes.HostConfig{
		NetworkMode: networkMode,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, checker.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, checker.IsNil)

	c.Assert(containerJSON.HostConfig.NetworkMode, checker.Equals, containertypes.NetworkMode(networkMode), check.Commentf("Mismatched NetworkMode"))
}

func (s *DockerSuite) TestContainerAPICreateWithCpuSharesCpuset(c *check.C) {
	// TODO Windows to Windows CI. The CpuShares part could be ported.
	testRequires(c, DaemonIsLinux)
	config := containertypes.Config{
		Image: "busybox",
	}

	hostConfig := containertypes.HostConfig{
		Resources: containertypes.Resources{
			CPUShares:  512,
			CpusetCpus: "0",
		},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, checker.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, checker.IsNil)

	out := inspectField(c, containerJSON.ID, "HostConfig.CpuShares")
	c.Assert(out, checker.Equals, "512")

	outCpuset := inspectField(c, containerJSON.ID, "HostConfig.CpusetCpus")
	c.Assert(outCpuset, checker.Equals, "0")
}

func (s *DockerSuite) TestContainerAPIVerifyHeader(c *check.C) {
	config := map[string]interface{}{
		"Image": "busybox",
	}

	create := func(ct string) (*http.Response, io.ReadCloser, error) {
		jsonData := bytes.NewBuffer(nil)
		c.Assert(json.NewEncoder(jsonData).Encode(config), checker.IsNil)
		return request.Post("/containers/create", request.RawContent(ioutil.NopCloser(jsonData)), request.ContentType(ct))
	}

	// Try with no content-type
	res, body, err := create("")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
	body.Close()

	// Try with wrong content-type
	res, body, err = create("application/xml")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
	body.Close()

	// now application/json
	res, body, err = create("application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)
	body.Close()
}

//Issue 14230. daemon should return 500 for invalid port syntax
func (s *DockerSuite) TestContainerAPIInvalidPortSyntax(c *check.C) {
	config := `{
				  "Image": "busybox",
				  "HostConfig": {
					"NetworkMode": "default",
					"PortBindings": {
					  "19039;1230": [
						{}
					  ]
					}
				  }
				}`

	res, body, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b[:]), checker.Contains, "invalid port")
}

func (s *DockerSuite) TestContainerAPIRestartPolicyInvalidPolicyName(c *check.C) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "something",
				"MaximumRetryCount": 0
			}
		}
	}`

	res, body, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b[:]), checker.Contains, "invalid restart policy")
}

func (s *DockerSuite) TestContainerAPIRestartPolicyRetryMismatch(c *check.C) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "always",
				"MaximumRetryCount": 2
			}
		}
	}`

	res, body, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b[:]), checker.Contains, "maximum retry count cannot be used with restart policy")
}

func (s *DockerSuite) TestContainerAPIRestartPolicyNegativeRetryCount(c *check.C) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "on-failure",
				"MaximumRetryCount": -2
			}
		}
	}`

	res, body, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b[:]), checker.Contains, "maximum retry count cannot be negative")
}

func (s *DockerSuite) TestContainerAPIRestartPolicyDefaultRetryCount(c *check.C) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "on-failure",
				"MaximumRetryCount": 0
			}
		}
	}`

	res, _, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)
}

// Issue 7941 - test to make sure a "null" in JSON is just ignored.
// W/o this fix a null in JSON would be parsed into a string var as "null"
func (s *DockerSuite) TestContainerAPIPostCreateNull(c *check.C) {
	config := `{
		"Hostname":"",
		"Domainname":"",
		"Memory":0,
		"MemorySwap":0,
		"CpuShares":0,
		"Cpuset":null,
		"AttachStdin":true,
		"AttachStdout":true,
		"AttachStderr":true,
		"ExposedPorts":{},
		"Tty":true,
		"OpenStdin":true,
		"StdinOnce":true,
		"Env":[],
		"Cmd":"ls",
		"Image":"busybox",
		"Volumes":{},
		"WorkingDir":"",
		"Entrypoint":null,
		"NetworkDisabled":false,
		"OnBuild":null}`

	res, body, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)

	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	type createResp struct {
		ID string
	}
	var container createResp
	c.Assert(json.Unmarshal(b, &container), checker.IsNil)
	out := inspectField(c, container.ID, "HostConfig.CpusetCpus")
	c.Assert(out, checker.Equals, "")

	outMemory := inspectField(c, container.ID, "HostConfig.Memory")
	c.Assert(outMemory, checker.Equals, "0")
	outMemorySwap := inspectField(c, container.ID, "HostConfig.MemorySwap")
	c.Assert(outMemorySwap, checker.Equals, "0")
}

func (s *DockerSuite) TestCreateWithTooLowMemoryLimit(c *check.C) {
	// TODO Windows: Port once memory is supported
	testRequires(c, DaemonIsLinux)
	config := `{
		"Image":     "busybox",
		"Cmd":       "ls",
		"OpenStdin": true,
		"CpuShares": 100,
		"Memory":    524287
	}`

	res, body, err := request.Post("/containers/create", request.RawString(config), request.JSON)
	c.Assert(err, checker.IsNil)
	b, err2 := request.ReadBody(body)
	c.Assert(err2, checker.IsNil)

	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
	c.Assert(string(b), checker.Contains, "Minimum memory limit allowed is 4MB")
}

func (s *DockerSuite) TestContainerAPIRename(c *check.C) {
	out, _ := dockerCmd(c, "run", "--name", "TestContainerAPIRename", "-d", "busybox", "sh")

	containerID := strings.TrimSpace(out)
	newName := "TestContainerAPIRenameNew"

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRename(context.Background(), containerID, newName)
	c.Assert(err, checker.IsNil)

	name := inspectField(c, containerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))
}

func (s *DockerSuite) TestContainerAPIKill(c *check.C) {
	name := "test-api-kill"
	runSleepingContainer(c, "-i", "--name", name)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerKill(context.Background(), name, "SIGKILL")
	c.Assert(err, checker.IsNil)

	state := inspectField(c, name, "State.Running")
	c.Assert(state, checker.Equals, "false", check.Commentf("got wrong State from container %s: %q", name, state))
}

func (s *DockerSuite) TestContainerAPIRestart(c *check.C) {
	name := "test-api-restart"
	runSleepingContainer(c, "-di", "--name", name)
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	timeout := 1 * time.Second
	err = cli.ContainerRestart(context.Background(), name, &timeout)
	c.Assert(err, checker.IsNil)

	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 15*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerAPIRestartNotimeoutParam(c *check.C) {
	name := "test-api-restart-no-timeout-param"
	out := runSleepingContainer(c, "-di", "--name", name)
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRestart(context.Background(), name, nil)
	c.Assert(err, checker.IsNil)

	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 15*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerAPIStart(c *check.C) {
	name := "testing-start"
	config := containertypes.Config{
		Image:     "busybox",
		Cmd:       append([]string{"/bin/sh", "-c"}, sleepCommandForDaemonPlatform()...),
		OpenStdin: true,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, name)
	c.Assert(err, checker.IsNil)

	err = cli.ContainerStart(context.Background(), name, types.ContainerStartOptions{})
	c.Assert(err, checker.IsNil)

	// second call to start should give 304
	// maybe add ContainerStartWithRaw to test it
	err = cli.ContainerStart(context.Background(), name, types.ContainerStartOptions{})
	c.Assert(err, checker.IsNil)

	// TODO(tibor): figure out why this doesn't work on windows
}

func (s *DockerSuite) TestContainerAPIStop(c *check.C) {
	name := "test-api-stop"
	runSleepingContainer(c, "-i", "--name", name)
	timeout := 30 * time.Second

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerStop(context.Background(), name, &timeout)
	c.Assert(err, checker.IsNil)
	c.Assert(waitInspect(name, "{{ .State.Running  }}", "false", 60*time.Second), checker.IsNil)

	// second call to start should give 304
	// maybe add ContainerStartWithRaw to test it
	err = cli.ContainerStop(context.Background(), name, &timeout)
	c.Assert(err, checker.IsNil)
}

func (s *DockerSuite) TestContainerAPIWait(c *check.C) {
	name := "test-api-wait"

	sleepCmd := "/bin/sleep"
	if testEnv.OSType == "windows" {
		sleepCmd = "sleep"
	}
	dockerCmd(c, "run", "--name", name, "busybox", sleepCmd, "2")

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	waitresC, errC := cli.ContainerWait(context.Background(), name, "")

	select {
	case err = <-errC:
		c.Assert(err, checker.IsNil)
	case waitres := <-waitresC:
		c.Assert(waitres.StatusCode, checker.Equals, int64(0))
	}
}

func (s *DockerSuite) TestContainerAPICopyNotExistsAnyMore(c *check.C) {
	name := "test-container-api-copy"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "/test.txt",
	}
	// no copy in client/
	res, _, err := request.Post("/containers/"+name+"/copy", request.JSONBody(postData))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNotFound)
}

func (s *DockerSuite) TestContainerAPICopyPre124(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	name := "test-container-api-copy"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "/test.txt",
	}

	res, body, err := request.Post("/v1.23/containers/"+name+"/copy", request.JSONBody(postData))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	found := false
	for tarReader := tar.NewReader(body); ; {
		h, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			c.Fatal(err)
		}
		if h.Name == "test.txt" {
			found = true
			break
		}
	}
	c.Assert(found, checker.True)
}

func (s *DockerSuite) TestContainerAPICopyResourcePathEmptyPre124(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	name := "test-container-api-copy-resource-empty"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "",
	}

	res, body, err := request.Post("/v1.23/containers/"+name+"/copy", request.JSONBody(postData))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Matches, "Path cannot be empty\n")
}

func (s *DockerSuite) TestContainerAPICopyResourcePathNotFoundPre124(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	name := "test-container-api-copy-resource-not-found"
	dockerCmd(c, "run", "--name", name, "busybox")

	postData := types.CopyConfig{
		Resource: "/notexist",
	}

	res, body, err := request.Post("/v1.23/containers/"+name+"/copy", request.JSONBody(postData))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNotFound)

	b, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Matches, "Could not find the file /notexist in container "+name+"\n")
}

func (s *DockerSuite) TestContainerAPICopyContainerNotFoundPr124(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	postData := types.CopyConfig{
		Resource: "/something",
	}

	res, _, err := request.Post("/v1.23/containers/notexists/copy", request.JSONBody(postData))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNotFound)
}

func (s *DockerSuite) TestContainerAPIDelete(c *check.C) {
	out := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	dockerCmd(c, "stop", id)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
	c.Assert(err, checker.IsNil)
}

func (s *DockerSuite) TestContainerAPIDeleteNotExist(c *check.C) {
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), "doesnotexist", types.ContainerRemoveOptions{})
	c.Assert(err.Error(), checker.Contains, "No such container: doesnotexist")
}

func (s *DockerSuite) TestContainerAPIDeleteForce(c *check.C) {
	out := runSleepingContainer(c)
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	removeOptions := types.ContainerRemoveOptions{
		Force: true,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), id, removeOptions)
	c.Assert(err, checker.IsNil)
}

func (s *DockerSuite) TestContainerAPIDeleteRemoveLinks(c *check.C) {
	// Windows does not support links
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--name", "tlink1", "busybox", "top")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	out, _ = dockerCmd(c, "run", "--link", "tlink1:tlink1", "--name", "tlink2", "-d", "busybox", "top")

	id2 := strings.TrimSpace(out)
	c.Assert(waitRun(id2), checker.IsNil)

	links := inspectFieldJSON(c, id2, "HostConfig.Links")
	c.Assert(links, checker.Equals, "[\"/tlink1:/tlink2/tlink1\"]", check.Commentf("expected to have links between containers"))

	removeOptions := types.ContainerRemoveOptions{
		RemoveLinks: true,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), "tlink2/tlink1", removeOptions)
	c.Assert(err, check.IsNil)

	linksPostRm := inspectFieldJSON(c, id2, "HostConfig.Links")
	c.Assert(linksPostRm, checker.Equals, "null", check.Commentf("call to api deleteContainer links should have removed the specified links"))
}

func (s *DockerSuite) TestContainerAPIDeleteConflict(c *check.C) {
	out := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
	expected := "cannot remove a running container"
	c.Assert(err.Error(), checker.Contains, expected)
}

func (s *DockerSuite) TestContainerAPIDeleteRemoveVolume(c *check.C) {
	testRequires(c, SameHostDaemon)

	vol := "/testvolume"
	if testEnv.OSType == "windows" {
		vol = `c:\testvolume`
	}

	out := runSleepingContainer(c, "-v", vol)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	source, err := inspectMountSourceField(id, vol)
	_, err = os.Stat(source)
	c.Assert(err, checker.IsNil)

	removeOptions := types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), id, removeOptions)
	c.Assert(err, check.IsNil)

	_, err = os.Stat(source)
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("expected to get ErrNotExist error, got %v", err))
}

// Regression test for https://github.com/docker/docker/issues/6231
func (s *DockerSuite) TestContainerAPIChunkedEncoding(c *check.C) {

	config := map[string]interface{}{
		"Image":     "busybox",
		"Cmd":       append([]string{"/bin/sh", "-c"}, sleepCommandForDaemonPlatform()...),
		"OpenStdin": true,
	}

	resp, _, err := request.Post("/containers/create", request.JSONBody(config), func(req *http.Request) error {
		// This is a cheat to make the http request do chunked encoding
		// Otherwise (just setting the Content-Encoding to chunked) net/http will overwrite
		// https://golang.org/src/pkg/net/http/request.go?s=11980:12172
		req.ContentLength = -1
		return nil
	})
	c.Assert(err, checker.IsNil, check.Commentf("error creating container with chunked encoding"))
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, checker.Equals, http.StatusCreated)
}

func (s *DockerSuite) TestContainerAPIPostContainerStop(c *check.C) {
	out := runSleepingContainer(c)

	containerID := strings.TrimSpace(out)
	c.Assert(waitRun(containerID), checker.IsNil)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerStop(context.Background(), containerID, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(waitInspect(containerID, "{{ .State.Running  }}", "false", 60*time.Second), checker.IsNil)
}

// #14170
func (s *DockerSuite) TestPostContainerAPICreateWithStringOrSliceEntrypoint(c *check.C) {
	config := containertypes.Config{
		Image:      "busybox",
		Entrypoint: []string{"echo"},
		Cmd:        []string{"hello", "world"},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "echotest")
	c.Assert(err, checker.IsNil)
	out, _ := dockerCmd(c, "start", "-a", "echotest")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")

	config2 := struct {
		Image      string
		Entrypoint string
		Cmd        []string
	}{"busybox", "echo", []string{"hello", "world"}}
	_, _, err = request.Post("/containers/create?name=echotest2", request.JSONBody(config2))
	c.Assert(err, checker.IsNil)
	out, _ = dockerCmd(c, "start", "-a", "echotest2")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")
}

// #14170
func (s *DockerSuite) TestPostContainersCreateWithStringOrSliceCmd(c *check.C) {
	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"echo", "hello", "world"},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "echotest")
	c.Assert(err, checker.IsNil)
	out, _ := dockerCmd(c, "start", "-a", "echotest")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")

	config2 := struct {
		Image      string
		Entrypoint string
		Cmd        string
	}{"busybox", "echo", "hello world"}
	_, _, err = request.Post("/containers/create?name=echotest2", request.JSONBody(config2))
	c.Assert(err, checker.IsNil)
	out, _ = dockerCmd(c, "start", "-a", "echotest2")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")
}

// regression #14318
func (s *DockerSuite) TestPostContainersCreateWithStringOrSliceCapAddDrop(c *check.C) {
	// Windows doesn't support CapAdd/CapDrop
	testRequires(c, DaemonIsLinux)
	config := struct {
		Image   string
		CapAdd  string
		CapDrop string
	}{"busybox", "NET_ADMIN", "SYS_ADMIN"}
	res, _, err := request.Post("/containers/create?name=capaddtest0", request.JSONBody(config))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)

	config2 := containertypes.Config{
		Image: "busybox",
	}
	hostConfig := containertypes.HostConfig{
		CapAdd:  []string{"NET_ADMIN", "SYS_ADMIN"},
		CapDrop: []string{"SETGID"},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config2, &hostConfig, &networktypes.NetworkingConfig{}, "capaddtest1")
	c.Assert(err, checker.IsNil)
}

// #14915
func (s *DockerSuite) TestContainerAPICreateNoHostConfig118(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only support 1.25 or later
	config := containertypes.Config{
		Image: "busybox",
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("v1.18"))
	c.Assert(err, checker.IsNil)

	_, err = cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, checker.IsNil)
}

// Ensure an error occurs when you have a container read-only rootfs but you
// extract an archive to a symlink in a writable volume which points to a
// directory outside of the volume.
func (s *DockerSuite) TestPutContainerArchiveErrSymlinkInVolumeToReadOnlyRootfs(c *check.C) {
	// Windows does not support read-only rootfs
	// Requires local volume mount bind.
	// --read-only + userns has remount issues
	testRequires(c, SameHostDaemon, NotUserNamespace, DaemonIsLinux)

	testVol := getTestDir(c, "test-put-container-archive-err-symlink-in-volume-to-read-only-rootfs-")
	defer os.RemoveAll(testVol)

	makeTestContentInDir(c, testVol)

	cID := makeTestContainer(c, testContainerOptions{
		readOnly: true,
		volumes:  defaultVolumes(testVol), // Our bind mount is at /vol2
	})

	// Attempt to extract to a symlink in the volume which points to a
	// directory outside the volume. This should cause an error because the
	// rootfs is read-only.
	var httpClient *http.Client
	cli, err := client.NewClient(daemonHost(), "v1.20", httpClient, map[string]string{})
	c.Assert(err, checker.IsNil)

	err = cli.CopyToContainer(context.Background(), cID, "/vol2/symlinkToAbsDir", nil, types.CopyToContainerOptions{})
	c.Assert(err.Error(), checker.Contains, "container rootfs is marked read-only")
}

func (s *DockerSuite) TestPostContainersCreateWithWrongCpusetValues(c *check.C) {
	// Not supported on Windows
	testRequires(c, DaemonIsLinux)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	config := containertypes.Config{
		Image: "busybox",
	}
	hostConfig1 := containertypes.HostConfig{
		Resources: containertypes.Resources{
			CpusetCpus: "1-42,,",
		},
	}
	name := "wrong-cpuset-cpus"

	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig1, &networktypes.NetworkingConfig{}, name)
	expected := "Invalid value 1-42,, for cpuset cpus"
	c.Assert(err.Error(), checker.Contains, expected)

	hostConfig2 := containertypes.HostConfig{
		Resources: containertypes.Resources{
			CpusetMems: "42-3,1--",
		},
	}
	name = "wrong-cpuset-mems"
	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig2, &networktypes.NetworkingConfig{}, name)
	expected = "Invalid value 42-3,1-- for cpuset mems"
	c.Assert(err.Error(), checker.Contains, expected)
}

func (s *DockerSuite) TestPostContainersCreateShmSizeNegative(c *check.C) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := containertypes.Config{
		Image: "busybox",
	}
	hostConfig := containertypes.HostConfig{
		ShmSize: -1,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "")
	c.Assert(err.Error(), checker.Contains, "SHM size can not be less than 0")
}

func (s *DockerSuite) TestPostContainersCreateShmSizeHostConfigOmitted(c *check.C) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	var defaultSHMSize int64 = 67108864
	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"mount"},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, check.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, check.IsNil)

	c.Assert(containerJSON.HostConfig.ShmSize, check.Equals, defaultSHMSize)

	out, _ := dockerCmd(c, "start", "-i", containerJSON.ID)
	shmRegexp := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegexp.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
}

func (s *DockerSuite) TestPostContainersCreateShmSizeOmitted(c *check.C) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"mount"},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, check.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, check.IsNil)

	c.Assert(containerJSON.HostConfig.ShmSize, check.Equals, int64(67108864))

	out, _ := dockerCmd(c, "start", "-i", containerJSON.ID)
	shmRegexp := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegexp.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
}

func (s *DockerSuite) TestPostContainersCreateWithShmSize(c *check.C) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"mount"},
	}

	hostConfig := containertypes.HostConfig{
		ShmSize: 1073741824,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, check.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, check.IsNil)

	c.Assert(containerJSON.HostConfig.ShmSize, check.Equals, int64(1073741824))

	out, _ := dockerCmd(c, "start", "-i", containerJSON.ID)
	shmRegex := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=1048576k`)
	if !shmRegex.MatchString(out) {
		c.Fatalf("Expected shm of 1GB in mount command, got %v", out)
	}
}

func (s *DockerSuite) TestPostContainersCreateMemorySwappinessHostConfigOmitted(c *check.C) {
	// Swappiness is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := containertypes.Config{
		Image: "busybox",
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	container, err := cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, check.IsNil)

	containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
	c.Assert(err, check.IsNil)

	c.Assert(containerJSON.HostConfig.MemorySwappiness, check.IsNil)
}

// check validation is done daemon side and not only in cli
func (s *DockerSuite) TestPostContainersCreateWithOomScoreAdjInvalidRange(c *check.C) {
	// OomScoreAdj is not supported on Windows
	testRequires(c, DaemonIsLinux)

	config := containertypes.Config{
		Image: "busybox",
	}

	hostConfig := containertypes.HostConfig{
		OomScoreAdj: 1001,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	name := "oomscoreadj-over"
	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, name)

	expected := "Invalid value 1001, range for oom score adj is [-1000, 1000]"
	c.Assert(err.Error(), checker.Contains, expected)

	hostConfig = containertypes.HostConfig{
		OomScoreAdj: -1001,
	}

	name = "oomscoreadj-low"
	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, name)

	expected = "Invalid value -1001, range for oom score adj is [-1000, 1000]"
	c.Assert(err.Error(), checker.Contains, expected)
}

// test case for #22210 where an empty container name caused panic.
func (s *DockerSuite) TestContainerAPIDeleteWithEmptyName(c *check.C) {
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ContainerRemove(context.Background(), "", types.ContainerRemoveOptions{})
	c.Assert(err.Error(), checker.Contains, "No such container")
}

func (s *DockerSuite) TestContainerAPIStatsWithNetworkDisabled(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)

	name := "testing-network-disabled"

	config := containertypes.Config{
		Image:           "busybox",
		Cmd:             []string{"top"},
		NetworkDisabled: true,
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &containertypes.HostConfig{}, &networktypes.NetworkingConfig{}, name)
	c.Assert(err, checker.IsNil)

	err = cli.ContainerStart(context.Background(), name, types.ContainerStartOptions{})
	c.Assert(err, checker.IsNil)

	c.Assert(waitRun(name), check.IsNil)

	type b struct {
		stats types.ContainerStats
		err   error
	}
	bc := make(chan b, 1)
	go func() {
		stats, err := cli.ContainerStats(context.Background(), name, false)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		c.Assert(sr.err, checker.IsNil)
		sr.stats.Body.Close()
	}
}

func (s *DockerSuite) TestContainersAPICreateMountsValidation(c *check.C) {
	type testCase struct {
		config     containertypes.Config
		hostConfig containertypes.HostConfig
		msg        string
	}

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	destPath := prefix + slash + "foo"
	notExistPath := prefix + slash + "notexist"

	cases := []testCase{
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type:   "notreal",
					Target: destPath,
				},
				},
			},

			msg: "mount type unknown",
		},
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type: "bind"}}},
			msg: "Target must not be empty",
		},
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type:   "bind",
					Target: destPath}}},
			msg: "Source must not be empty",
		},
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type:   "bind",
					Source: notExistPath,
					Target: destPath}}},
			msg: "bind source path does not exist",
		},
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type: "volume"}}},
			msg: "Target must not be empty",
		},
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type:   "volume",
					Source: "hello",
					Target: destPath}}},
			msg: "",
		},
		{
			config: containertypes.Config{
				Image: "busybox",
			},
			hostConfig: containertypes.HostConfig{
				Mounts: []mounttypes.Mount{{
					Type:   "volume",
					Source: "hello2",
					Target: destPath,
					VolumeOptions: &mounttypes.VolumeOptions{
						DriverConfig: &mounttypes.Driver{
							Name: "local"}}}}},
			msg: "",
		},
	}

	if SameHostDaemon() {
		tmpDir, err := ioutils.TempDir("", "test-mounts-api")
		c.Assert(err, checker.IsNil)
		defer os.RemoveAll(tmpDir)
		cases = append(cases, []testCase{
			{
				config: containertypes.Config{
					Image: "busybox",
				},
				hostConfig: containertypes.HostConfig{
					Mounts: []mounttypes.Mount{{
						Type:   "bind",
						Source: tmpDir,
						Target: destPath}}},
				msg: "",
			},
			{
				config: containertypes.Config{
					Image: "busybox",
				},
				hostConfig: containertypes.HostConfig{
					Mounts: []mounttypes.Mount{{
						Type:          "bind",
						Source:        tmpDir,
						Target:        destPath,
						VolumeOptions: &mounttypes.VolumeOptions{}}}},
				msg: "VolumeOptions must not be specified",
			},
		}...)
	}

	if DaemonIsLinux() {
		cases = append(cases, []testCase{
			{
				config: containertypes.Config{
					Image: "busybox",
				},
				hostConfig: containertypes.HostConfig{
					Mounts: []mounttypes.Mount{{
						Type:   "volume",
						Source: "hello3",
						Target: destPath,
						VolumeOptions: &mounttypes.VolumeOptions{
							DriverConfig: &mounttypes.Driver{
								Name:    "local",
								Options: map[string]string{"o": "size=1"}}}}}},
				msg: "",
			},
			{
				config: containertypes.Config{
					Image: "busybox",
				},
				hostConfig: containertypes.HostConfig{
					Mounts: []mounttypes.Mount{{
						Type:   "tmpfs",
						Target: destPath}}},
				msg: "",
			},
			{
				config: containertypes.Config{
					Image: "busybox",
				},
				hostConfig: containertypes.HostConfig{
					Mounts: []mounttypes.Mount{{
						Type:   "tmpfs",
						Target: destPath,
						TmpfsOptions: &mounttypes.TmpfsOptions{
							SizeBytes: 4096 * 1024,
							Mode:      0700,
						}}}},
				msg: "",
			},

			{
				config: containertypes.Config{
					Image: "busybox",
				},
				hostConfig: containertypes.HostConfig{
					Mounts: []mounttypes.Mount{{
						Type:   "tmpfs",
						Source: "/shouldnotbespecified",
						Target: destPath}}},
				msg: "Source must not be specified",
			},
		}...)

	}
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	for i, x := range cases {
		c.Logf("case %d", i)
		_, err = cli.ContainerCreate(context.Background(), &x.config, &x.hostConfig, &networktypes.NetworkingConfig{}, "")
		if len(x.msg) > 0 {
			c.Assert(err.Error(), checker.Contains, x.msg, check.Commentf("%v", cases[i].config))
		} else {
			c.Assert(err, checker.IsNil)
		}
	}
}

func (s *DockerSuite) TestContainerAPICreateMountsBindRead(c *check.C) {
	testRequires(c, NotUserNamespace, SameHostDaemon)
	// also with data in the host side
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	destPath := prefix + slash + "foo"
	tmpDir, err := ioutil.TempDir("", "test-mounts-api-bind")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(tmpDir)
	err = ioutil.WriteFile(filepath.Join(tmpDir, "bar"), []byte("hello"), 666)
	c.Assert(err, checker.IsNil)
	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", "cat /foo/bar"},
	}
	hostConfig := containertypes.HostConfig{
		Mounts: []mounttypes.Mount{
			{Type: "bind", Source: tmpDir, Target: destPath},
		},
	}
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "test")
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "start", "-a", "test")
	c.Assert(out, checker.Equals, "hello")
}

// Test Mounts comes out as expected for the MountPoint
func (s *DockerSuite) TestContainersAPICreateMountsCreate(c *check.C) {
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	destPath := prefix + slash + "foo"

	var (
		testImg string
	)
	if testEnv.OSType != "windows" {
		testImg = "test-mount-config"
		buildImageSuccessfully(c, testImg, build.WithDockerfile(`
	FROM busybox
	RUN mkdir `+destPath+` && touch `+destPath+slash+`bar
	CMD cat `+destPath+slash+`bar
	`))
	} else {
		testImg = "busybox"
	}

	type testCase struct {
		spec     mounttypes.Mount
		expected types.MountPoint
	}

	var selinuxSharedLabel string
	if runtime.GOOS == "linux" {
		selinuxSharedLabel = "z"
	}

	cases := []testCase{
		// use literal strings here for `Type` instead of the defined constants in the volume package to keep this honest
		// Validation of the actual `Mount` struct is done in another test is not needed here
		{
			spec:     mounttypes.Mount{Type: "volume", Target: destPath},
			expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mounttypes.Mount{Type: "volume", Target: destPath + slash},
			expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mounttypes.Mount{Type: "volume", Target: destPath, Source: "test1"},
			expected: types.MountPoint{Type: "volume", Name: "test1", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mounttypes.Mount{Type: "volume", Target: destPath, ReadOnly: true, Source: "test2"},
			expected: types.MountPoint{Type: "volume", Name: "test2", RW: false, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mounttypes.Mount{Type: "volume", Target: destPath, Source: "test3", VolumeOptions: &mounttypes.VolumeOptions{DriverConfig: &mounttypes.Driver{Name: volume.DefaultDriverName}}},
			expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", Name: "test3", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
	}

	if SameHostDaemon() {
		// setup temp dir for testing binds
		tmpDir1, err := ioutil.TempDir("", "test-mounts-api-1")
		c.Assert(err, checker.IsNil)
		defer os.RemoveAll(tmpDir1)
		cases = append(cases, []testCase{
			{
				spec: mounttypes.Mount{
					Type:   "bind",
					Source: tmpDir1,
					Target: destPath,
				},
				expected: types.MountPoint{
					Type:        "bind",
					RW:          true,
					Destination: destPath,
					Source:      tmpDir1,
				},
			},
			{
				spec:     mounttypes.Mount{Type: "bind", Source: tmpDir1, Target: destPath, ReadOnly: true},
				expected: types.MountPoint{Type: "bind", RW: false, Destination: destPath, Source: tmpDir1},
			},
		}...)

		// for modes only supported on Linux
		if DaemonIsLinux() {
			tmpDir3, err := ioutils.TempDir("", "test-mounts-api-3")
			c.Assert(err, checker.IsNil)
			defer os.RemoveAll(tmpDir3)

			c.Assert(mount.Mount(tmpDir3, tmpDir3, "none", "bind,rw"), checker.IsNil)
			c.Assert(mount.ForceMount("", tmpDir3, "none", "shared"), checker.IsNil)

			cases = append(cases, []testCase{
				{
					spec:     mounttypes.Mount{Type: "bind", Source: tmpDir3, Target: destPath},
					expected: types.MountPoint{Type: "bind", RW: true, Destination: destPath, Source: tmpDir3},
				},
				{
					spec:     mounttypes.Mount{Type: "bind", Source: tmpDir3, Target: destPath, ReadOnly: true},
					expected: types.MountPoint{Type: "bind", RW: false, Destination: destPath, Source: tmpDir3},
				},
				{
					spec:     mounttypes.Mount{Type: "bind", Source: tmpDir3, Target: destPath, ReadOnly: true, BindOptions: &mounttypes.BindOptions{Propagation: "shared"}},
					expected: types.MountPoint{Type: "bind", RW: false, Destination: destPath, Source: tmpDir3, Propagation: "shared"},
				},
			}...)
		}
	}

	if testEnv.OSType != "windows" { // Windows does not support volume populate
		cases = append(cases, []testCase{
			{
				spec:     mounttypes.Mount{Type: "volume", Target: destPath, VolumeOptions: &mounttypes.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
			},
			{
				spec:     mounttypes.Mount{Type: "volume", Target: destPath + slash, VolumeOptions: &mounttypes.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
			},
			{
				spec:     mounttypes.Mount{Type: "volume", Target: destPath, Source: "test4", VolumeOptions: &mounttypes.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Type: "volume", Name: "test4", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
			},
			{
				spec:     mounttypes.Mount{Type: "volume", Target: destPath, Source: "test5", ReadOnly: true, VolumeOptions: &mounttypes.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Type: "volume", Name: "test5", RW: false, Destination: destPath, Mode: selinuxSharedLabel},
			},
		}...)
	}

	type wrapper struct {
		containertypes.Config
		HostConfig containertypes.HostConfig
	}
	type createResp struct {
		ID string `json:"Id"`
	}

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	for i, x := range cases {
		c.Logf("case %d - config: %v", i, x.spec)
		container, err := apiclient.ContainerCreate(
			ctx,
			&containertypes.Config{Image: testImg},
			&containertypes.HostConfig{Mounts: []mounttypes.Mount{x.spec}},
			&networktypes.NetworkingConfig{},
			"")
		require.NoError(c, err)

		containerInspect, err := apiclient.ContainerInspect(ctx, container.ID)
		require.NoError(c, err)
		mps := containerInspect.Mounts
		require.Len(c, mps, 1)
		mountPoint := mps[0]

		if x.expected.Source != "" {
			assert.Equal(c, x.expected.Source, mountPoint.Source)
		}
		if x.expected.Name != "" {
			assert.Equal(c, x.expected.Name, mountPoint.Name)
		}
		if x.expected.Driver != "" {
			assert.Equal(c, x.expected.Driver, mountPoint.Driver)
		}
		if x.expected.Propagation != "" {
			assert.Equal(c, x.expected.Propagation, mountPoint.Propagation)
		}
		assert.Equal(c, x.expected.RW, mountPoint.RW)
		assert.Equal(c, x.expected.Type, mountPoint.Type)
		assert.Equal(c, x.expected.Mode, mountPoint.Mode)
		assert.Equal(c, x.expected.Destination, mountPoint.Destination)

		err = apiclient.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
		require.NoError(c, err)
		poll.WaitOn(c, containerExit(apiclient, container.ID), poll.WithDelay(time.Second))

		err = apiclient.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		require.NoError(c, err)

		switch {

		// Named volumes still exist after the container is removed
		case x.spec.Type == "volume" && len(x.spec.Source) > 0:
			_, err := apiclient.VolumeInspect(ctx, mountPoint.Name)
			require.NoError(c, err)

		// Bind mounts are never removed with the container
		case x.spec.Type == "bind":

		// anonymous volumes are removed
		default:
			_, err := apiclient.VolumeInspect(ctx, mountPoint.Name)
			assert.True(c, client.IsErrNotFound(err))
		}
	}
}

func containerExit(apiclient client.APIClient, name string) func(poll.LogT) poll.Result {
	return func(logT poll.LogT) poll.Result {
		container, err := apiclient.ContainerInspect(context.Background(), name)
		if err != nil {
			return poll.Error(err)
		}
		switch container.State.Status {
		case "created", "running":
			return poll.Continue("container %s is %s, waiting for exit", name, container.State.Status)
		}
		return poll.Success()
	}
}

func (s *DockerSuite) TestContainersAPICreateMountsTmpfs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	type testCase struct {
		cfg             mounttypes.Mount
		expectedOptions []string
	}
	target := "/foo"
	cases := []testCase{
		{
			cfg: mounttypes.Mount{
				Type:   "tmpfs",
				Target: target},
			expectedOptions: []string{"rw", "nosuid", "nodev", "noexec", "relatime"},
		},
		{
			cfg: mounttypes.Mount{
				Type:   "tmpfs",
				Target: target,
				TmpfsOptions: &mounttypes.TmpfsOptions{
					SizeBytes: 4096 * 1024, Mode: 0700}},
			expectedOptions: []string{"rw", "nosuid", "nodev", "noexec", "relatime", "size=4096k", "mode=700"},
		},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	config := containertypes.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", fmt.Sprintf("mount | grep 'tmpfs on %s'", target)},
	}
	for i, x := range cases {
		cName := fmt.Sprintf("test-tmpfs-%d", i)
		hostConfig := containertypes.HostConfig{
			Mounts: []mounttypes.Mount{x.cfg},
		}

		_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, cName)
		c.Assert(err, checker.IsNil)
		out, _ := dockerCmd(c, "start", "-a", cName)
		for _, option := range x.expectedOptions {
			c.Assert(out, checker.Contains, option)
		}
	}
}

// Regression test for #33334
// Makes sure that when a container which has a custom stop signal + restart=always
// gets killed (with SIGKILL) by the kill API, that the restart policy is cancelled.
func (s *DockerSuite) TestContainerKillCustomStopSignal(c *check.C) {
	id := strings.TrimSpace(runSleepingContainer(c, "--stop-signal=SIGTERM", "--restart=always"))
	res, _, err := request.Post("/containers/" + id + "/kill")
	c.Assert(err, checker.IsNil)
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent, check.Commentf(string(b)))
	err = waitInspect(id, "{{.State.Running}} {{.State.Restarting}}", "false false", 30*time.Second)
	c.Assert(err, checker.IsNil)
}
