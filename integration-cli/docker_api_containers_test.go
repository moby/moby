package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestContainerApiGetAll(c *check.C) {
	startCount, err := getContainerCount()
	c.Assert(err, checker.IsNil, check.Commentf("Cannot query container count"))

	name := "getall"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var inspectJSON []struct {
		Names []string
	}
	err = json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil, check.Commentf("unable to unmarshal response body"))

	c.Assert(inspectJSON, checker.HasLen, startCount+1)

	actual := inspectJSON[0].Names[0]
	c.Assert(actual, checker.Equals, "/"+name)
}

// regression test for empty json field being omitted #13691
func (s *DockerSuite) TestContainerApiGetJSONNoFieldsOmitted(c *check.C) {
	dockerCmd(c, "run", "busybox", "true")

	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	// empty Labels field triggered this bug, make sense to check for everything
	// cause even Ports for instance can trigger this bug
	// better safe than sorry..
	fields := []string{
		"Id",
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
		if !strings.Contains(string(body), f) {
			c.Fatalf("Field %s is missing and it shouldn't", f)
		}
	}
}

type containerPs struct {
	Names []string
	Ports []map[string]interface{}
}

// regression test for non-empty fields from #13901
func (s *DockerSuite) TestContainerApiPsOmitFields(c *check.C) {
	// Problematic for Windows porting due to networking not yet being passed back
	testRequires(c, DaemonIsLinux)
	name := "pstest"
	port := 80
	runSleepingContainer(c, "--name", name, "--expose", strconv.Itoa(port))

	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var resp []containerPs
	err = json.Unmarshal(body, &resp)
	c.Assert(err, checker.IsNil)

	var foundContainer *containerPs
	for _, container := range resp {
		for _, testName := range container.Names {
			if "/"+name == testName {
				foundContainer = &container
				break
			}
		}
	}

	c.Assert(foundContainer.Ports, checker.HasLen, 1)
	c.Assert(foundContainer.Ports[0]["PrivatePort"], checker.Equals, float64(port))
	_, ok := foundContainer.Ports[0]["PublicPort"]
	c.Assert(ok, checker.Not(checker.Equals), true)
	_, ok = foundContainer.Ports[0]["IP"]
	c.Assert(ok, checker.Not(checker.Equals), true)
}

func (s *DockerSuite) TestContainerApiGetExport(c *check.C) {
	// TODO: Investigate why this fails on Windows to Windows CI
	testRequires(c, DaemonIsLinux)
	name := "exportcontainer"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test")

	status, body, err := sockRequest("GET", "/containers/"+name+"/export", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	found := false
	for tarReader := tar.NewReader(bytes.NewReader(body)); ; {
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

func (s *DockerSuite) TestContainerApiGetChanges(c *check.C) {
	// Not supported on Windows as Windows does not support docker diff (/containers/name/changes)
	testRequires(c, DaemonIsLinux)
	name := "changescontainer"
	dockerCmd(c, "run", "--name", name, "busybox", "rm", "/etc/passwd")

	status, body, err := sockRequest("GET", "/containers/"+name+"/changes", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	changes := []struct {
		Kind int
		Path string
	}{}
	c.Assert(json.Unmarshal(body, &changes), checker.IsNil, check.Commentf("unable to unmarshal response body"))

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
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	var (
		name = "statscontainer"
	)
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	type b struct {
		status int
		body   []byte
		err    error
	}
	bc := make(chan b, 1)
	go func() {
		status, body, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		bc <- b{status, body, err}
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
		c.Assert(sr.status, checker.Equals, http.StatusOK)

		dec := json.NewDecoder(bytes.NewBuffer(sr.body))
		var s *types.Stats
		// decode only one object from the stream
		c.Assert(dec.Decode(&s), checker.IsNil)
	}
}

func (s *DockerSuite) TestGetContainerStatsRmRunning(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)

	buf := &integration.ChannelBuffer{make(chan []byte, 1)}
	defer buf.Close()
	chErr := make(chan error, 1)
	go func() {
		_, body, err := sockRequestRaw("GET", "/containers/"+id+"/stats?stream=1", nil, "application/json")
		if err != nil {
			chErr <- err
		}
		defer body.Close()
		_, err = io.Copy(buf, body)
		chErr <- err
	}()
	defer func() {
		select {
		case err := <-chErr:
			c.Assert(err, checker.IsNil)
		default:
			return
		}
	}()

	b := make([]byte, 32)
	// make sure we've got some stats
	_, err := buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.IsNil)

	// Now remove without `-f` and make sure we are still pulling stats
	_, _, err = dockerCmdWithError("rm", id)
	c.Assert(err, checker.Not(checker.IsNil), check.Commentf("rm should have failed but didn't"))
	_, err = buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "kill", id)
}

// regression test for gh13421
// previous test was just checking one stat entry so it didn't fail (stats with
// stream false always return one stat)
func (s *DockerSuite) TestGetContainerStatsStream(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	name := "statscontainer"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	type b struct {
		status int
		body   []byte
		err    error
	}
	bc := make(chan b, 1)
	go func() {
		status, body, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		bc <- b{status, body, err}
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
		c.Assert(sr.status, checker.Equals, http.StatusOK)

		s := string(sr.body)
		// count occurrences of "read" of types.Stats
		if l := strings.Count(s, "read"); l < 2 {
			c.Fatalf("Expected more than one stat streamed, got %d", l)
		}
	}
}

func (s *DockerSuite) TestGetContainerStatsNoStream(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	name := "statscontainer"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	type b struct {
		status int
		body   []byte
		err    error
	}
	bc := make(chan b, 1)
	go func() {
		status, body, err := sockRequest("GET", "/containers/"+name+"/stats?stream=0", nil)
		bc <- b{status, body, err}
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
		c.Assert(sr.status, checker.Equals, http.StatusOK)

		s := string(sr.body)
		// count occurrences of "read" of types.Stats
		c.Assert(strings.Count(s, "read"), checker.Equals, 1, check.Commentf("Expected only one stat streamed, got %d", strings.Count(s, "read")))
	}
}

func (s *DockerSuite) TestGetStoppedContainerStats(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	name := "statscontainer"
	dockerCmd(c, "create", "--name", name, "busybox", "top")

	type stats struct {
		status int
		err    error
	}
	chResp := make(chan stats)

	// We expect an immediate response, but if it's not immediate, the test would hang, so put it in a goroutine
	// below we'll check this on a timeout.
	go func() {
		resp, body, err := sockRequestRaw("GET", "/containers/"+name+"/stats", nil, "")
		body.Close()
		chResp <- stats{resp.StatusCode, err}
	}()

	select {
	case r := <-chResp:
		c.Assert(r.err, checker.IsNil)
		c.Assert(r.status, checker.Equals, http.StatusOK)
	case <-time.After(10 * time.Second):
		c.Fatal("timeout waiting for stats response for stopped container")
	}
}

func (s *DockerSuite) TestContainerApiPause(c *check.C) {
	// Problematic on Windows as Windows does not support pause
	testRequires(c, DaemonIsLinux)
	defer unpauseAllContainers()
	out, _ := dockerCmd(c, "run", "-d", "busybox", "sleep", "30")
	ContainerID := strings.TrimSpace(out)

	status, _, err := sockRequest("POST", "/containers/"+ContainerID+"/pause", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	pausedContainers, err := getSliceOfPausedContainers()
	c.Assert(err, checker.IsNil, check.Commentf("error thrown while checking if containers were paused"))

	if len(pausedContainers) != 1 || stringid.TruncateID(ContainerID) != pausedContainers[0] {
		c.Fatalf("there should be one paused container and not %d", len(pausedContainers))
	}

	status, _, err = sockRequest("POST", "/containers/"+ContainerID+"/unpause", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	pausedContainers, err = getSliceOfPausedContainers()
	c.Assert(err, checker.IsNil, check.Commentf("error thrown while checking if containers were paused"))
	c.Assert(pausedContainers, checker.IsNil, check.Commentf("There should be no paused container."))
}

func (s *DockerSuite) TestContainerApiTop(c *check.C) {
	// Problematic on Windows as Windows does not support top
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "top")
	id := strings.TrimSpace(string(out))
	c.Assert(waitRun(id), checker.IsNil)

	type topResp struct {
		Titles    []string
		Processes [][]string
	}
	var top topResp
	status, b, err := sockRequest("GET", "/containers/"+id+"/top?ps_args=aux", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(json.Unmarshal(b, &top), checker.IsNil)
	c.Assert(top.Titles, checker.HasLen, 11, check.Commentf("expected 11 titles, found %d: %v", len(top.Titles), top.Titles))

	if top.Titles[0] != "USER" || top.Titles[10] != "COMMAND" {
		c.Fatalf("expected `USER` at `Titles[0]` and `COMMAND` at Titles[10]: %v", top.Titles)
	}
	c.Assert(top.Processes, checker.HasLen, 2, check.Commentf("expected 2 processes, found %d: %v", len(top.Processes), top.Processes))
	c.Assert(top.Processes[0][10], checker.Equals, "/bin/sh -c top")
	c.Assert(top.Processes[1][10], checker.Equals, "top")
}

func (s *DockerSuite) TestContainerApiCommit(c *check.C) {
	cName := "testapicommit"
	dockerCmd(c, "run", "--name="+cName, "busybox", "/bin/sh", "-c", "touch /test")

	name := "testcontainerapicommit"
	status, b, err := sockRequest("POST", "/commit?repo="+name+"&testtag=tag&container="+cName, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	type resp struct {
		ID string
	}
	var img resp
	c.Assert(json.Unmarshal(b, &img), checker.IsNil)

	cmd := inspectField(c, img.ID, "Config.Cmd")
	c.Assert(cmd, checker.Equals, "[/bin/sh -c touch /test]", check.Commentf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	dockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerSuite) TestContainerApiCommitWithLabelInConfig(c *check.C) {
	cName := "testapicommitwithconfig"
	dockerCmd(c, "run", "--name="+cName, "busybox", "/bin/sh", "-c", "touch /test")

	config := map[string]interface{}{
		"Labels": map[string]string{"key1": "value1", "key2": "value2"},
	}

	name := "testcontainerapicommitwithconfig"
	status, b, err := sockRequest("POST", "/commit?repo="+name+"&container="+cName, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	type resp struct {
		ID string
	}
	var img resp
	c.Assert(json.Unmarshal(b, &img), checker.IsNil)

	label1 := inspectFieldMap(c, img.ID, "Config.Labels", "key1")
	c.Assert(label1, checker.Equals, "value1")

	label2 := inspectFieldMap(c, img.ID, "Config.Labels", "key2")
	c.Assert(label2, checker.Equals, "value2")

	cmd := inspectField(c, img.ID, "Config.Cmd")
	c.Assert(cmd, checker.Equals, "[/bin/sh -c touch /test]", check.Commentf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	dockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerSuite) TestContainerApiBadPort(c *check.C) {
	// TODO Windows to Windows CI - Port this test
	testRequires(c, DaemonIsLinux)
	config := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", "echo test"},
		"PortBindings": map[string]interface{}{
			"8080/tcp": []map[string]interface{}{
				{
					"HostIP":   "",
					"HostPort": "aa80",
				},
			},
		},
	}

	jsonData := bytes.NewBuffer(nil)
	json.NewEncoder(jsonData).Encode(config)

	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	c.Assert(strings.TrimSpace(string(b)), checker.Equals, `Invalid port specification: "aa80"`, check.Commentf("Incorrect error msg: %s", string(b)))
}

func (s *DockerSuite) TestContainerApiCreate(c *check.C) {
	config := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", "touch /test && ls /test"},
	}

	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	type createResp struct {
		ID string
	}
	var container createResp
	c.Assert(json.Unmarshal(b, &container), checker.IsNil)

	out, _ := dockerCmd(c, "start", "-a", container.ID)
	c.Assert(strings.TrimSpace(out), checker.Equals, "/test")
}

func (s *DockerSuite) TestContainerApiCreateEmptyConfig(c *check.C) {
	config := map[string]interface{}{}

	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)

	expected := "Config cannot be empty in order to create a container\n"
	c.Assert(string(b), checker.Equals, expected)
}

func (s *DockerSuite) TestContainerApiCreateMultipleNetworksConfig(c *check.C) {
	// Container creation must fail if client specified configurations for more than one network
	config := map[string]interface{}{
		"Image": "busybox",
		"NetworkingConfig": networktypes.NetworkingConfig{
			EndpointsConfig: map[string]*networktypes.EndpointSettings{
				"net1": {},
				"net2": {},
				"net3": {},
			},
		},
	}

	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	// network name order in error message is not deterministic
	c.Assert(string(b), checker.Contains, "Container cannot be connected to network endpoints")
	c.Assert(string(b), checker.Contains, "net1")
	c.Assert(string(b), checker.Contains, "net2")
	c.Assert(string(b), checker.Contains, "net3")
}

func (s *DockerSuite) TestContainerApiCreateWithHostName(c *check.C) {
	// TODO Windows: Port this test once hostname is supported
	testRequires(c, DaemonIsLinux)
	hostName := "test-host"
	config := map[string]interface{}{
		"Image":    "busybox",
		"Hostname": hostName,
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), checker.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), checker.IsNil)
	c.Assert(containerJSON.Config.Hostname, checker.Equals, hostName, check.Commentf("Mismatched Hostname"))
}

func (s *DockerSuite) TestContainerApiCreateWithDomainName(c *check.C) {
	// TODO Windows: Port this test once domain name is supported
	testRequires(c, DaemonIsLinux)
	domainName := "test-domain"
	config := map[string]interface{}{
		"Image":      "busybox",
		"Domainname": domainName,
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), checker.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), checker.IsNil)
	c.Assert(containerJSON.Config.Domainname, checker.Equals, domainName, check.Commentf("Mismatched Domainname"))
}

func (s *DockerSuite) TestContainerApiCreateBridgeNetworkMode(c *check.C) {
	// Windows does not support bridge
	testRequires(c, DaemonIsLinux)
	UtilCreateNetworkMode(c, "bridge")
}

func (s *DockerSuite) TestContainerApiCreateOtherNetworkModes(c *check.C) {
	// Windows does not support these network modes
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	UtilCreateNetworkMode(c, "host")
	UtilCreateNetworkMode(c, "container:web1")
}

func UtilCreateNetworkMode(c *check.C, networkMode string) {
	config := map[string]interface{}{
		"Image":      "busybox",
		"HostConfig": map[string]interface{}{"NetworkMode": networkMode},
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), checker.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), checker.IsNil)
	c.Assert(containerJSON.HostConfig.NetworkMode, checker.Equals, containertypes.NetworkMode(networkMode), check.Commentf("Mismatched NetworkMode"))
}

func (s *DockerSuite) TestContainerApiCreateWithCpuSharesCpuset(c *check.C) {
	// TODO Windows to Windows CI. The CpuShares part could be ported.
	testRequires(c, DaemonIsLinux)
	config := map[string]interface{}{
		"Image":      "busybox",
		"CpuShares":  512,
		"CpusetCpus": "0",
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), checker.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON

	c.Assert(json.Unmarshal(body, &containerJSON), checker.IsNil)

	out := inspectField(c, containerJSON.ID, "HostConfig.CpuShares")
	c.Assert(out, checker.Equals, "512")

	outCpuset := inspectField(c, containerJSON.ID, "HostConfig.CpusetCpus")
	c.Assert(outCpuset, checker.Equals, "0")
}

func (s *DockerSuite) TestContainerApiVerifyHeader(c *check.C) {
	config := map[string]interface{}{
		"Image": "busybox",
	}

	create := func(ct string) (*http.Response, io.ReadCloser, error) {
		jsonData := bytes.NewBuffer(nil)
		c.Assert(json.NewEncoder(jsonData).Encode(config), checker.IsNil)
		return sockRequestRaw("POST", "/containers/create", jsonData, ct)
	}

	// Try with no content-type
	res, body, err := create("")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)
	body.Close()

	// Try with wrong content-type
	res, body, err = create("application/xml")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)
	body.Close()

	// now application/json
	res, body, err = create("application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)
	body.Close()
}

//Issue 14230. daemon should return 500 for invalid port syntax
func (s *DockerSuite) TestContainerApiInvalidPortSyntax(c *check.C) {
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

	res, body, err := sockRequestRaw("POST", "/containers/create", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)

	b, err := readBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b[:]), checker.Contains, "Invalid port")
}

// Issue 7941 - test to make sure a "null" in JSON is just ignored.
// W/o this fix a null in JSON would be parsed into a string var as "null"
func (s *DockerSuite) TestContainerApiPostCreateNull(c *check.C) {
	// TODO Windows to Windows CI. Bit of this with alternate fields checked
	// can probably be ported.
	testRequires(c, DaemonIsLinux)
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

	res, body, err := sockRequestRaw("POST", "/containers/create", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)

	b, err := readBody(body)
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

	res, body, err := sockRequestRaw("POST", "/containers/create", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	b, err2 := readBody(body)
	c.Assert(err2, checker.IsNil)

	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(b), checker.Contains, "Minimum memory limit allowed is 4MB")
}

func (s *DockerSuite) TestContainerApiRename(c *check.C) {
	// TODO Windows: Debug why this sometimes fails on TP5. For now, leave disabled
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--name", "TestContainerApiRename", "-d", "busybox", "sh")

	containerID := strings.TrimSpace(out)
	newName := "TestContainerApiRenameNew"
	statusCode, _, err := sockRequest("POST", "/containers/"+containerID+"/rename?name="+newName, nil)
	c.Assert(err, checker.IsNil)
	// 204 No Content is expected, not 200
	c.Assert(statusCode, checker.Equals, http.StatusNoContent)

	name := inspectField(c, containerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))
}

func (s *DockerSuite) TestContainerApiKill(c *check.C) {
	name := "test-api-kill"
	runSleepingContainer(c, "-i", "--name", name)

	status, _, err := sockRequest("POST", "/containers/"+name+"/kill", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	state := inspectField(c, name, "State.Running")
	c.Assert(state, checker.Equals, "false", check.Commentf("got wrong State from container %s: %q", name, state))
}

func (s *DockerSuite) TestContainerApiRestart(c *check.C) {
	// TODO Windows to Windows CI. This is flaky due to the timing
	testRequires(c, DaemonIsLinux)
	name := "test-api-restart"
	dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")

	status, _, err := sockRequest("POST", "/containers/"+name+"/restart?t=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 5*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerApiRestartNotimeoutParam(c *check.C) {
	// TODO Windows to Windows CI. This is flaky due to the timing
	testRequires(c, DaemonIsLinux)
	name := "test-api-restart-no-timeout-param"
	out, _ := dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("POST", "/containers/"+name+"/restart", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 5*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerApiStart(c *check.C) {
	name := "testing-start"
	config := map[string]interface{}{
		"Image":     "busybox",
		"Cmd":       append([]string{"/bin/sh", "-c"}, defaultSleepCommand...),
		"OpenStdin": true,
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	status, _, err = sockRequest("POST", "/containers/"+name+"/start", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	// second call to start should give 304
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", nil)
	c.Assert(err, checker.IsNil)

	// TODO(tibor): figure out why this doesn't work on windows
	if isLocalDaemon {
		c.Assert(status, checker.Equals, http.StatusNotModified)
	}
}

func (s *DockerSuite) TestContainerApiStop(c *check.C) {
	name := "test-api-stop"
	runSleepingContainer(c, "-i", "--name", name)

	status, _, err := sockRequest("POST", "/containers/"+name+"/stop?t=30", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Running  }}", "false", 60*time.Second), checker.IsNil)

	// second call to start should give 304
	status, _, err = sockRequest("POST", "/containers/"+name+"/stop?t=30", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotModified)
}

func (s *DockerSuite) TestContainerApiWait(c *check.C) {
	name := "test-api-wait"

	sleepCmd := "/bin/sleep"
	if daemonPlatform == "windows" {
		sleepCmd = "sleep"
	}
	dockerCmd(c, "run", "--name", name, "busybox", sleepCmd, "5")

	status, body, err := sockRequest("POST", "/containers/"+name+"/wait", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(waitInspect(name, "{{ .State.Running  }}", "false", 60*time.Second), checker.IsNil)

	var waitres types.ContainerWaitResponse
	c.Assert(json.Unmarshal(body, &waitres), checker.IsNil)
	c.Assert(waitres.StatusCode, checker.Equals, 0)
}

func (s *DockerSuite) TestContainerApiCopyNotExistsAnyMore(c *check.C) {
	// TODO Windows to Windows CI. This can be ported.
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "/test.txt",
	}

	status, _, err := sockRequest("POST", "/containers/"+name+"/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
}

func (s *DockerSuite) TestContainerApiCopyPre124(c *check.C) {
	// TODO Windows to Windows CI. This can be ported.
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "/test.txt",
	}

	status, body, err := sockRequest("POST", "/v1.23/containers/"+name+"/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	found := false
	for tarReader := tar.NewReader(bytes.NewReader(body)); ; {
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

func (s *DockerSuite) TestContainerApiCopyResourcePathEmptyPr124(c *check.C) {
	// TODO Windows to Windows CI. This can be ported.
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy-resource-empty"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "",
	}

	status, body, err := sockRequest("POST", "/v1.23/containers/"+name+"/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Matches, "Path cannot be empty\n")
}

func (s *DockerSuite) TestContainerApiCopyResourcePathNotFoundPre124(c *check.C) {
	// TODO Windows to Windows CI. This can be ported.
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy-resource-not-found"
	dockerCmd(c, "run", "--name", name, "busybox")

	postData := types.CopyConfig{
		Resource: "/notexist",
	}

	status, body, err := sockRequest("POST", "/v1.23/containers/"+name+"/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Matches, "Could not find the file /notexist in container "+name+"\n")
}

func (s *DockerSuite) TestContainerApiCopyContainerNotFoundPr124(c *check.C) {
	postData := types.CopyConfig{
		Resource: "/something",
	}

	status, _, err := sockRequest("POST", "/v1.23/containers/notexists/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
}

func (s *DockerSuite) TestContainerApiDelete(c *check.C) {
	out, _ := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	dockerCmd(c, "stop", id)

	status, _, err := sockRequest("DELETE", "/containers/"+id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
}

func (s *DockerSuite) TestContainerApiDeleteNotExist(c *check.C) {
	status, body, err := sockRequest("DELETE", "/containers/doesnotexist", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
	c.Assert(string(body), checker.Matches, "No such container: doesnotexist\n")
}

func (s *DockerSuite) TestContainerApiDeleteForce(c *check.C) {
	out, _ := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id+"?force=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
}

func (s *DockerSuite) TestContainerApiDeleteRemoveLinks(c *check.C) {
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

	status, b, err := sockRequest("DELETE", "/containers/tlink2/tlink1?link=1", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNoContent, check.Commentf(string(b)))

	linksPostRm := inspectFieldJSON(c, id2, "HostConfig.Links")
	c.Assert(linksPostRm, checker.Equals, "null", check.Commentf("call to api deleteContainer links should have removed the specified links"))
}

func (s *DockerSuite) TestContainerApiDeleteConflict(c *check.C) {
	out, _ := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict)
}

func (s *DockerSuite) TestContainerApiDeleteRemoveVolume(c *check.C) {
	testRequires(c, SameHostDaemon)

	vol := "/testvolume"
	if daemonPlatform == "windows" {
		vol = `c:\testvolume`
	}

	out, _ := runSleepingContainer(c, "-v", vol)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	source, err := inspectMountSourceField(id, vol)
	_, err = os.Stat(source)
	c.Assert(err, checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id+"?v=1&force=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	_, err = os.Stat(source)
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("expected to get ErrNotExist error, got %v", err))
}

// Regression test for https://github.com/docker/docker/issues/6231
func (s *DockerSuite) TestContainerApiChunkedEncoding(c *check.C) {
	// TODO Windows CI: This can be ported
	testRequires(c, DaemonIsLinux)

	conn, err := sockConn(time.Duration(10 * time.Second))
	c.Assert(err, checker.IsNil)
	client := httputil.NewClientConn(conn, nil)
	defer client.Close()

	config := map[string]interface{}{
		"Image":     "busybox",
		"Cmd":       append([]string{"/bin/sh", "-c"}, defaultSleepCommand...),
		"OpenStdin": true,
	}
	b, err := json.Marshal(config)
	c.Assert(err, checker.IsNil)

	req, err := http.NewRequest("POST", "/containers/create", bytes.NewBuffer(b))
	c.Assert(err, checker.IsNil)
	req.Header.Set("Content-Type", "application/json")
	// This is a cheat to make the http request do chunked encoding
	// Otherwise (just setting the Content-Encoding to chunked) net/http will overwrite
	// https://golang.org/src/pkg/net/http/request.go?s=11980:12172
	req.ContentLength = -1

	resp, err := client.Do(req)
	c.Assert(err, checker.IsNil, check.Commentf("error creating container with chunked encoding"))
	resp.Body.Close()
	c.Assert(resp.StatusCode, checker.Equals, http.StatusCreated)
}

func (s *DockerSuite) TestContainerApiPostContainerStop(c *check.C) {
	out, _ := runSleepingContainer(c)

	containerID := strings.TrimSpace(out)
	c.Assert(waitRun(containerID), checker.IsNil)

	statusCode, _, err := sockRequest("POST", "/containers/"+containerID+"/stop", nil)
	c.Assert(err, checker.IsNil)
	// 204 No Content is expected, not 200
	c.Assert(statusCode, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(containerID, "{{ .State.Running  }}", "false", 60*time.Second), checker.IsNil)
}

// #14170
func (s *DockerSuite) TestPostContainerApiCreateWithStringOrSliceEntrypoint(c *check.C) {
	config := struct {
		Image      string
		Entrypoint string
		Cmd        []string
	}{"busybox", "echo", []string{"hello", "world"}}
	_, _, err := sockRequest("POST", "/containers/create?name=echotest", config)
	c.Assert(err, checker.IsNil)
	out, _ := dockerCmd(c, "start", "-a", "echotest")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")

	config2 := struct {
		Image      string
		Entrypoint []string
		Cmd        []string
	}{"busybox", []string{"echo"}, []string{"hello", "world"}}
	_, _, err = sockRequest("POST", "/containers/create?name=echotest2", config2)
	c.Assert(err, checker.IsNil)
	out, _ = dockerCmd(c, "start", "-a", "echotest2")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")
}

// #14170
func (s *DockerSuite) TestPostContainersCreateWithStringOrSliceCmd(c *check.C) {
	config := struct {
		Image      string
		Entrypoint string
		Cmd        string
	}{"busybox", "echo", "hello world"}
	_, _, err := sockRequest("POST", "/containers/create?name=echotest", config)
	c.Assert(err, checker.IsNil)
	out, _ := dockerCmd(c, "start", "-a", "echotest")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")

	config2 := struct {
		Image string
		Cmd   []string
	}{"busybox", []string{"echo", "hello", "world"}}
	_, _, err = sockRequest("POST", "/containers/create?name=echotest2", config2)
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
	status, _, err := sockRequest("POST", "/containers/create?name=capaddtest0", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	config2 := struct {
		Image   string
		CapAdd  []string
		CapDrop []string
	}{"busybox", []string{"NET_ADMIN", "SYS_ADMIN"}, []string{"SETGID"}}
	status, _, err = sockRequest("POST", "/containers/create?name=capaddtest1", config2)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)
}

// #14915
func (s *DockerSuite) TestContainerApiCreateNoHostConfig118(c *check.C) {
	config := struct {
		Image string
	}{"busybox"}
	status, _, err := sockRequest("POST", "/v1.18/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)
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
	defer deleteContainer(cID)

	// Attempt to extract to a symlink in the volume which points to a
	// directory outside the volume. This should cause an error because the
	// rootfs is read-only.
	query := make(url.Values, 1)
	query.Set("path", "/vol2/symlinkToAbsDir")
	urlPath := fmt.Sprintf("/v1.20/containers/%s/archive?%s", cID, query.Encode())

	statusCode, body, err := sockRequest("PUT", urlPath, nil)
	c.Assert(err, checker.IsNil)

	if !isCpCannotCopyReadOnly(fmt.Errorf(string(body))) {
		c.Fatalf("expected ErrContainerRootfsReadonly error, but got %d: %s", statusCode, string(body))
	}
}

func (s *DockerSuite) TestContainerApiGetContainersJSONEmpty(c *check.C) {
	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(string(body), checker.Equals, "[]\n")
}

func (s *DockerSuite) TestPostContainersCreateWithWrongCpusetValues(c *check.C) {
	// Not supported on Windows
	testRequires(c, DaemonIsLinux)

	c1 := struct {
		Image      string
		CpusetCpus string
	}{"busybox", "1-42,,"}
	name := "wrong-cpuset-cpus"
	status, body, err := sockRequest("POST", "/containers/create?name="+name, c1)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	expected := "Invalid value 1-42,, for cpuset cpus\n"
	c.Assert(string(body), checker.Equals, expected)

	c2 := struct {
		Image      string
		CpusetMems string
	}{"busybox", "42-3,1--"}
	name = "wrong-cpuset-mems"
	status, body, err = sockRequest("POST", "/containers/create?name="+name, c2)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	expected = "Invalid value 42-3,1-- for cpuset mems\n"
	c.Assert(string(body), checker.Equals, expected)
}

func (s *DockerSuite) TestPostContainersCreateShmSizeNegative(c *check.C) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	config := map[string]interface{}{
		"Image":      "busybox",
		"HostConfig": map[string]interface{}{"ShmSize": -1},
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Contains, "SHM size must be greater than 0")
}

func (s *DockerSuite) TestPostContainersCreateShmSizeHostConfigOmitted(c *check.C) {
	// ShmSize is not supported on Windows
	testRequires(c, DaemonIsLinux)
	var defaultSHMSize int64 = 67108864
	config := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   "mount",
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), check.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), check.IsNil)

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
	config := map[string]interface{}{
		"Image":      "busybox",
		"HostConfig": map[string]interface{}{},
		"Cmd":        "mount",
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), check.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), check.IsNil)

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
	config := map[string]interface{}{
		"Image":      "busybox",
		"Cmd":        "mount",
		"HostConfig": map[string]interface{}{"ShmSize": 1073741824},
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), check.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), check.IsNil)

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
	config := map[string]interface{}{
		"Image": "busybox",
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), check.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), check.IsNil)

	c.Assert(*containerJSON.HostConfig.MemorySwappiness, check.Equals, int64(-1))
}

// check validation is done daemon side and not only in cli
func (s *DockerSuite) TestPostContainersCreateWithOomScoreAdjInvalidRange(c *check.C) {
	// OomScoreAdj is not supported on Windows
	testRequires(c, DaemonIsLinux)

	config := struct {
		Image       string
		OomScoreAdj int
	}{"busybox", 1001}
	name := "oomscoreadj-over"
	status, b, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	expected := "Invalid value 1001, range for oom score adj is [-1000, 1000]"
	if !strings.Contains(string(b), expected) {
		c.Fatalf("Expected output to contain %q, got %q", expected, string(b))
	}

	config = struct {
		Image       string
		OomScoreAdj int
	}{"busybox", -1001}
	name = "oomscoreadj-low"
	status, b, err = sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	expected = "Invalid value -1001, range for oom score adj is [-1000, 1000]"
	if !strings.Contains(string(b), expected) {
		c.Fatalf("Expected output to contain %q, got %q", expected, string(b))
	}
}

// test case for #22210 where an emtpy container name caused panic.
func (s *DockerSuite) TestContainerApiDeleteWithEmptyName(c *check.C) {
	status, out, err := sockRequest("DELETE", "/containers/", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	c.Assert(string(out), checker.Contains, "No container name or ID supplied")
}
