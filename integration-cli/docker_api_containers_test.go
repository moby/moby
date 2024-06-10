package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	dconfig "github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"github.com/docker/docker/volume"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

func (s *DockerAPISuite) TestContainerAPIGetAll(c *testing.T) {
	startCount := getContainerCount(c)
	const name = "getall"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "true")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctx := testutil.GetContext(c)
	containers, err := apiClient.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	assert.NilError(c, err)
	assert.Equal(c, len(containers), startCount+1)
	actual := containers[0].Names[0]
	assert.Equal(c, actual, "/"+name)
}

// regression test for empty json field being omitted #13691
func (s *DockerAPISuite) TestContainerAPIGetJSONNoFieldsOmitted(c *testing.T) {
	startCount := getContainerCount(c)
	cli.DockerCmd(c, "run", "busybox", "true")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	options := container.ListOptions{
		All: true,
	}
	ctx := testutil.GetContext(c)
	containers, err := apiClient.ContainerList(ctx, options)
	assert.NilError(c, err)
	assert.Equal(c, len(containers), startCount+1)
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

func (s *DockerAPISuite) TestContainerAPIGetExport(c *testing.T) {
	// Not supported on Windows as Windows does not support docker export
	testRequires(c, DaemonIsLinux)
	const name = "exportcontainer"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "touch", "/test")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	body, err := apiClient.ContainerExport(testutil.GetContext(c), name)
	assert.NilError(c, err)
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
	assert.Assert(c, found, "The created test file has not been found in the exported image")
}

func (s *DockerAPISuite) TestContainerAPIGetChanges(c *testing.T) {
	// Not supported on Windows as Windows does not support docker diff (/containers/name/changes)
	testRequires(c, DaemonIsLinux)
	const name = "changescontainer"
	cli.DockerCmd(c, "run", "--name", name, "busybox", "rm", "/etc/passwd")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	changes, err := apiClient.ContainerDiff(testutil.GetContext(c), name)
	assert.NilError(c, err)

	// Check the changelog for removal of /etc/passwd
	success := false
	for _, elem := range changes {
		if elem.Path == "/etc/passwd" && elem.Kind == 2 {
			success = true
		}
	}
	assert.Assert(c, success, "/etc/passwd has been removed but is not present in the diff")
}

func (s *DockerAPISuite) TestGetContainerStats(c *testing.T) {
	const name = "statscontainer"
	runSleepingContainer(c, "--name", name)

	type b struct {
		stats types.ContainerStats
		err   error
	}

	bc := make(chan b, 1)
	go func() {
		apiClient, err := client.NewClientWithOpts(client.FromEnv)
		assert.NilError(c, err)
		defer apiClient.Close()

		stats, err := apiClient.ContainerStats(testutil.GetContext(c), name, true)
		assert.NilError(c, err)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	cli.DockerCmd(c, "rm", "-f", name)

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
		assert.NilError(c, dec.Decode(&s))
	}
}

func (s *DockerAPISuite) TestGetContainerStatsRmRunning(c *testing.T) {
	id := runSleepingContainer(c)

	buf := &ChannelBuffer{C: make(chan []byte, 1)}
	defer buf.Close()

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	stats, err := apiClient.ContainerStats(testutil.GetContext(c), id, true)
	assert.NilError(c, err)
	defer stats.Body.Close()

	chErr := make(chan error, 1)
	go func() {
		_, err = io.Copy(buf, stats.Body)
		chErr <- err
	}()

	b := make([]byte, 32)
	// make sure we've got some stats
	_, err = buf.ReadTimeout(b, 2*time.Second)
	assert.NilError(c, err)

	// Now remove without `-f` and make sure we are still pulling stats
	_, _, err = dockerCmdWithError("rm", id)
	assert.Assert(c, err != nil, "rm should have failed but didn't")
	_, err = buf.ReadTimeout(b, 2*time.Second)
	assert.NilError(c, err)

	cli.DockerCmd(c, "rm", "-f", id)
	assert.Assert(c, <-chErr == nil)
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
func (s *DockerAPISuite) TestGetContainerStatsStream(c *testing.T) {
	const name = "statscontainer"
	runSleepingContainer(c, "--name", name)

	type b struct {
		stats types.ContainerStats
		err   error
	}

	bc := make(chan b, 1)
	go func() {
		apiClient, err := client.NewClientWithOpts(client.FromEnv)
		assert.NilError(c, err)
		defer apiClient.Close()

		stats, err := apiClient.ContainerStats(testutil.GetContext(c), name, true)
		assert.NilError(c, err)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	cli.DockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		b, err := io.ReadAll(sr.stats.Body)
		defer sr.stats.Body.Close()
		assert.NilError(c, err)
		s := string(b)
		// count occurrences of "read" of types.Stats
		if l := strings.Count(s, "read"); l < 2 {
			c.Fatalf("Expected more than one stat streamed, got %d", l)
		}
	}
}

func (s *DockerAPISuite) TestGetContainerStatsNoStream(c *testing.T) {
	const name = "statscontainer2"
	runSleepingContainer(c, "--name", name)

	type b struct {
		stats types.ContainerStats
		err   error
	}

	bc := make(chan b, 1)

	go func() {
		apiClient, err := client.NewClientWithOpts(client.FromEnv)
		assert.NilError(c, err)
		defer apiClient.Close()

		stats, err := apiClient.ContainerStats(testutil.GetContext(c), name, false)
		assert.NilError(c, err)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	cli.DockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		b, err := io.ReadAll(sr.stats.Body)
		defer sr.stats.Body.Close()
		assert.NilError(c, err)
		s := string(b)
		// count occurrences of `"read"` of types.Stats
		assert.Assert(c, strings.Count(s, `"read"`) == 1, "Expected only one stat streamed, got %d", strings.Count(s, `"read"`))
	}
}

func (s *DockerAPISuite) TestGetStoppedContainerStats(c *testing.T) {
	const name = "statscontainer3"
	cli.DockerCmd(c, "create", "--name", name, "busybox", "ps")

	chResp := make(chan error, 1)

	// We expect an immediate response, but if it's not immediate, the test would hang, so put it in a goroutine
	// below we'll check this on a timeout.
	go func() {
		apiClient, err := client.NewClientWithOpts(client.FromEnv)
		assert.NilError(c, err)
		defer apiClient.Close()

		resp, err := apiClient.ContainerStats(testutil.GetContext(c), name, false)
		assert.NilError(c, err)
		defer resp.Body.Close()
		chResp <- err
	}()

	select {
	case err := <-chResp:
		assert.NilError(c, err)
	case <-time.After(10 * time.Second):
		c.Fatal("timeout waiting for stats response for stopped container")
	}
}

func (s *DockerAPISuite) TestContainerAPIPause(c *testing.T) {
	// Problematic on Windows as Windows does not support pause
	testRequires(c, DaemonIsLinux)

	getPaused := func(c *testing.T) []string {
		return strings.Fields(cli.DockerCmd(c, "ps", "-f", "status=paused", "-q", "-a").Combined())
	}

	out := cli.DockerCmd(c, "run", "-d", "busybox", "sleep", "30").Combined()
	ContainerID := strings.TrimSpace(out)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerPause(testutil.GetContext(c), ContainerID)
	assert.NilError(c, err)

	pausedContainers := getPaused(c)

	if len(pausedContainers) != 1 || stringid.TruncateID(ContainerID) != pausedContainers[0] {
		c.Fatalf("there should be one paused container and not %d", len(pausedContainers))
	}

	err = apiClient.ContainerUnpause(testutil.GetContext(c), ContainerID)
	assert.NilError(c, err)

	pausedContainers = getPaused(c)
	assert.Equal(c, len(pausedContainers), 0, "There should be no paused container.")
}

func (s *DockerAPISuite) TestContainerAPITop(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out := cli.DockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "top && true").Stdout()
	id := strings.TrimSpace(out)
	cli.WaitRun(c, id)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	// sort by comm[andline] to make sure order stays the same in case of PID rollover
	top, err := apiClient.ContainerTop(testutil.GetContext(c), id, []string{"aux", "--sort=comm"})
	assert.NilError(c, err)
	assert.Equal(c, len(top.Titles), 11, fmt.Sprintf("expected 11 titles, found %d: %v", len(top.Titles), top.Titles))

	if top.Titles[0] != "USER" || top.Titles[10] != "COMMAND" {
		c.Fatalf("expected `USER` at `Titles[0]` and `COMMAND` at Titles[10]: %v", top.Titles)
	}
	assert.Equal(c, len(top.Processes), 2, fmt.Sprintf("expected 2 processes, found %d: %v", len(top.Processes), top.Processes))
	assert.Equal(c, top.Processes[0][10], "/bin/sh -c top && true")
	assert.Equal(c, top.Processes[1][10], "top")
}

func (s *DockerAPISuite) TestContainerAPITopWindows(c *testing.T) {
	testRequires(c, DaemonIsWindows)
	id := runSleepingContainer(c, "-d")
	cli.WaitRun(c, id)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	top, err := apiClient.ContainerTop(testutil.GetContext(c), id, nil)
	assert.NilError(c, err)
	assert.Equal(c, len(top.Titles), 4, "expected 4 titles, found %d: %v", len(top.Titles), top.Titles)

	if top.Titles[0] != "Name" || top.Titles[3] != "Private Working Set" {
		c.Fatalf("expected `Name` at `Titles[0]` and `Private Working Set` at Titles[3]: %v", top.Titles)
	}
	assert.Assert(c, len(top.Processes) >= 2, "expected at least 2 processes, found %d: %v", len(top.Processes), top.Processes)

	foundProcess := false
	expectedProcess := "busybox.exe"
	for _, process := range top.Processes {
		if process[0] == expectedProcess {
			foundProcess = true
			break
		}
	}

	assert.Assert(c, foundProcess, "expected to find %s: %v", expectedProcess, top.Processes)
}

func (s *DockerAPISuite) TestContainerAPICommit(c *testing.T) {
	const cName = "testapicommit"
	cli.DockerCmd(c, "run", "--name="+cName, "busybox", "/bin/sh", "-c", "touch /test")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	options := container.CommitOptions{
		Reference: "testcontainerapicommit:testtag",
	}

	img, err := apiClient.ContainerCommit(testutil.GetContext(c), cName, options)
	assert.NilError(c, err)

	cmd := inspectField(c, img.ID, "Config.Cmd")
	assert.Equal(c, cmd, "[/bin/sh -c touch /test]", fmt.Sprintf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	cli.DockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerAPISuite) TestContainerAPICommitWithLabelInConfig(c *testing.T) {
	const cName = "testapicommitwithconfig"
	cli.DockerCmd(c, "run", "--name="+cName, "busybox", "/bin/sh", "-c", "touch /test")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	config := container.Config{
		Labels: map[string]string{"key1": "value1", "key2": "value2"},
	}

	options := container.CommitOptions{
		Reference: "testcontainerapicommitwithconfig",
		Config:    &config,
	}

	img, err := apiClient.ContainerCommit(testutil.GetContext(c), cName, options)
	assert.NilError(c, err)

	label1 := inspectFieldMap(c, img.ID, "Config.Labels", "key1")
	assert.Equal(c, label1, "value1")

	label2 := inspectFieldMap(c, img.ID, "Config.Labels", "key2")
	assert.Equal(c, label2, "value2")

	cmd := inspectField(c, img.ID, "Config.Cmd")
	assert.Equal(c, cmd, "[/bin/sh -c touch /test]", fmt.Sprintf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	cli.DockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerAPISuite) TestContainerAPIBadPort(c *testing.T) {
	// TODO Windows to Windows CI - Port this test
	testRequires(c, DaemonIsLinux)

	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", "echo test"},
	}

	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"8080/tcp": []nat.PortBinding{
				{
					HostIP:   "",
					HostPort: "aa80",
				},
			},
		},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "")
	assert.ErrorContains(c, err, `invalid port specification: "aa80"`)
}

func (s *DockerAPISuite) TestContainerAPICreate(c *testing.T) {
	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", "touch /test && ls /test"},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	out := cli.DockerCmd(c, "start", "-a", ctr.ID).Stdout()
	assert.Equal(c, strings.TrimSpace(out), "/test")
}

func (s *DockerAPISuite) TestContainerAPICreateEmptyConfig(c *testing.T) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &container.Config{}, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")

	assert.ErrorContains(c, err, "no command specified")
}

func (s *DockerAPISuite) TestContainerAPICreateBridgeNetworkMode(c *testing.T) {
	// Windows does not support bridge
	testRequires(c, DaemonIsLinux)
	UtilCreateNetworkMode(c, "bridge")
}

func (s *DockerAPISuite) TestContainerAPICreateOtherNetworkModes(c *testing.T) {
	// Windows does not support these network modes
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	UtilCreateNetworkMode(c, "host")
	UtilCreateNetworkMode(c, "container:web1")
}

func UtilCreateNetworkMode(c *testing.T, networkMode container.NetworkMode) {
	config := container.Config{
		Image: "busybox",
	}

	hostConfig := container.HostConfig{
		NetworkMode: networkMode,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	containerJSON, err := apiClient.ContainerInspect(testutil.GetContext(c), ctr.ID)
	assert.NilError(c, err)

	assert.Equal(c, containerJSON.HostConfig.NetworkMode, networkMode, "Mismatched NetworkMode")
}

func (s *DockerAPISuite) TestContainerAPICreateWithCpuSharesCpuset(c *testing.T) {
	// TODO Windows to Windows CI. The CpuShares part could be ported.
	testRequires(c, DaemonIsLinux)
	config := container.Config{
		Image: "busybox",
	}

	hostConfig := container.HostConfig{
		Resources: container.Resources{
			CPUShares:  512,
			CpusetCpus: "0",
		},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	containerJSON, err := apiClient.ContainerInspect(testutil.GetContext(c), ctr.ID)
	assert.NilError(c, err)

	out := inspectField(c, containerJSON.ID, "HostConfig.CpuShares")
	assert.Equal(c, out, "512")

	outCpuset := inspectField(c, containerJSON.ID, "HostConfig.CpusetCpus")
	assert.Equal(c, outCpuset, "0")
}

func (s *DockerAPISuite) TestContainerAPIVerifyHeader(c *testing.T) {
	config := map[string]interface{}{
		"Image": "busybox",
	}

	create := func(ct string) (*http.Response, io.ReadCloser, error) {
		jsonData := bytes.NewBuffer(nil)
		assert.Assert(c, json.NewEncoder(jsonData).Encode(config) == nil)
		return request.Post(testutil.GetContext(c), "/containers/create", request.RawContent(io.NopCloser(jsonData)), request.ContentType(ct))
	}

	// Try with no content-type
	res, body, err := create("")
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)
	_ = body.Close()

	// Try with wrong content-type
	res, body, err = create("application/xml")
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)
	_ = body.Close()

	// now application/json
	res, body, err = create("application/json")
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)
	_ = body.Close()
}

// Issue 14230. daemon should return 500 for invalid port syntax
func (s *DockerAPISuite) TestContainerAPIInvalidPortSyntax(c *testing.T) {
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

	res, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(string(b[:]), "invalid port"))
}

func (s *DockerAPISuite) TestContainerAPIRestartPolicyInvalidPolicyName(c *testing.T) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "something",
				"MaximumRetryCount": 0
			}
		}
	}`

	res, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(string(b[:]), "invalid restart policy"))
}

func (s *DockerAPISuite) TestContainerAPIRestartPolicyRetryMismatch(c *testing.T) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "always",
				"MaximumRetryCount": 2
			}
		}
	}`

	res, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(string(b[:]), "invalid restart policy: maximum retry count can only be used with 'on-failure'"))
}

func (s *DockerAPISuite) TestContainerAPIRestartPolicyNegativeRetryCount(c *testing.T) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "on-failure",
				"MaximumRetryCount": -2
			}
		}
	}`

	res, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)

	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(string(b[:]), "maximum retry count cannot be negative"))
}

func (s *DockerAPISuite) TestContainerAPIRestartPolicyDefaultRetryCount(c *testing.T) {
	config := `{
		"Image": "busybox",
		"HostConfig": {
			"RestartPolicy": {
				"Name": "on-failure",
				"MaximumRetryCount": 0
			}
		}
	}`

	res, _, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)
}

// Issue 7941 - test to make sure a "null" in JSON is just ignored.
// W/o this fix a null in JSON would be parsed into a string var as "null"
func (s *DockerAPISuite) TestContainerAPIPostCreateNull(c *testing.T) {
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

	res, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)

	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	type createResp struct {
		ID string
	}
	var ctr createResp
	assert.Assert(c, json.Unmarshal(b, &ctr) == nil)
	out := inspectField(c, ctr.ID, "HostConfig.CpusetCpus")
	assert.Equal(c, out, "")

	outMemory := inspectField(c, ctr.ID, "HostConfig.Memory")
	assert.Equal(c, outMemory, "0")
	outMemorySwap := inspectField(c, ctr.ID, "HostConfig.MemorySwap")
	assert.Equal(c, outMemorySwap, "0")
}

func (s *DockerAPISuite) TestCreateWithTooLowMemoryLimit(c *testing.T) {
	// TODO Windows: Port once memory is supported
	testRequires(c, DaemonIsLinux)
	config := `{
		"Image":     "busybox",
		"Cmd":       "ls",
		"OpenStdin": true,
		"CpuShares": 100,
		"Memory":    524287
	}`

	res, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.RawString(config), request.JSON)
	assert.NilError(c, err)
	b, err2 := request.ReadBody(body)
	assert.Assert(c, err2 == nil)

	assert.Equal(c, res.StatusCode, http.StatusBadRequest)
	assert.Assert(c, strings.Contains(string(b), "Minimum memory limit allowed is 6MB"))
}

func (s *DockerAPISuite) TestContainerAPIRename(c *testing.T) {
	out := cli.DockerCmd(c, "run", "--name", "TestContainerAPIRename", "-d", "busybox", "sh").Stdout()
	containerID := strings.TrimSpace(out)
	const newName = "TestContainerAPIRenameNew"

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRename(testutil.GetContext(c), containerID, newName)
	assert.NilError(c, err)

	name := inspectField(c, containerID, "Name")
	assert.Equal(c, name, "/"+newName, "Failed to rename container")
}

func (s *DockerAPISuite) TestContainerAPIKill(c *testing.T) {
	const name = "test-api-kill"
	runSleepingContainer(c, "-i", "--name", name)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerKill(testutil.GetContext(c), name, "SIGKILL")
	assert.NilError(c, err)

	state := inspectField(c, name, "State.Running")
	assert.Equal(c, state, "false", fmt.Sprintf("got wrong State from container %s: %q", name, state))
}

func (s *DockerAPISuite) TestContainerAPIRestart(c *testing.T) {
	const name = "test-api-restart"
	runSleepingContainer(c, "-di", "--name", name)
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	timeout := 1
	err = apiClient.ContainerRestart(testutil.GetContext(c), name, container.StopOptions{Timeout: &timeout})
	assert.NilError(c, err)

	assert.Assert(c, waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 15*time.Second) == nil)
}

func (s *DockerAPISuite) TestContainerAPIRestartNotimeoutParam(c *testing.T) {
	const name = "test-api-restart-no-timeout-param"
	id := runSleepingContainer(c, "-di", "--name", name)
	cli.WaitRun(c, id)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRestart(testutil.GetContext(c), name, container.StopOptions{})
	assert.NilError(c, err)

	assert.Assert(c, waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 15*time.Second) == nil)
}

func (s *DockerAPISuite) TestContainerAPIStart(c *testing.T) {
	const name = "testing-start"
	config := container.Config{
		Image:     "busybox",
		Cmd:       append([]string{"/bin/sh", "-c"}, sleepCommandForDaemonPlatform()...),
		OpenStdin: true,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, name)
	assert.NilError(c, err)

	err = apiClient.ContainerStart(testutil.GetContext(c), name, container.StartOptions{})
	assert.NilError(c, err)

	// second call to start should give 304
	// maybe add ContainerStartWithRaw to test it
	err = apiClient.ContainerStart(testutil.GetContext(c), name, container.StartOptions{})
	assert.NilError(c, err)

	// TODO(tibor): figure out why this doesn't work on windows
}

func (s *DockerAPISuite) TestContainerAPIStop(c *testing.T) {
	const name = "test-api-stop"
	runSleepingContainer(c, "-i", "--name", name)
	timeout := 30

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerStop(testutil.GetContext(c), name, container.StopOptions{
		Timeout: &timeout,
	})
	assert.NilError(c, err)
	assert.Assert(c, waitInspect(name, "{{ .State.Running  }}", "false", 60*time.Second) == nil)

	// second call to start should give 304
	// maybe add ContainerStartWithRaw to test it
	err = apiClient.ContainerStop(testutil.GetContext(c), name, container.StopOptions{
		Timeout: &timeout,
	})
	assert.NilError(c, err)
}

func (s *DockerAPISuite) TestContainerAPIWait(c *testing.T) {
	const name = "test-api-wait"

	sleepCmd := "/bin/sleep"
	if testEnv.DaemonInfo.OSType == "windows" {
		sleepCmd = "sleep"
	}
	cli.DockerCmd(c, "run", "--name", name, "busybox", sleepCmd, "2")

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	waitResC, errC := apiClient.ContainerWait(testutil.GetContext(c), name, "")

	select {
	case err = <-errC:
		assert.NilError(c, err)
	case waitRes := <-waitResC:
		assert.Equal(c, waitRes.StatusCode, int64(0))
	}
}

func (s *DockerAPISuite) TestContainerAPIDelete(c *testing.T) {
	id := runSleepingContainer(c)
	cli.WaitRun(c, id)
	cli.DockerCmd(c, "stop", id)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRemove(testutil.GetContext(c), id, container.RemoveOptions{})
	assert.NilError(c, err)
}

func (s *DockerAPISuite) TestContainerAPIDeleteNotExist(c *testing.T) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRemove(testutil.GetContext(c), "doesnotexist", container.RemoveOptions{})
	assert.ErrorContains(c, err, "No such container: doesnotexist")
}

func (s *DockerAPISuite) TestContainerAPIDeleteForce(c *testing.T) {
	id := runSleepingContainer(c)
	cli.WaitRun(c, id)

	removeOptions := container.RemoveOptions{
		Force: true,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRemove(testutil.GetContext(c), id, removeOptions)
	assert.NilError(c, err)
}

func (s *DockerAPISuite) TestContainerAPIDeleteRemoveLinks(c *testing.T) {
	// Windows does not support links
	testRequires(c, DaemonIsLinux)
	out := cli.DockerCmd(c, "run", "-d", "--name", "tlink1", "busybox", "top").Stdout()
	id := strings.TrimSpace(out)
	cli.WaitRun(c, id)

	out = cli.DockerCmd(c, "run", "--link", "tlink1:tlink1", "--name", "tlink2", "-d", "busybox", "top").Stdout()
	id2 := strings.TrimSpace(out)
	cli.WaitRun(c, id2)

	links := inspectFieldJSON(c, id2, "HostConfig.Links")
	assert.Equal(c, links, `["/tlink1:/tlink2/tlink1"]`, "expected to have links between containers")

	removeOptions := container.RemoveOptions{
		RemoveLinks: true,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRemove(testutil.GetContext(c), "tlink2/tlink1", removeOptions)
	assert.NilError(c, err)

	linksPostRm := inspectFieldJSON(c, id2, "HostConfig.Links")
	assert.Equal(c, linksPostRm, "null", "call to api deleteContainer links should have removed the specified links")
}

func (s *DockerAPISuite) TestContainerAPIDeleteRemoveVolume(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon)

	vol := "/testvolume"
	if testEnv.DaemonInfo.OSType == "windows" {
		vol = `c:\testvolume`
	}

	id := runSleepingContainer(c, "-v", vol)
	cli.WaitRun(c, id)

	source, err := inspectMountSourceField(id, vol)
	assert.NilError(c, err)
	_, err = os.Stat(source)
	assert.NilError(c, err)

	removeOptions := container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRemove(testutil.GetContext(c), id, removeOptions)
	assert.NilError(c, err)

	_, err = os.Stat(source)
	assert.Assert(c, os.IsNotExist(err), "expected to get ErrNotExist error, got %v", err)
}

// Regression test for https://github.com/docker/docker/issues/6231
func (s *DockerAPISuite) TestContainerAPIChunkedEncoding(c *testing.T) {
	config := map[string]interface{}{
		"Image":     "busybox",
		"Cmd":       append([]string{"/bin/sh", "-c"}, sleepCommandForDaemonPlatform()...),
		"OpenStdin": true,
	}

	resp, _, err := request.Post(testutil.GetContext(c), "/containers/create", request.JSONBody(config), request.With(func(req *http.Request) error {
		// This is a cheat to make the http request do chunked encoding
		// Otherwise (just setting the Content-Encoding to chunked) net/http will overwrite
		// https://golang.org/src/pkg/net/http/request.go?s=11980:12172
		req.ContentLength = -1
		return nil
	}))
	assert.Assert(c, err == nil, "error creating container with chunked encoding")
	defer resp.Body.Close()
	assert.Equal(c, resp.StatusCode, http.StatusCreated)
}

func (s *DockerAPISuite) TestContainerAPIPostContainerStop(c *testing.T) {
	containerID := runSleepingContainer(c)
	cli.WaitRun(c, containerID)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerStop(testutil.GetContext(c), containerID, container.StopOptions{})
	assert.NilError(c, err)
	assert.Assert(c, waitInspect(containerID, "{{ .State.Running  }}", "false", 60*time.Second) == nil)
}

// #14170
func (s *DockerAPISuite) TestPostContainerAPICreateWithStringOrSliceEntrypoint(c *testing.T) {
	config := container.Config{
		Image:      "busybox",
		Entrypoint: []string{"echo"},
		Cmd:        []string{"hello", "world"},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "echotest")
	assert.NilError(c, err)
	out := cli.DockerCmd(c, "start", "-a", "echotest").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello world")

	config2 := struct {
		Image      string
		Entrypoint string
		Cmd        []string
	}{"busybox", "echo", []string{"hello", "world"}}
	_, _, err = request.Post(testutil.GetContext(c), "/containers/create?name=echotest2", request.JSONBody(config2))
	assert.NilError(c, err)
	out = cli.DockerCmd(c, "start", "-a", "echotest2").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello world")
}

// #14170
func (s *DockerAPISuite) TestPostContainersCreateWithStringOrSliceCmd(c *testing.T) {
	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"echo", "hello", "world"},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "echotest")
	assert.NilError(c, err)
	out := cli.DockerCmd(c, "start", "-a", "echotest").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello world")

	config2 := struct {
		Image      string
		Entrypoint string
		Cmd        string
	}{"busybox", "echo", "hello world"}
	_, _, err = request.Post(testutil.GetContext(c), "/containers/create?name=echotest2", request.JSONBody(config2))
	assert.NilError(c, err)
	out = cli.DockerCmd(c, "start", "-a", "echotest2").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello world")
}

// regression #14318
// for backward compatibility testing with and without CAP_ prefix
// and with upper and lowercase
func (s *DockerAPISuite) TestPostContainersCreateWithStringOrSliceCapAddDrop(c *testing.T) {
	// Windows doesn't support CapAdd/CapDrop
	testRequires(c, DaemonIsLinux)
	config := struct {
		Image   string
		CapAdd  string
		CapDrop string
	}{"busybox", "NET_ADMIN", "cap_sys_admin"}
	res, _, err := request.Post(testutil.GetContext(c), "/containers/create?name=capaddtest0", request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)

	config2 := container.Config{
		Image: "busybox",
	}
	hostConfig := container.HostConfig{
		CapAdd:  []string{"net_admin", "SYS_ADMIN"},
		CapDrop: []string{"SETGID", "CAP_SETPCAP"},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config2, &hostConfig, &network.NetworkingConfig{}, nil, "capaddtest1")
	assert.NilError(c, err)
}

// Ensure an error occurs when you have a container read-only rootfs but you
// extract an archive to a symlink in a writable volume which points to a
// directory outside of the volume.
func (s *DockerAPISuite) TestPutContainerArchiveErrSymlinkInVolumeToReadOnlyRootfs(c *testing.T) {
	// Windows does not support read-only rootfs
	// Requires local volume mount bind.
	// --read-only + userns has remount issues
	testRequires(c, testEnv.IsLocalDaemon, NotUserNamespace, DaemonIsLinux)

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
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)

	err = apiClient.CopyToContainer(testutil.GetContext(c), cID, "/vol2/symlinkToAbsDir", nil, types.CopyToContainerOptions{})
	assert.ErrorContains(c, err, "container rootfs is marked read-only")
}

func (s *DockerAPISuite) TestPostContainersCreateWithWrongCpusetValues(c *testing.T) {
	// Not supported on Windows
	testRequires(c, DaemonIsLinux)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	config := container.Config{
		Image: "busybox",
	}
	hostConfig1 := container.HostConfig{
		Resources: container.Resources{
			CpusetCpus: "1-42,,",
		},
	}
	const name = "wrong-cpuset-cpus"

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig1, &network.NetworkingConfig{}, nil, name)
	expected := "Invalid value 1-42,, for cpuset cpus"
	assert.ErrorContains(c, err, expected)

	hostConfig2 := container.HostConfig{
		Resources: container.Resources{
			CpusetMems: "42-3,1--",
		},
	}
	const name2 = "wrong-cpuset-mems"
	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig2, &network.NetworkingConfig{}, nil, name2)
	expected = "Invalid value 42-3,1-- for cpuset mems"
	assert.ErrorContains(c, err, expected)
}

func (s *DockerAPISuite) TestPostContainersCreateShmSizeNegative(c *testing.T) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := container.Config{
		Image: "busybox",
	}
	hostConfig := container.HostConfig{
		ShmSize: -1,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "")
	assert.ErrorContains(c, err, "SHM size can not be less than 0")
}

func (s *DockerAPISuite) TestPostContainersCreateShmSizeHostConfigOmitted(c *testing.T) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)

	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"mount"},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	containerJSON, err := apiClient.ContainerInspect(testutil.GetContext(c), ctr.ID)
	assert.NilError(c, err)

	assert.Equal(c, containerJSON.HostConfig.ShmSize, dconfig.DefaultShmSize)

	out := cli.DockerCmd(c, "start", "-i", containerJSON.ID).Combined()
	shmRegexp := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegexp.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
}

func (s *DockerAPISuite) TestPostContainersCreateShmSizeOmitted(c *testing.T) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"mount"},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	containerJSON, err := apiClient.ContainerInspect(testutil.GetContext(c), ctr.ID)
	assert.NilError(c, err)

	assert.Equal(c, containerJSON.HostConfig.ShmSize, int64(67108864))

	out := cli.DockerCmd(c, "start", "-i", containerJSON.ID).Combined()
	shmRegexp := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegexp.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
}

func (s *DockerAPISuite) TestPostContainersCreateWithShmSize(c *testing.T) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"mount"},
	}

	hostConfig := container.HostConfig{
		ShmSize: 1073741824,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	containerJSON, err := apiClient.ContainerInspect(testutil.GetContext(c), ctr.ID)
	assert.NilError(c, err)

	assert.Equal(c, containerJSON.HostConfig.ShmSize, int64(1073741824))

	out := cli.DockerCmd(c, "start", "-i", containerJSON.ID).Combined()
	shmRegex := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=1048576k`)
	if !shmRegex.MatchString(out) {
		c.Fatalf("Expected shm of 1GB in mount command, got %v", out)
	}
}

func (s *DockerAPISuite) TestPostContainersCreateMemorySwappinessHostConfigOmitted(c *testing.T) {
	// Swappiness is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := container.Config{
		Image: "busybox",
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	ctr, err := apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")
	assert.NilError(c, err)

	containerJSON, err := apiClient.ContainerInspect(testutil.GetContext(c), ctr.ID)
	assert.NilError(c, err)

	assert.Assert(c, containerJSON.HostConfig.MemorySwappiness == nil)
}

// check validation is done daemon side and not only in cli
func (s *DockerAPISuite) TestPostContainersCreateWithOomScoreAdjInvalidRange(c *testing.T) {
	// OomScoreAdj is not supported on Windows
	testRequires(c, DaemonIsLinux)

	config := container.Config{
		Image: "busybox",
	}

	hostConfig := container.HostConfig{
		OomScoreAdj: 1001,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	const name = "oomscoreadj-over"
	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, name)

	expected := "Invalid value 1001, range for oom score adj is [-1000, 1000]"
	assert.ErrorContains(c, err, expected)

	hostConfig = container.HostConfig{
		OomScoreAdj: -1001,
	}

	const name2 = "oomscoreadj-low"
	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, name2)

	expected = "Invalid value -1001, range for oom score adj is [-1000, 1000]"
	assert.ErrorContains(c, err, expected)
}

// test case for #22210 where an empty container name caused panic.
func (s *DockerAPISuite) TestContainerAPIDeleteWithEmptyName(c *testing.T) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	err = apiClient.ContainerRemove(testutil.GetContext(c), "", container.RemoveOptions{})
	assert.Check(c, errdefs.IsNotFound(err))
}

func (s *DockerAPISuite) TestContainerAPIStatsWithNetworkDisabled(c *testing.T) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)

	const name = "testing-network-disabled"

	config := container.Config{
		Image:           "busybox",
		Cmd:             []string{"top"},
		NetworkDisabled: true,
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &container.HostConfig{}, &network.NetworkingConfig{}, nil, name)
	assert.NilError(c, err)

	err = apiClient.ContainerStart(testutil.GetContext(c), name, container.StartOptions{})
	assert.NilError(c, err)
	cli.WaitRun(c, name)

	type b struct {
		stats types.ContainerStats
		err   error
	}
	bc := make(chan b, 1)
	go func() {
		stats, err := apiClient.ContainerStats(testutil.GetContext(c), name, false)
		bc <- b{stats, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	cli.DockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		assert.Assert(c, sr.err == nil)
		sr.stats.Body.Close()
	}
}

func (s *DockerAPISuite) TestContainersAPICreateMountsValidation(c *testing.T) {
	type testCase struct {
		config     container.Config
		hostConfig container.HostConfig
		msg        string
	}

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	destPath := prefix + slash + "foo"
	notExistPath := prefix + slash + "notexist"

	cases := []testCase{
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type:   "notreal",
					Target: destPath,
				}},
			},

			msg: "mount type unknown",
		},
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type: "bind",
				}},
			},
			msg: "Target must not be empty",
		},
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type:   "bind",
					Target: destPath,
				}},
			},
			msg: "Source must not be empty",
		},
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type:   "bind",
					Source: notExistPath,
					Target: destPath,
				}},
			},
			msg: "source path does not exist",
			// FIXME(vdemeester) fails into e2e, migrate to integration/container anyway
			// msg: "source path does not exist: " + notExistPath,
		},
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type: "volume",
				}},
			},
			msg: "Target must not be empty",
		},
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type:   "volume",
					Source: "hello",
					Target: destPath,
				}},
			},
			msg: "",
		},
		{
			config: container.Config{
				Image: "busybox",
			},
			hostConfig: container.HostConfig{
				Mounts: []mount.Mount{{
					Type:   "volume",
					Source: "hello2",
					Target: destPath,
					VolumeOptions: &mount.VolumeOptions{
						DriverConfig: &mount.Driver{
							Name: "local",
						},
					},
				}},
			},
			msg: "",
		},
	}

	if testEnv.IsLocalDaemon() {
		tmpDir, err := os.MkdirTemp("", "test-mounts-api")
		assert.NilError(c, err)
		defer os.RemoveAll(tmpDir)
		cases = append(cases, []testCase{
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{{
						Type:   "bind",
						Source: tmpDir,
						Target: destPath,
					}},
				},
				msg: "",
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{{
						Type:          "bind",
						Source:        tmpDir,
						Target:        destPath,
						VolumeOptions: &mount.VolumeOptions{},
					}},
				},
				msg: "VolumeOptions must not be specified",
			},
		}...)
	}

	if DaemonIsWindows() {
		cases = append(cases, []testCase{
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{
						{
							Type:   "volume",
							Source: "not-supported-on-windows",
							Target: destPath,
							VolumeOptions: &mount.VolumeOptions{
								DriverConfig: &mount.Driver{
									Name:    "local",
									Options: map[string]string{"type": "tmpfs"},
								},
							},
						},
					},
				},
				msg: `options are not supported on this platform`,
			},
		}...)
	}

	if DaemonIsLinux() {
		cases = append(cases, []testCase{
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{
						{
							Type:   "volume",
							Source: "missing-device-opt",
							Target: destPath,
							VolumeOptions: &mount.VolumeOptions{
								DriverConfig: &mount.Driver{
									Name:    "local",
									Options: map[string]string{"foobar": "foobaz"},
								},
							},
						},
					},
				},
				msg: `invalid option: "foobar"`,
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{
						{
							Type:   "volume",
							Source: "missing-device-opt",
							Target: destPath,
							VolumeOptions: &mount.VolumeOptions{
								DriverConfig: &mount.Driver{
									Name:    "local",
									Options: map[string]string{"type": "tmpfs"},
								},
							},
						},
					},
				},
				msg: `missing required option: "device"`,
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{
						{
							Type:   "volume",
							Source: "missing-type-opt",
							Target: destPath,
							VolumeOptions: &mount.VolumeOptions{
								DriverConfig: &mount.Driver{
									Name:    "local",
									Options: map[string]string{"device": "tmpfs"},
								},
							},
						},
					},
				},
				msg: `missing required option: "type"`,
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{
						{
							Type:   "volume",
							Source: "hello4",
							Target: destPath,
							VolumeOptions: &mount.VolumeOptions{
								DriverConfig: &mount.Driver{
									Name:    "local",
									Options: map[string]string{"o": "size=1", "type": "tmpfs", "device": "tmpfs"},
								},
							},
						},
					},
				},
				msg: "",
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{{
						Type:   "tmpfs",
						Target: destPath,
					}},
				},
				msg: "",
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{{
						Type:   "tmpfs",
						Target: destPath,
						TmpfsOptions: &mount.TmpfsOptions{
							SizeBytes: 4096 * 1024,
							Mode:      0o700,
						},
					}},
				},
				msg: "",
			},
			{
				config: container.Config{
					Image: "busybox",
				},
				hostConfig: container.HostConfig{
					Mounts: []mount.Mount{{
						Type:   "tmpfs",
						Source: "/shouldnotbespecified",
						Target: destPath,
					}},
				},
				msg: "Source must not be specified",
			},
		}...)
	}
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	// TODO add checks for statuscode returned by API
	for i, x := range cases {
		x := x
		c.Run(fmt.Sprintf("case %d", i), func(c *testing.T) {
			_, err = apiClient.ContainerCreate(testutil.GetContext(c), &x.config, &x.hostConfig, &network.NetworkingConfig{}, nil, "")
			if len(x.msg) > 0 {
				assert.ErrorContains(c, err, x.msg, "%v", cases[i].config)
			} else {
				assert.NilError(c, err)
			}
		})
	}
}

func (s *DockerAPISuite) TestContainerAPICreateMountsBindRead(c *testing.T) {
	testRequires(c, NotUserNamespace, testEnv.IsLocalDaemon)
	// also with data in the host side
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	destPath := prefix + slash + "foo"
	tmpDir, err := os.MkdirTemp("", "test-mounts-api-bind")
	assert.NilError(c, err)
	defer os.RemoveAll(tmpDir)
	err = os.WriteFile(filepath.Join(tmpDir, "bar"), []byte("hello"), 0o666)
	assert.NilError(c, err)
	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", "cat /foo/bar"},
	}
	hostConfig := container.HostConfig{
		Mounts: []mount.Mount{
			{Type: "bind", Source: tmpDir, Target: destPath},
		},
	}
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "test")
	assert.NilError(c, err)

	out := cli.DockerCmd(c, "start", "-a", "test").Combined()
	assert.Equal(c, out, "hello")
}

// Test Mounts comes out as expected for the MountPoint
func (s *DockerAPISuite) TestContainersAPICreateMountsCreate(c *testing.T) {
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	destPath := prefix + slash + "foo"

	var testImg string
	if testEnv.DaemonInfo.OSType != "windows" {
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
		spec     mount.Mount
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
			spec:     mount.Mount{Type: "volume", Target: destPath},
			expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mount.Mount{Type: "volume", Target: destPath + slash},
			expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mount.Mount{Type: "volume", Target: destPath, Source: "test1"},
			expected: types.MountPoint{Type: "volume", Name: "test1", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mount.Mount{Type: "volume", Target: destPath, ReadOnly: true, Source: "test2"},
			expected: types.MountPoint{Type: "volume", Name: "test2", RW: false, Destination: destPath, Mode: selinuxSharedLabel},
		},
		{
			spec:     mount.Mount{Type: "volume", Target: destPath, Source: "test3", VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{Name: volume.DefaultDriverName}}},
			expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", Name: "test3", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
		},
	}

	if testEnv.IsLocalDaemon() {
		// setup temp dir for testing binds
		tmpDir1, err := os.MkdirTemp("", "test-mounts-api-1")
		assert.NilError(c, err)
		defer os.RemoveAll(tmpDir1)
		cases = append(cases, []testCase{
			{
				spec: mount.Mount{
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
				spec:     mount.Mount{Type: "bind", Source: tmpDir1, Target: destPath, ReadOnly: true},
				expected: types.MountPoint{Type: "bind", RW: false, Destination: destPath, Source: tmpDir1},
			},
		}...)

		// for modes only supported on Linux
		if DaemonIsLinux() {
			tmpDir3, err := os.MkdirTemp("", "test-mounts-api-3")
			assert.NilError(c, err)
			defer os.RemoveAll(tmpDir3)

			if assert.Check(c, mountWrapper(c, tmpDir3, tmpDir3, "none", "bind,shared")) {
				cases = append(cases, []testCase{
					{
						spec:     mount.Mount{Type: "bind", Source: tmpDir3, Target: destPath},
						expected: types.MountPoint{Type: "bind", RW: true, Destination: destPath, Source: tmpDir3},
					},
					{
						spec:     mount.Mount{Type: "bind", Source: tmpDir3, Target: destPath, ReadOnly: true},
						expected: types.MountPoint{Type: "bind", RW: false, Destination: destPath, Source: tmpDir3},
					},
					{
						spec:     mount.Mount{Type: "bind", Source: tmpDir3, Target: destPath, ReadOnly: true, BindOptions: &mount.BindOptions{Propagation: "shared"}},
						expected: types.MountPoint{Type: "bind", RW: false, Destination: destPath, Source: tmpDir3, Propagation: "shared"},
					},
				}...)
			}
		}
	}

	if testEnv.DaemonInfo.OSType != "windows" { // Windows does not support volume populate
		cases = append(cases, []testCase{
			{
				spec:     mount.Mount{Type: "volume", Target: destPath, VolumeOptions: &mount.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
			},
			{
				spec:     mount.Mount{Type: "volume", Target: destPath + slash, VolumeOptions: &mount.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Driver: volume.DefaultDriverName, Type: "volume", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
			},
			{
				spec:     mount.Mount{Type: "volume", Target: destPath, Source: "test4", VolumeOptions: &mount.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Type: "volume", Name: "test4", RW: true, Destination: destPath, Mode: selinuxSharedLabel},
			},
			{
				spec:     mount.Mount{Type: "volume", Target: destPath, Source: "test5", ReadOnly: true, VolumeOptions: &mount.VolumeOptions{NoCopy: true}},
				expected: types.MountPoint{Type: "volume", Name: "test5", RW: false, Destination: destPath, Mode: selinuxSharedLabel},
			},
		}...)
	}

	ctx := testutil.GetContext(c)
	apiclient := testEnv.APIClient()
	for i, x := range cases {
		x := x
		c.Run(fmt.Sprintf("%d config: %v", i, x.spec), func(c *testing.T) {
			ctr, err := apiclient.ContainerCreate(
				ctx,
				&container.Config{Image: testImg},
				&container.HostConfig{Mounts: []mount.Mount{x.spec}},
				&network.NetworkingConfig{},
				nil,
				"")
			assert.NilError(c, err)

			containerInspect, err := apiclient.ContainerInspect(ctx, ctr.ID)
			assert.NilError(c, err)
			mps := containerInspect.Mounts
			assert.Assert(c, is.Len(mps, 1))
			mountPoint := mps[0]

			if x.expected.Source != "" {
				assert.Check(c, is.Equal(x.expected.Source, mountPoint.Source))
			}
			if x.expected.Name != "" {
				assert.Check(c, is.Equal(x.expected.Name, mountPoint.Name))
			}
			if x.expected.Driver != "" {
				assert.Check(c, is.Equal(x.expected.Driver, mountPoint.Driver))
			}
			if x.expected.Propagation != "" {
				assert.Check(c, is.Equal(x.expected.Propagation, mountPoint.Propagation))
			}
			assert.Check(c, is.Equal(x.expected.RW, mountPoint.RW))
			assert.Check(c, is.Equal(x.expected.Type, mountPoint.Type))
			assert.Check(c, is.Equal(x.expected.Mode, mountPoint.Mode))
			assert.Check(c, is.Equal(x.expected.Destination, mountPoint.Destination))

			err = apiclient.ContainerStart(ctx, ctr.ID, container.StartOptions{})
			assert.NilError(c, err)
			poll.WaitOn(c, containerExit(ctx, apiclient, ctr.ID), poll.WithDelay(time.Second))

			err = apiclient.ContainerRemove(ctx, ctr.ID, container.RemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			})
			assert.NilError(c, err)

			switch {
			// Named volumes still exist after the container is removed
			case x.spec.Type == "volume" && len(x.spec.Source) > 0:
				_, err := apiclient.VolumeInspect(ctx, mountPoint.Name)
				assert.NilError(c, err)

			// Bind mounts are never removed with the container
			case x.spec.Type == "bind":

			// anonymous volumes are removed
			default:
				_, err := apiclient.VolumeInspect(ctx, mountPoint.Name)
				assert.Check(c, is.ErrorType(err, errdefs.IsNotFound))
			}
		})
	}
}

func containerExit(ctx context.Context, apiclient client.APIClient, name string) func(poll.LogT) poll.Result {
	return func(logT poll.LogT) poll.Result {
		ctr, err := apiclient.ContainerInspect(ctx, name)
		if err != nil {
			return poll.Error(err)
		}
		switch ctr.State.Status {
		case "created", "running":
			return poll.Continue("container %s is %s, waiting for exit", name, ctr.State.Status)
		}
		return poll.Success()
	}
}

func (s *DockerAPISuite) TestContainersAPICreateMountsTmpfs(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	type testCase struct {
		cfg             mount.Mount
		expectedOptions []string
	}
	target := "/foo"
	cases := []testCase{
		{
			cfg: mount.Mount{
				Type:   "tmpfs",
				Target: target,
			},
			expectedOptions: []string{"rw", "nosuid", "nodev", "noexec", "relatime"},
		},
		{
			cfg: mount.Mount{
				Type:   "tmpfs",
				Target: target,
				TmpfsOptions: &mount.TmpfsOptions{
					SizeBytes: 4096 * 1024, Mode: 0o700,
				},
			},
			expectedOptions: []string{"rw", "nosuid", "nodev", "noexec", "relatime", "size=4096k", "mode=700"},
		},
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	config := container.Config{
		Image: "busybox",
		Cmd:   []string{"/bin/sh", "-c", fmt.Sprintf("mount | grep 'tmpfs on %s'", target)},
	}
	for i, x := range cases {
		cName := fmt.Sprintf("test-tmpfs-%d", i)
		hostConfig := container.HostConfig{
			Mounts: []mount.Mount{x.cfg},
		}

		_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, cName)
		assert.NilError(c, err)
		out := cli.DockerCmd(c, "start", "-a", cName).Combined()
		for _, option := range x.expectedOptions {
			assert.Assert(c, strings.Contains(out, option))
		}
	}
}

// Regression test for #33334
// Makes sure that when a container which has a custom stop signal + restart=always
// gets killed (with SIGKILL) by the kill API, that the restart policy is cancelled.
func (s *DockerAPISuite) TestContainerKillCustomStopSignal(c *testing.T) {
	id := strings.TrimSpace(runSleepingContainer(c, "--stop-signal=SIGTERM", "--restart=always"))
	res, _, err := request.Post(testutil.GetContext(c), "/containers/"+id+"/kill")
	assert.NilError(c, err)
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent, string(b))
	err = waitInspect(id, "{{.State.Running}} {{.State.Restarting}}", "false false", 30*time.Second)
	assert.NilError(c, err)
}
