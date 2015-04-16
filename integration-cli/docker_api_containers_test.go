package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

func TestContainerApiGetAll(t *testing.T) {
	defer deleteAllContainers()

	startCount, err := getContainerCount()
	if err != nil {
		t.Fatalf("Cannot query container count: %v", err)
	}

	name := "getall"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	_, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	if err != nil {
		t.Fatalf("GET all containers sockRequest failed: %v", err)
	}

	var inspectJSON []struct {
		Names []string
	}
	if err = json.Unmarshal(body, &inspectJSON); err != nil {
		t.Fatalf("unable to unmarshal response body: %v", err)
	}

	if len(inspectJSON) != startCount+1 {
		t.Fatalf("Expected %d container(s), %d found (started with: %d)", startCount+1, len(inspectJSON), startCount)
	}

	if actual := inspectJSON[0].Names[0]; actual != "/"+name {
		t.Fatalf("Container Name mismatch. Expected: %q, received: %q\n", "/"+name, actual)
	}

	logDone("container REST API - check GET json/all=1")
}

func TestContainerApiGetExport(t *testing.T) {
	defer deleteAllContainers()

	name := "exportcontainer"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "touch", "/test")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	_, body, err := sockRequest("GET", "/containers/"+name+"/export", nil)
	if err != nil {
		t.Fatalf("GET containers/export sockRequest failed: %v", err)
	}

	found := false
	for tarReader := tar.NewReader(bytes.NewReader(body)); ; {
		h, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if h.Name == "test" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("The created test file has not been found in the exported image")
	}

	logDone("container REST API - check GET containers/export")
}

func TestContainerApiGetChanges(t *testing.T) {
	defer deleteAllContainers()

	name := "changescontainer"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "rm", "/etc/passwd")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	_, body, err := sockRequest("GET", "/containers/"+name+"/changes", nil)
	if err != nil {
		t.Fatalf("GET containers/changes sockRequest failed: %v", err)
	}

	changes := []struct {
		Kind int
		Path string
	}{}
	if err = json.Unmarshal(body, &changes); err != nil {
		t.Fatalf("unable to unmarshal response body: %v", err)
	}

	// Check the changelog for removal of /etc/passwd
	success := false
	for _, elem := range changes {
		if elem.Path == "/etc/passwd" && elem.Kind == 2 {
			success = true
		}
	}
	if !success {
		t.Fatalf("/etc/passwd has been removed but is not present in the diff")
	}

	logDone("container REST API - check GET containers/changes")
}

func TestContainerApiStartVolumeBinds(t *testing.T) {
	defer deleteAllContainers()
	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	if status, _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && status != http.StatusCreated {
		t.Fatal(err)
	}

	bindPath := randomUnixTmpDirPath("test")
	config = map[string]interface{}{
		"Binds": []string{bindPath + ":/tmp"},
	}
	if status, _, err := sockRequest("POST", "/containers/"+name+"/start", config); err != nil && status != http.StatusNoContent {
		t.Fatal(err)
	}

	pth, err := inspectFieldMap(name, "Volumes", "/tmp")
	if err != nil {
		t.Fatal(err)
	}

	if pth != bindPath {
		t.Fatalf("expected volume host path to be %s, got %s", bindPath, pth)
	}

	logDone("container REST API - check volume binds on start")
}

// Test for GH#10618
func TestContainerApiStartDupVolumeBinds(t *testing.T) {
	defer deleteAllContainers()
	name := "testdups"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	if status, _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && status != http.StatusCreated {
		t.Fatal(err)
	}

	bindPath1 := randomUnixTmpDirPath("test1")
	bindPath2 := randomUnixTmpDirPath("test2")

	config = map[string]interface{}{
		"Binds": []string{bindPath1 + ":/tmp", bindPath2 + ":/tmp"},
	}
	if _, body, err := sockRequest("POST", "/containers/"+name+"/start", config); err == nil {
		t.Fatal("expected container start to fail when duplicate volume binds to same container path")
	} else {
		if !strings.Contains(string(body), "Duplicate volume") {
			t.Fatalf("Expected failure due to duplicate bind mounts to same path, instead got: %q with error: %v", string(body), err)
		}
	}

	logDone("container REST API - check for duplicate volume binds error on start")
}
func TestContainerApiStartVolumesFrom(t *testing.T) {
	defer deleteAllContainers()
	volName := "voltst"
	volPath := "/tmp"

	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", volName, "-v", volPath, "busybox")); err != nil {
		t.Fatal(out, err)
	}

	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{volPath: {}},
	}

	if status, _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && status != http.StatusCreated {
		t.Fatal(err)
	}

	config = map[string]interface{}{
		"VolumesFrom": []string{volName},
	}
	if status, _, err := sockRequest("POST", "/containers/"+name+"/start", config); err != nil && status != http.StatusNoContent {
		t.Fatal(err)
	}

	pth, err := inspectFieldMap(name, "Volumes", volPath)
	if err != nil {
		t.Fatal(err)
	}
	pth2, err := inspectFieldMap(volName, "Volumes", volPath)
	if err != nil {
		t.Fatal(err)
	}

	if pth != pth2 {
		t.Fatalf("expected volume host path to be %s, got %s", pth, pth2)
	}

	logDone("container REST API - check VolumesFrom on start")
}

// Ensure that volumes-from has priority over binds/anything else
// This is pretty much the same as TestRunApplyVolumesFromBeforeVolumes, except with passing the VolumesFrom and the bind on start
func TestVolumesFromHasPriority(t *testing.T) {
	defer deleteAllContainers()
	volName := "voltst2"
	volPath := "/tmp"

	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", volName, "-v", volPath, "busybox")); err != nil {
		t.Fatal(out, err)
	}

	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{volPath: {}},
	}

	if status, _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && status != http.StatusCreated {
		t.Fatal(err)
	}

	bindPath := randomUnixTmpDirPath("test")
	config = map[string]interface{}{
		"VolumesFrom": []string{volName},
		"Binds":       []string{bindPath + ":/tmp"},
	}
	if status, _, err := sockRequest("POST", "/containers/"+name+"/start", config); err != nil && status != http.StatusNoContent {
		t.Fatal(err)
	}

	pth, err := inspectFieldMap(name, "Volumes", volPath)
	if err != nil {
		t.Fatal(err)
	}
	pth2, err := inspectFieldMap(volName, "Volumes", volPath)
	if err != nil {
		t.Fatal(err)
	}

	if pth != pth2 {
		t.Fatalf("expected volume host path to be %s, got %s", pth, pth2)
	}

	logDone("container REST API - check VolumesFrom has priority")
}

func TestGetContainerStats(t *testing.T) {
	defer deleteAllContainers()
	var (
		name   = "statscontainer"
		runCmd = exec.Command(dockerBinary, "run", "-d", "--name", name, "busybox", "top")
	)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}
	type b struct {
		body []byte
		err  error
	}
	bc := make(chan b, 1)
	go func() {
		_, body, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		bc <- b{body, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	if _, err := runCommand(exec.Command(dockerBinary, "rm", "-f", name)); err != nil {
		t.Fatal(err)
	}

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		if sr.err != nil {
			t.Fatal(sr.err)
		}

		dec := json.NewDecoder(bytes.NewBuffer(sr.body))
		var s *types.Stats
		// decode only one object from the stream
		if err := dec.Decode(&s); err != nil {
			t.Fatal(err)
		}
	}
	logDone("container REST API - check GET containers/stats")
}

func TestGetStoppedContainerStats(t *testing.T) {
	defer deleteAllContainers()
	var (
		name   = "statscontainer"
		runCmd = exec.Command(dockerBinary, "create", "--name", name, "busybox", "top")
	)
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	go func() {
		// We'll never get return for GET stats from sockRequest as of now,
		// just send request and see if panic or error would happen on daemon side.
		_, _, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// allow some time to send request and let daemon deal with it
	time.Sleep(1 * time.Second)

	logDone("container REST API - check GET stopped containers/stats")
}

func TestBuildApiDockerfilePath(t *testing.T) {
	// Test to make sure we stop people from trying to leave the
	// build context when specifying the path to the dockerfile
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte("FROM busybox")
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	}); err != nil {
		t.Fatalf("failed to write tar file header: %v", err)
	}
	if _, err := tw.Write(dockerfile); err != nil {
		t.Fatalf("failed to write tar file content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar archive: %v", err)
	}

	_, body, err := sockRequestRaw("POST", "/build?dockerfile=../Dockerfile", buffer, "application/x-tar")
	if err == nil {
		out, _ := readBody(body)
		t.Fatalf("Build was supposed to fail: %s", out)
	}
	out, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "must be within the build context") {
		t.Fatalf("Didn't complain about leaving build context: %s", out)
	}

	logDone("container REST API - check build w/bad Dockerfile path")
}

func TestBuildApiDockerFileRemote(t *testing.T) {
	server, err := fakeStorage(map[string]string{
		"testD": `FROM busybox
COPY * /tmp/
RUN find / -name ba*
RUN find /tmp/`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	_, body, err := sockRequestRaw("POST", "/build?dockerfile=baz&remote="+server.URL()+"/testD", nil, "application/json")
	if err != nil {
		t.Fatalf("Build failed: %s", err)
	}
	buf, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure Dockerfile exists.
	// Make sure 'baz' doesn't exist ANYWHERE despite being mentioned in the URL
	out := string(buf)
	if !strings.Contains(out, "/tmp/Dockerfile") ||
		strings.Contains(out, "baz") {
		t.Fatalf("Incorrect output: %s", out)
	}

	logDone("container REST API - check build with -f from remote")
}

func TestBuildApiLowerDockerfile(t *testing.T) {
	git, err := fakeGIT("repo", map[string]string{
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer git.Close()

	_, body, err := sockRequestRaw("POST", "/build?remote="+git.RepoURL, nil, "application/json")
	if err != nil {
		buf, _ := readBody(body)
		t.Fatalf("Build failed: %s\n%q", err, buf)
	}
	buf, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}

	out := string(buf)
	if !strings.Contains(out, "from dockerfile") {
		t.Fatalf("Incorrect output: %s", out)
	}

	logDone("container REST API - check build with lower dockerfile")
}

func TestBuildApiBuildGitWithF(t *testing.T) {
	git, err := fakeGIT("repo", map[string]string{
		"baz": `FROM busybox
RUN echo from baz`,
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	_, body, err := sockRequestRaw("POST", "/build?dockerfile=baz&remote="+git.RepoURL, nil, "application/json")
	if err != nil {
		buf, _ := readBody(body)
		t.Fatalf("Build failed: %s\n%q", err, buf)
	}
	buf, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}

	out := string(buf)
	if !strings.Contains(out, "from baz") {
		t.Fatalf("Incorrect output: %s", out)
	}

	logDone("container REST API - check build from git w/F")
}

func TestBuildApiDoubleDockerfile(t *testing.T) {
	testRequires(t, UnixCli) // dockerfile overwrites Dockerfile on Windows
	git, err := fakeGIT("repo", map[string]string{
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	_, body, err := sockRequestRaw("POST", "/build?remote="+git.RepoURL, nil, "application/json")
	if err != nil {
		t.Fatalf("Build failed: %s", err)
	}
	buf, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}

	out := string(buf)
	if !strings.Contains(out, "from Dockerfile") {
		t.Fatalf("Incorrect output: %s", out)
	}

	logDone("container REST API - check build with two dockerfiles")
}

func TestBuildApiDockerfileSymlink(t *testing.T) {
	// Test to make sure we stop people from trying to leave the
	// build context when specifying a symlink as the path to the dockerfile
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	if err := tw.WriteHeader(&tar.Header{
		Name:     "Dockerfile",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}); err != nil {
		t.Fatalf("failed to write tar file header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar archive: %v", err)
	}

	_, body, err := sockRequestRaw("POST", "/build", buffer, "application/x-tar")
	if err == nil {
		out, _ := readBody(body)
		t.Fatalf("Build was supposed to fail: %s", out)
	}
	out, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}

	// The reason the error is "Cannot locate specified Dockerfile" is because
	// in the builder, the symlink is resolved within the context, therefore
	// Dockerfile -> /etc/passwd becomes etc/passwd from the context which is
	// a nonexistent file.
	if !strings.Contains(string(out), "Cannot locate specified Dockerfile: Dockerfile") {
		t.Fatalf("Didn't complain about leaving build context: %s", out)
	}

	logDone("container REST API - check build w/bad Dockerfile symlink path")
}

// #9981 - Allow a docker created volume (ie, one in /var/lib/docker/volumes) to be used to overwrite (via passing in Binds on api start) an existing volume
func TestPostContainerBindNormalVolume(t *testing.T) {
	defer deleteAllContainers()

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "-v", "/foo", "--name=one", "busybox"))
	if err != nil {
		t.Fatal(err, out)
	}

	fooDir, err := inspectFieldMap("one", "Volumes", "/foo")
	if err != nil {
		t.Fatal(err)
	}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "create", "-v", "/foo", "--name=two", "busybox"))
	if err != nil {
		t.Fatal(err, out)
	}

	bindSpec := map[string][]string{"Binds": {fooDir + ":/foo"}}
	if status, _, err := sockRequest("POST", "/containers/two/start", bindSpec); err != nil && status != http.StatusNoContent {
		t.Fatal(err)
	}

	fooDir2, err := inspectFieldMap("two", "Volumes", "/foo")
	if err != nil {
		t.Fatal(err)
	}

	if fooDir2 != fooDir {
		t.Fatalf("expected volume path to be %s, got: %s", fooDir, fooDir2)
	}

	logDone("container REST API - can use path from normal volume as bind-mount to overwrite another volume")
}

func TestContainerApiPause(t *testing.T) {
	defer deleteAllContainers()
	defer unpauseAllContainers()
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sleep", "30")
	out, _, err := runCommandWithOutput(runCmd)

	if err != nil {
		t.Fatalf("failed to create a container: %s, %v", out, err)
	}
	ContainerID := strings.TrimSpace(out)

	if status, _, err := sockRequest("POST", "/containers/"+ContainerID+"/pause", nil); err != nil && status != http.StatusNoContent {
		t.Fatalf("POST a container pause: sockRequest failed: %v", err)
	}

	pausedContainers, err := getSliceOfPausedContainers()

	if err != nil {
		t.Fatalf("error thrown while checking if containers were paused: %v", err)
	}

	if len(pausedContainers) != 1 || stringid.TruncateID(ContainerID) != pausedContainers[0] {
		t.Fatalf("there should be one paused container and not %d", len(pausedContainers))
	}

	if status, _, err := sockRequest("POST", "/containers/"+ContainerID+"/unpause", nil); err != nil && status != http.StatusNoContent {
		t.Fatalf("POST a container pause: sockRequest failed: %v", err)
	}

	pausedContainers, err = getSliceOfPausedContainers()

	if err != nil {
		t.Fatalf("error thrown while checking if containers were paused: %v", err)
	}

	if pausedContainers != nil {
		t.Fatalf("There should be no paused container.")
	}

	logDone("container REST API - check POST containers/pause and unpause")
}

func TestContainerApiTop(t *testing.T) {
	defer deleteAllContainers()
	out, _, _ := dockerCmd(t, "run", "-d", "-i", "busybox", "/bin/sh", "-c", "cat")
	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}

	type topResp struct {
		Titles    []string
		Processes [][]string
	}
	var top topResp
	_, b, err := sockRequest("GET", "/containers/"+id+"/top?ps_args=aux", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}

	if len(top.Titles) != 11 {
		t.Fatalf("expected 11 titles, found %d: %v", len(top.Titles), top.Titles)
	}

	if top.Titles[0] != "USER" || top.Titles[10] != "COMMAND" {
		t.Fatalf("expected `USER` at `Titles[0]` and `COMMAND` at Titles[10]: %v", top.Titles)
	}
	if len(top.Processes) != 2 {
		t.Fatalf("expeted 2 processes, found %d: %v", len(top.Processes), top.Processes)
	}
	if top.Processes[0][10] != "/bin/sh -c cat" {
		t.Fatalf("expected `/bin/sh -c cat`, found: %s", top.Processes[0][10])
	}
	if top.Processes[1][10] != "cat" {
		t.Fatalf("expected `cat`, found: %s", top.Processes[1][10])
	}

	logDone("containers REST API -  GET /containers/<id>/top")
}

func TestContainerApiCommit(t *testing.T) {
	out, _, _ := dockerCmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "touch /test")
	id := strings.TrimSpace(out)

	name := "testcommit"
	_, b, err := sockRequest("POST", "/commit?repo="+name+"&testtag=tag&container="+id, nil)
	if err != nil && !strings.Contains(err.Error(), "200 OK: 201") {
		t.Fatal(err)
	}

	type resp struct {
		Id string
	}
	var img resp
	if err := json.Unmarshal(b, &img); err != nil {
		t.Fatal(err)
	}
	defer deleteImages(img.Id)

	out, err = inspectField(img.Id, "Config.Cmd")
	if out != "[/bin/sh -c touch /test]" {
		t.Fatalf("got wrong Cmd from commit: %q", out)
	}
	// sanity check, make sure the image is what we think it is
	dockerCmd(t, "run", img.Id, "ls", "/test")

	logDone("containers REST API - POST /commit")
}

func TestContainerApiCreate(t *testing.T) {
	defer deleteAllContainers()
	config := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", "touch /test && ls /test"},
	}

	_, b, err := sockRequest("POST", "/containers/create", config)
	if err != nil && !strings.Contains(err.Error(), "200 OK: 201") {
		t.Fatal(err)
	}
	type createResp struct {
		Id string
	}
	var container createResp
	if err := json.Unmarshal(b, &container); err != nil {
		t.Fatal(err)
	}

	out, _, _ := dockerCmd(t, "start", "-a", container.Id)
	if strings.TrimSpace(out) != "/test" {
		t.Fatalf("expected output `/test`, got %q", out)
	}

	logDone("containers REST API - POST /containers/create")
}

func TestContainerApiVerifyHeader(t *testing.T) {
	defer deleteAllContainers()
	config := map[string]interface{}{
		"Image": "busybox",
	}

	create := func(ct string) (int, io.ReadCloser, error) {
		jsonData := bytes.NewBuffer(nil)
		if err := json.NewEncoder(jsonData).Encode(config); err != nil {
			t.Fatal(err)
		}
		return sockRequestRaw("POST", "/containers/create", jsonData, ct)
	}

	// Try with no content-type
	_, body, err := create("")
	if err == nil {
		b, _ := readBody(body)
		t.Fatalf("expected error when content-type is not set: %q", string(b))
	}
	body.Close()
	// Try with wrong content-type
	_, body, err = create("application/xml")
	if err == nil {
		b, _ := readBody(body)
		t.Fatalf("expected error when content-type is not set: %q", string(b))
	}
	body.Close()

	// now application/json
	_, body, err = create("application/json")
	if err != nil && !strings.Contains(err.Error(), "200 OK: 201") {
		b, _ := readBody(body)
		t.Fatalf("%v - %q", err, string(b))
	}
	body.Close()

	logDone("containers REST API - verify create header")
}

// Issue 7941 - test to make sure a "null" in JSON is just ignored.
// W/o this fix a null in JSON would be parsed into a string var as "null"
func TestContainerApiPostCreateNull(t *testing.T) {
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
		"PortSpecs":null,
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

	_, body, err := sockRequestRaw("POST", "/containers/create", strings.NewReader(config), "application/json")
	if err != nil && !strings.Contains(err.Error(), "200 OK: 201") {
		b, _ := readBody(body)
		t.Fatal(err, string(b))
	}

	b, err := readBody(body)
	if err != nil {
		t.Fatal(err)
	}
	type createResp struct {
		Id string
	}
	var container createResp
	if err := json.Unmarshal(b, &container); err != nil {
		t.Fatal(err)
	}

	out, err := inspectField(container.Id, "HostConfig.CpusetCpus")
	if err != nil {
		t.Fatal(err, out)
	}
	if out != "" {
		t.Fatalf("expected empty string, got %q", out)
	}

	logDone("containers REST API - Create Null")
}
