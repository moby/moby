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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestContainerApiGetAll(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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
func (s *DockerSuite) TestContainerPsOmitFields(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "pstest"
	port := 80
	dockerCmd(c, "run", "-d", "--name", name, "--expose", strconv.Itoa(port), "busybox", "top")

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

func (s *DockerSuite) TestContainerApiStartVolumeBinds(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	bindPath := randomTmpDirPath("test", daemonPlatform)
	config = map[string]interface{}{
		"Binds": []string{bindPath + ":/tmp"},
	}
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	pth, err := inspectMountSourceField(name, "/tmp")
	c.Assert(err, checker.IsNil)
	c.Assert(pth, checker.Equals, bindPath, check.Commentf("expected volume host path to be %s, got %s", bindPath, pth))
}

// Test for GH#10618
func (s *DockerSuite) TestContainerApiStartDupVolumeBinds(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testdups"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	bindPath1 := randomTmpDirPath("test1", daemonPlatform)
	bindPath2 := randomTmpDirPath("test2", daemonPlatform)

	config = map[string]interface{}{
		"Binds": []string{bindPath1 + ":/tmp", bindPath2 + ":/tmp"},
	}
	status, body, err := sockRequest("POST", "/containers/"+name+"/start", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Contains, "Duplicate bind", check.Commentf("Expected failure due to duplicate bind mounts to same path, instead got: %q with error: %v", string(body), err))
}

func (s *DockerSuite) TestContainerApiStartVolumesFrom(c *check.C) {
	testRequires(c, DaemonIsLinux)
	volName := "voltst"
	volPath := "/tmp"

	dockerCmd(c, "run", "-d", "--name", volName, "-v", volPath, "busybox")

	name := "TestContainerApiStartVolumesFrom"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{volPath: {}},
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	config = map[string]interface{}{
		"VolumesFrom": []string{volName},
	}
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	pth, err := inspectMountSourceField(name, volPath)
	c.Assert(err, checker.IsNil)
	pth2, err := inspectMountSourceField(volName, volPath)
	c.Assert(err, checker.IsNil)
	c.Assert(pth, checker.Equals, pth2, check.Commentf("expected volume host path to be %s, got %s", pth, pth2))
}

func (s *DockerSuite) TestGetContainerStats(c *check.C) {
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
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)

	buf := &integration.ChannelBuffer{make(chan []byte, 1)}
	defer buf.Close()
	chErr := make(chan error)
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
		c.Assert(<-chErr, checker.IsNil)
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
	dockerCmd(c, "rm", "-f", id)

	_, err = buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.Not(checker.IsNil))
}

// regression test for gh13421
// previous test was just checking one stat entry so it didn't fail (stats with
// stream false always return one stat)
func (s *DockerSuite) TestGetContainerStatsStream(c *check.C) {
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
	testRequires(c, DaemonIsLinux)
	// TODO: this test does nothing because we are c.Assert'ing in goroutine
	var (
		name = "statscontainer"
	)
	dockerCmd(c, "create", "--name", name, "busybox", "top")

	go func() {
		// We'll never get return for GET stats from sockRequest as of now,
		// just send request and see if panic or error would happen on daemon side.
		status, _, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		c.Assert(err, checker.IsNil)
		c.Assert(status, checker.Equals, http.StatusOK)
	}()

	// allow some time to send request and let daemon deal with it
	time.Sleep(1 * time.Second)
}

// #9981 - Allow a docker created volume (ie, one in /var/lib/docker/volumes) to be used to overwrite (via passing in Binds on api start) an existing volume
func (s *DockerSuite) TestPostContainerBindNormalVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "create", "-v", "/foo", "--name=one", "busybox")

	fooDir, err := inspectMountSourceField("one", "/foo")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "create", "-v", "/foo", "--name=two", "busybox")

	bindSpec := map[string][]string{"Binds": {fooDir + ":/foo"}}
	status, _, err := sockRequest("POST", "/containers/two/start", bindSpec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	fooDir2, err := inspectMountSourceField("two", "/foo")
	c.Assert(err, checker.IsNil)
	c.Assert(fooDir2, checker.Equals, fooDir, check.Commentf("expected volume path to be %s, got: %s", fooDir, fooDir2))
}

func (s *DockerSuite) TestContainerApiPause(c *check.C) {
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
	testRequires(c, DaemonIsLinux)
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

	cmd, err := inspectField(img.ID, "Config.Cmd")
	c.Assert(err, checker.IsNil)
	c.Assert(cmd, checker.Equals, "{[/bin/sh -c touch /test]}", check.Commentf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	dockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerSuite) TestContainerApiCommitWithLabelInConfig(c *check.C) {
	testRequires(c, DaemonIsLinux)
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

	label1, err := inspectFieldMap(img.ID, "Config.Labels", "key1")
	c.Assert(err, checker.IsNil)
	c.Assert(label1, checker.Equals, "value1")

	label2, err := inspectFieldMap(img.ID, "Config.Labels", "key2")
	c.Assert(err, checker.IsNil)
	c.Assert(label2, checker.Equals, "value2")

	cmd, err := inspectField(img.ID, "Config.Cmd")
	c.Assert(err, checker.IsNil)
	c.Assert(cmd, checker.Equals, "{[/bin/sh -c touch /test]}", check.Commentf("got wrong Cmd from commit: %q", cmd))

	// sanity check, make sure the image is what we think it is
	dockerCmd(c, "run", img.ID, "ls", "/test")
}

func (s *DockerSuite) TestContainerApiBadPort(c *check.C) {
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
	testRequires(c, DaemonIsLinux)
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

func (s *DockerSuite) TestContainerApiCreateWithHostName(c *check.C) {
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

func (s *DockerSuite) TestContainerApiCreateNetworkMode(c *check.C) {
	testRequires(c, DaemonIsLinux)
	UtilCreateNetworkMode(c, "host")
	UtilCreateNetworkMode(c, "bridge")
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
	c.Assert(containerJSON.HostConfig.NetworkMode, checker.Equals, runconfig.NetworkMode(networkMode), check.Commentf("Mismatched NetworkMode"))
}

func (s *DockerSuite) TestContainerApiCreateWithCpuSharesCpuset(c *check.C) {
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

	out, err := inspectField(containerJSON.ID, "HostConfig.CpuShares")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Equals, "512")

	outCpuset, errCpuset := inspectField(containerJSON.ID, "HostConfig.CpusetCpus")
	c.Assert(errCpuset, checker.IsNil, check.Commentf("Output: %s", outCpuset))
	c.Assert(outCpuset, checker.Equals, "0")
}

func (s *DockerSuite) TestContainerApiVerifyHeader(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
	config := `{
				  "Image": "busybox",
				  "HostConfig": {
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

	out, err := inspectField(container.ID, "HostConfig.CpusetCpus")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Equals, "")

	outMemory, errMemory := inspectField(container.ID, "HostConfig.Memory")
	c.Assert(outMemory, checker.Equals, "0")
	c.Assert(errMemory, checker.IsNil)
	outMemorySwap, errMemorySwap := inspectField(container.ID, "HostConfig.MemorySwap")
	c.Assert(outMemorySwap, checker.Equals, "0")
	c.Assert(errMemorySwap, checker.IsNil)
}

func (s *DockerSuite) TestCreateWithTooLowMemoryLimit(c *check.C) {
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

func (s *DockerSuite) TestStartWithTooLowMemoryLimit(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "busybox")

	containerID := strings.TrimSpace(out)

	config := `{
                "CpuShares": 100,
                "Memory":    524287
        }`

	res, body, err := sockRequestRaw("POST", "/containers/"+containerID+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	b, err2 := readBody(body)
	c.Assert(err2, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(b), checker.Contains, "Minimum memory limit allowed is 4MB")
}

func (s *DockerSuite) TestContainerApiRename(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--name", "TestContainerApiRename", "-d", "busybox", "sh")

	containerID := strings.TrimSpace(out)
	newName := "TestContainerApiRenameNew"
	statusCode, _, err := sockRequest("POST", "/containers/"+containerID+"/rename?name="+newName, nil)
	c.Assert(err, checker.IsNil)
	// 204 No Content is expected, not 200
	c.Assert(statusCode, checker.Equals, http.StatusNoContent)

	name, err := inspectField(containerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))
}

func (s *DockerSuite) TestContainerApiKill(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-api-kill"
	dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")

	status, _, err := sockRequest("POST", "/containers/"+name+"/kill", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	state, err := inspectField(name, "State.Running")
	c.Assert(err, checker.IsNil)
	c.Assert(state, checker.Equals, "false", check.Commentf("got wrong State from container %s: %q", name, state))
}

func (s *DockerSuite) TestContainerApiRestart(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-api-restart"
	dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")

	status, _, err := sockRequest("POST", "/containers/"+name+"/restart?t=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 5*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerApiRestartNotimeoutParam(c *check.C) {
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
	testRequires(c, DaemonIsLinux)
	name := "testing-start"
	config := map[string]interface{}{
		"Image":     "busybox",
		"Cmd":       []string{"/bin/sh", "-c", "/bin/top"},
		"OpenStdin": true,
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	conf := make(map[string]interface{})
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", conf)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	// second call to start should give 304
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", conf)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotModified)
}

func (s *DockerSuite) TestContainerApiStop(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-api-stop"
	dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")

	status, _, err := sockRequest("POST", "/containers/"+name+"/stop?t=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Running  }}", "false", 5*time.Second), checker.IsNil)

	// second call to start should give 304
	status, _, err = sockRequest("POST", "/containers/"+name+"/stop?t=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotModified)
}

func (s *DockerSuite) TestContainerApiWait(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-api-wait"
	dockerCmd(c, "run", "--name", name, "busybox", "sleep", "5")

	status, body, err := sockRequest("POST", "/containers/"+name+"/wait", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(waitInspect(name, "{{ .State.Running  }}", "false", 5*time.Second), checker.IsNil)

	var waitres types.ContainerWaitResponse
	c.Assert(json.Unmarshal(body, &waitres), checker.IsNil)
	c.Assert(waitres.StatusCode, checker.Equals, 0)
}

func (s *DockerSuite) TestContainerApiCopy(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "/test.txt",
	}

	status, body, err := sockRequest("POST", "/containers/"+name+"/copy", postData)
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

func (s *DockerSuite) TestContainerApiCopyResourcePathEmpty(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy-resource-empty"
	dockerCmd(c, "run", "--name", name, "busybox", "touch", "/test.txt")

	postData := types.CopyConfig{
		Resource: "",
	}

	status, body, err := sockRequest("POST", "/containers/"+name+"/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Matches, "Path cannot be empty\n")
}

func (s *DockerSuite) TestContainerApiCopyResourcePathNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-container-api-copy-resource-not-found"
	dockerCmd(c, "run", "--name", name, "busybox")

	postData := types.CopyConfig{
		Resource: "/notexist",
	}

	status, body, err := sockRequest("POST", "/containers/"+name+"/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Matches, "Could not find the file /notexist in container "+name+"\n")
}

func (s *DockerSuite) TestContainerApiCopyContainerNotFound(c *check.C) {
	postData := types.CopyConfig{
		Resource: "/something",
	}

	status, _, err := sockRequest("POST", "/containers/notexists/copy", postData)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
}

func (s *DockerSuite) TestContainerApiDelete(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")

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
	c.Assert(string(body), checker.Matches, "no such id: doesnotexist\n")
}

func (s *DockerSuite) TestContainerApiDeleteForce(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id+"?force=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
}

func (s *DockerSuite) TestContainerApiDeleteRemoveLinks(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--name", "tlink1", "busybox", "top")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	out, _ = dockerCmd(c, "run", "--link", "tlink1:tlink1", "--name", "tlink2", "-d", "busybox", "top")

	id2 := strings.TrimSpace(out)
	c.Assert(waitRun(id2), checker.IsNil)

	links, err := inspectFieldJSON(id2, "HostConfig.Links")
	c.Assert(err, checker.IsNil)
	c.Assert(links, checker.Equals, "[\"/tlink1:/tlink2/tlink1\"]", check.Commentf("expected to have links between containers"))

	status, _, err := sockRequest("DELETE", "/containers/tlink2/tlink1?link=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	linksPostRm, err := inspectFieldJSON(id2, "HostConfig.Links")
	c.Assert(err, checker.IsNil)
	c.Assert(linksPostRm, checker.Equals, "null", check.Commentf("call to api deleteContainer links should have removed the specified links"))
}

func (s *DockerSuite) TestContainerApiDeleteConflict(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict)
}

func (s *DockerSuite) TestContainerApiDeleteRemoveVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon)

	out, _ := dockerCmd(c, "run", "-d", "-v", "/testvolume", "busybox", "top")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	source, err := inspectMountSourceField(id, "/testvolume")
	_, err = os.Stat(source)
	c.Assert(err, checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id+"?v=1&force=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	_, err = os.Stat(source)
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("expected to get ErrNotExist error, got %v", err))
}

// Regression test for https://github.com/docker/docker/issues/6231
func (s *DockerSuite) TestContainersApiChunkedEncoding(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "-v", "/foo", "busybox", "true")
	id := strings.TrimSpace(out)

	conn, err := sockConn(time.Duration(10 * time.Second))
	c.Assert(err, checker.IsNil)
	client := httputil.NewClientConn(conn, nil)
	defer client.Close()

	bindCfg := strings.NewReader(`{"Binds": ["/tmp:/foo"]}`)
	req, err := http.NewRequest("POST", "/containers/"+id+"/start", bindCfg)
	c.Assert(err, checker.IsNil)
	req.Header.Set("Content-Type", "application/json")
	// This is a cheat to make the http request do chunked encoding
	// Otherwise (just setting the Content-Encoding to chunked) net/http will overwrite
	// https://golang.org/src/pkg/net/http/request.go?s=11980:12172
	req.ContentLength = -1

	resp, err := client.Do(req)
	c.Assert(err, checker.IsNil, check.Commentf("error starting container with chunked encoding"))
	resp.Body.Close()
	c.Assert(resp.StatusCode, checker.Equals, 204)

	out, err = inspectFieldJSON(id, "HostConfig.Binds")
	c.Assert(err, checker.IsNil)

	var binds []string
	c.Assert(json.NewDecoder(strings.NewReader(out)).Decode(&binds), checker.IsNil)
	c.Assert(binds, checker.HasLen, 1, check.Commentf("Got unexpected binds: %v", binds))

	expected := "/tmp:/foo"
	c.Assert(binds[0], checker.Equals, expected, check.Commentf("got incorrect bind spec"))
}

func (s *DockerSuite) TestPostContainerStop(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")

	containerID := strings.TrimSpace(out)
	c.Assert(waitRun(containerID), checker.IsNil)

	statusCode, _, err := sockRequest("POST", "/containers/"+containerID+"/stop", nil)
	c.Assert(err, checker.IsNil)
	// 204 No Content is expected, not 200
	c.Assert(statusCode, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(containerID, "{{ .State.Running  }}", "false", 5*time.Second), checker.IsNil)
}

// #14170
func (s *DockerSuite) TestPostContainersCreateWithStringOrSliceEntrypoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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

// #14640
func (s *DockerSuite) TestPostContainersStartWithoutLinksInHostConfig(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	dockerCmd(c, "create", "--name", name, "busybox", "top")

	hc, err := inspectFieldJSON(name, "HostConfig")
	c.Assert(err, checker.IsNil)
	config := `{"HostConfig":` + hc + `}`

	res, b, err := sockRequestRaw("POST", "/containers/"+name+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()
}

// #14640
func (s *DockerSuite) TestPostContainersStartWithLinksInHostConfig(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	dockerCmd(c, "run", "--name", "foo", "-d", "busybox", "top")
	dockerCmd(c, "create", "--name", name, "--link", "foo:bar", "busybox", "top")

	hc, err := inspectFieldJSON(name, "HostConfig")
	c.Assert(err, checker.IsNil)
	config := `{"HostConfig":` + hc + `}`

	res, b, err := sockRequestRaw("POST", "/containers/"+name+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()
}

// #14640
func (s *DockerSuite) TestPostContainersStartWithLinksInHostConfigIdLinked(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	out, _ := dockerCmd(c, "run", "--name", "link0", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	dockerCmd(c, "create", "--name", name, "--link", id, "busybox", "top")

	hc, err := inspectFieldJSON(name, "HostConfig")
	c.Assert(err, checker.IsNil)
	config := `{"HostConfig":` + hc + `}`

	res, b, err := sockRequestRaw("POST", "/containers/"+name+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()
}

// #14915
func (s *DockerSuite) TestContainersApiCreateNoHostConfig118(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	// Requires local volume mount bind.
	// --read-only + userns has remount issues
	testRequires(c, SameHostDaemon, NotUserNamespace)

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

func (s *DockerSuite) TestContainersApiGetContainersJSONEmpty(c *check.C) {
	testRequires(c, DaemonIsLinux)

	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(string(body), checker.Equals, "[]\n")
}

func (s *DockerSuite) TestPostContainersCreateWithWrongCpusetValues(c *check.C) {
	testRequires(c, DaemonIsLinux)

	c1 := struct {
		Image      string
		CpusetCpus string
	}{"busybox", "1-42,,"}
	name := "wrong-cpuset-cpus"
	status, body, err := sockRequest("POST", "/containers/create?name="+name, c1)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	expected := "Invalid value 1-42,, for cpuset cpus.\n"
	c.Assert(string(body), checker.Equals, expected)

	c2 := struct {
		Image      string
		CpusetMems string
	}{"busybox", "42-3,1--"}
	name = "wrong-cpuset-mems"
	status, body, err = sockRequest("POST", "/containers/create?name="+name, c2)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)
	expected = "Invalid value 42-3,1-- for cpuset mems.\n"
	c.Assert(string(body), checker.Equals, expected)
}

func (s *DockerSuite) TestStartWithNilDNS(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "busybox")
	containerID := strings.TrimSpace(out)

	config := `{"HostConfig": {"Dns": null}}`

	res, b, err := sockRequestRaw("POST", "/containers/"+containerID+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()

	dns, err := inspectFieldJSON(containerID, "HostConfig.Dns")
	c.Assert(err, checker.IsNil)
	c.Assert(dns, checker.Equals, "[]")
}

func (s *DockerSuite) TestPostContainersCreateShmSizeNegative(c *check.C) {
	config := map[string]interface{}{
		"Image":      "busybox",
		"HostConfig": map[string]interface{}{"ShmSize": -1},
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Contains, "SHM size must be greater then 0")
}

func (s *DockerSuite) TestPostContainersCreateShmSizeZero(c *check.C) {
	config := map[string]interface{}{
		"Image":      "busybox",
		"HostConfig": map[string]interface{}{"ShmSize": 0},
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(string(body), checker.Contains, "SHM size must be greater then 0")
}

func (s *DockerSuite) TestPostContainersCreateShmSizeHostConfigOmitted(c *check.C) {
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

	c.Assert(containerJSON.HostConfig.ShmSize, check.IsNil)

	out, _ := dockerCmd(c, "start", "-i", containerJSON.ID)
	shmRegexp := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegexp.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
}

func (s *DockerSuite) TestPostContainersCreateShmSizeOmitted(c *check.C) {
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

	c.Assert(*containerJSON.HostConfig.ShmSize, check.Equals, int64(67108864))

	out, _ := dockerCmd(c, "start", "-i", containerJSON.ID)
	shmRegexp := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegexp.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
}

func (s *DockerSuite) TestPostContainersCreateWithShmSize(c *check.C) {
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

	c.Assert(*containerJSON.HostConfig.ShmSize, check.Equals, int64(1073741824))

	out, _ := dockerCmd(c, "start", "-i", containerJSON.ID)
	shmRegex := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=1048576k`)
	if !shmRegex.MatchString(out) {
		c.Fatalf("Expected shm of 1GB in mount command, got %v", out)
	}
}

func (s *DockerSuite) TestPostContainersCreateMemorySwappinessHostConfigOmitted(c *check.C) {
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

	c.Assert(containerJSON.HostConfig.MemorySwappiness, check.IsNil)
}
