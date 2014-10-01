package docker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

func TestSaveImageAndThenLoad(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	// save image
	r := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/images/"+unitTestImageID+"/get", nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}
	tarball := r.Body

	// delete the image
	r = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", "/images/"+unitTestImageID, nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	// make sure there is no image
	r = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/images/"+unitTestImageID+"/get", nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusNotFound {
		t.Fatalf("%d NotFound expected, received %d\n", http.StatusNotFound, r.Code)
	}

	// load the image
	r = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/images/load", tarball)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	// finally make sure the image is there
	r = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/images/"+unitTestImageID+"/get", nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}
}

func TestGetContainersTop(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/sh", "-c", "cat"},
			OpenStdin: true,
		},
		t,
	)
	defer func() {
		// Make sure the process dies before destroying daemon
		containerKill(eng, containerID, t)
		containerWait(eng, containerID, t)
	}()

	startContainer(eng, containerID, t)

	setTimeout(t, "Waiting for the container to be started timed out", 10*time.Second, func() {
		for {
			if containerRunning(eng, containerID, t) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	if !containerRunning(eng, containerID, t) {
		t.Fatalf("Container should be running")
	}

	// Make sure sh spawn up cat
	setTimeout(t, "read/write assertion timed out", 2*time.Second, func() {
		in, out := containerAttach(eng, containerID, t)
		if err := assertPipe("hello\n", "hello", out, in, 150); err != nil {
			t.Fatal(err)
		}
	})

	r := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/containers/"+containerID+"/top?ps_args=aux", nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	var procs engine.Env
	if err := procs.Decode(r.Body); err != nil {
		t.Fatal(err)
	}

	if len(procs.GetList("Titles")) != 11 {
		t.Fatalf("Expected 11 titles, found %d.", len(procs.GetList("Titles")))
	}
	if procs.GetList("Titles")[0] != "USER" || procs.GetList("Titles")[10] != "COMMAND" {
		t.Fatalf("Expected Titles[0] to be USER and Titles[10] to be COMMAND, found %s and %s.", procs.GetList("Titles")[0], procs.GetList("Titles")[10])
	}
	processes := [][]string{}
	if err := procs.GetJson("Processes", &processes); err != nil {
		t.Fatal(err)
	}
	if len(processes) != 2 {
		t.Fatalf("Expected 2 processes, found %d.", len(processes))
	}
	if processes[0][10] != "/bin/sh -c cat" {
		t.Fatalf("Expected `/bin/sh -c cat`, found %s.", processes[0][10])
	}
	if processes[1][10] != "/bin/sh -c cat" {
		t.Fatalf("Expected `/bin/sh -c cat`, found %s.", processes[1][10])
	}
}

func TestPostCommit(t *testing.T) {
	eng := NewTestEngine(t)
	b := &builder.BuilderJob{Engine: eng}
	b.Install()
	defer mkDaemonFromEngine(eng, t).Nuke()

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image: unitTestImageID,
			Cmd:   []string{"touch", "/test"},
		},
		t,
	)

	containerRun(eng, containerID, t)

	req, err := http.NewRequest("POST", "/commit?repo=testrepo&testtag=tag&container="+containerID, bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	var env engine.Env
	if err := env.Decode(r.Body); err != nil {
		t.Fatal(err)
	}
	if err := eng.Job("image_inspect", env.Get("Id")).Run(); err != nil {
		t.Fatalf("The image has not been committed")
	}
}

func TestPostContainersCreate(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	configJSON, err := json.Marshal(&runconfig.Config{
		Image:  unitTestImageID,
		Memory: 33554432,
		Cmd:    []string{"touch", "/test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/create", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	var apiRun engine.Env
	if err := apiRun.Decode(r.Body); err != nil {
		t.Fatal(err)
	}
	containerID := apiRun.Get("Id")

	containerAssertExists(eng, containerID, t)
	containerRun(eng, containerID, t)

	if !containerFileExists(eng, containerID, "test", t) {
		t.Fatal("Test file was not created")
	}
}

func TestPostJsonVerify(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	configJSON, err := json.Marshal(&runconfig.Config{
		Image:  unitTestImageID,
		Memory: 33554432,
		Cmd:    []string{"touch", "/test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/create", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()

	server.ServeRequest(eng, api.APIVERSION, r, req)

	// Don't add Content-Type header
	// req.Header.Set("Content-Type", "application/json")

	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusInternalServerError || !strings.Contains(((*r.Body).String()), "application/json") {
		t.Fatal("Create should have failed due to no Content-Type header - got:", r)
	}

	// Now add header but with wrong type and retest
	req.Header.Set("Content-Type", "application/xml")

	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusInternalServerError || !strings.Contains(((*r.Body).String()), "application/json") {
		t.Fatal("Create should have failed due to wrong Content-Type header - got:", r)
	}
}

// Issue 7941 - test to make sure a "null" in JSON is just ignored.
// W/o this fix a null in JSON would be parsed into a string var as "null"
func TestPostCreateNull(t *testing.T) {
	eng := NewTestEngine(t)
	daemon := mkDaemonFromEngine(eng, t)
	defer daemon.Nuke()

	configStr := fmt.Sprintf(`{
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
		"Image":"%s",
		"Volumes":{},
		"WorkingDir":"",
		"Entrypoint":null,
		"NetworkDisabled":false,
		"OnBuild":null}`, unitTestImageID)

	req, err := http.NewRequest("POST", "/containers/create", strings.NewReader(configStr))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	var apiRun engine.Env
	if err := apiRun.Decode(r.Body); err != nil {
		t.Fatal(err)
	}
	containerID := apiRun.Get("Id")

	containerAssertExists(eng, containerID, t)

	c, _ := daemon.Get(containerID)
	if c.Config.Cpuset != "" {
		t.Fatalf("Cpuset should have been empty - instead its:" + c.Config.Cpuset)
	}
}

func TestPostContainersKill(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
		t,
	)

	startContainer(eng, containerID, t)

	// Give some time to the process to start
	containerWaitTimeout(eng, containerID, t)

	if !containerRunning(eng, containerID, t) {
		t.Errorf("Container should be running")
	}

	r := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/containers/"+containerID+"/kill", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if containerRunning(eng, containerID, t) {
		t.Fatalf("The container hasn't been killed")
	}
}

func TestPostContainersRestart(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/top"},
			OpenStdin: true,
		},
		t,
	)

	startContainer(eng, containerID, t)

	// Give some time to the process to start
	containerWaitTimeout(eng, containerID, t)

	if !containerRunning(eng, containerID, t) {
		t.Errorf("Container should be running")
	}

	req, err := http.NewRequest("POST", "/containers/"+containerID+"/restart?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	// Give some time to the process to restart
	containerWaitTimeout(eng, containerID, t)

	if !containerRunning(eng, containerID, t) {
		t.Fatalf("Container should be running")
	}

	containerKill(eng, containerID, t)
}

func TestPostContainersStart(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(
		eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
		t,
	)

	hostConfigJSON, err := json.Marshal(&runconfig.HostConfig{})

	req, err := http.NewRequest("POST", "/containers/"+containerID+"/start", bytes.NewReader(hostConfigJSON))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	containerAssertExists(eng, containerID, t)

	req, err = http.NewRequest("POST", "/containers/"+containerID+"/start", bytes.NewReader(hostConfigJSON))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	r = httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)

	// Starting an already started container should return a 304
	assertHttpNotError(r, t)
	if r.Code != http.StatusNotModified {
		t.Fatalf("%d NOT MODIFIER expected, received %d\n", http.StatusNotModified, r.Code)
	}
	containerAssertExists(eng, containerID, t)
	containerKill(eng, containerID, t)
}

func TestPostContainersStop(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/top"},
			OpenStdin: true,
		},
		t,
	)

	startContainer(eng, containerID, t)

	// Give some time to the process to start
	containerWaitTimeout(eng, containerID, t)

	if !containerRunning(eng, containerID, t) {
		t.Errorf("Container should be running")
	}

	// Note: as it is a POST request, it requires a body.
	req, err := http.NewRequest("POST", "/containers/"+containerID+"/stop?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if containerRunning(eng, containerID, t) {
		t.Fatalf("The container hasn't been stopped")
	}

	req, err = http.NewRequest("POST", "/containers/"+containerID+"/stop?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	r = httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)

	// Stopping an already stopper container should return a 304
	assertHttpNotError(r, t)
	if r.Code != http.StatusNotModified {
		t.Fatalf("%d NOT MODIFIER expected, received %d\n", http.StatusNotModified, r.Code)
	}
}

func TestPostContainersWait(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/sleep", "1"},
			OpenStdin: true,
		},
		t,
	)
	startContainer(eng, containerID, t)

	setTimeout(t, "Wait timed out", 3*time.Second, func() {
		r := httptest.NewRecorder()
		req, err := http.NewRequest("POST", "/containers/"+containerID+"/wait", bytes.NewReader([]byte{}))
		if err != nil {
			t.Fatal(err)
		}
		server.ServeRequest(eng, api.APIVERSION, r, req)
		assertHttpNotError(r, t)
		var apiWait engine.Env
		if err := apiWait.Decode(r.Body); err != nil {
			t.Fatal(err)
		}
		if apiWait.GetInt("StatusCode") != 0 {
			t.Fatalf("Non zero exit code for sleep: %d\n", apiWait.GetInt("StatusCode"))
		}
	})

	if containerRunning(eng, containerID, t) {
		t.Fatalf("The container should be stopped after wait")
	}
}

func TestPostContainersAttach(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
		t,
	)
	// Start the process
	startContainer(eng, containerID, t)

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	// Try to avoid the timeout in destroy. Best effort, don't check error
	defer func() {
		closeWrap(stdin, stdinPipe, stdout, stdoutPipe)
		containerKill(eng, containerID, t)
	}()

	// Attach to it
	c1 := make(chan struct{})
	go func() {
		defer close(c1)

		r := &hijackTester{
			ResponseRecorder: httptest.NewRecorder(),
			in:               stdin,
			out:              stdoutPipe,
		}

		req, err := http.NewRequest("POST", "/containers/"+containerID+"/attach?stream=1&stdin=1&stdout=1&stderr=1", bytes.NewReader([]byte{}))
		if err != nil {
			t.Fatal(err)
		}

		server.ServeRequest(eng, api.APIVERSION, r, req)
		assertHttpNotError(r.ResponseRecorder, t)
	}()

	// Acknowledge hijack
	setTimeout(t, "hijack acknowledge timed out", 2*time.Second, func() {
		stdout.Read([]byte{})
		stdout.Read(make([]byte, 4096))
	})

	setTimeout(t, "read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", string([]byte{1, 0, 0, 0, 0, 0, 0, 6})+"hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (client disconnects)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// Wait for attach to finish, the client disconnected, therefore, Attach finished his job
	setTimeout(t, "Waiting for CmdAttach timed out", 10*time.Second, func() {
		<-c1
	})

	// We closed stdin, expect /bin/cat to still be running
	// Wait a little bit to make sure container.monitor() did his thing
	containerWaitTimeout(eng, containerID, t)

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin, _ := containerAttach(eng, containerID, t)
	cStdin.Close()
	containerWait(eng, containerID, t)
}

func TestPostContainersAttachStderr(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/sh", "-c", "/bin/cat >&2"},
			OpenStdin: true,
		},
		t,
	)
	// Start the process
	startContainer(eng, containerID, t)

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	// Try to avoid the timeout in destroy. Best effort, don't check error
	defer func() {
		closeWrap(stdin, stdinPipe, stdout, stdoutPipe)
		containerKill(eng, containerID, t)
	}()

	// Attach to it
	c1 := make(chan struct{})
	go func() {
		defer close(c1)

		r := &hijackTester{
			ResponseRecorder: httptest.NewRecorder(),
			in:               stdin,
			out:              stdoutPipe,
		}

		req, err := http.NewRequest("POST", "/containers/"+containerID+"/attach?stream=1&stdin=1&stdout=1&stderr=1", bytes.NewReader([]byte{}))
		if err != nil {
			t.Fatal(err)
		}

		server.ServeRequest(eng, api.APIVERSION, r, req)
		assertHttpNotError(r.ResponseRecorder, t)
	}()

	// Acknowledge hijack
	setTimeout(t, "hijack acknowledge timed out", 2*time.Second, func() {
		stdout.Read([]byte{})
		stdout.Read(make([]byte, 4096))
	})

	setTimeout(t, "read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", string([]byte{2, 0, 0, 0, 0, 0, 0, 6})+"hello", stdout, stdinPipe, 150); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (client disconnects)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// Wait for attach to finish, the client disconnected, therefore, Attach finished his job
	setTimeout(t, "Waiting for CmdAttach timed out", 10*time.Second, func() {
		<-c1
	})

	// We closed stdin, expect /bin/cat to still be running
	// Wait a little bit to make sure container.monitor() did his thing
	containerWaitTimeout(eng, containerID, t)

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin, _ := containerAttach(eng, containerID, t)
	cStdin.Close()
	containerWait(eng, containerID, t)
}

func TestOptionsRoute(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	r := httptest.NewRecorder()
	req, err := http.NewRequest("OPTIONS", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusOK {
		t.Errorf("Expected response for OPTIONS request to be \"200\", %v found.", r.Code)
	}
}

func TestGetEnabledCors(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	r := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)
	if r.Code != http.StatusOK {
		t.Errorf("Expected response for OPTIONS request to be \"200\", %v found.", r.Code)
	}

	allowOrigin := r.Header().Get("Access-Control-Allow-Origin")
	allowHeaders := r.Header().Get("Access-Control-Allow-Headers")
	allowMethods := r.Header().Get("Access-Control-Allow-Methods")

	if allowOrigin != "*" {
		t.Errorf("Expected header Access-Control-Allow-Origin to be \"*\", %s found.", allowOrigin)
	}
	if allowHeaders != "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth" {
		t.Errorf("Expected header Access-Control-Allow-Headers to be \"Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth\", %s found.", allowHeaders)
	}
	if allowMethods != "GET, POST, DELETE, PUT, OPTIONS" {
		t.Errorf("Expected hearder Access-Control-Allow-Methods to be \"GET, POST, DELETE, PUT, OPTIONS\", %s found.", allowMethods)
	}
}

func TestDeleteImages(t *testing.T) {
	eng := NewTestEngine(t)
	//we expect errors, so we disable stderr
	eng.Stderr = ioutil.Discard
	defer mkDaemonFromEngine(eng, t).Nuke()

	initialImages := getImages(eng, t, true, "")

	if err := eng.Job("tag", unitTestImageName, "test", "test").Run(); err != nil {
		t.Fatal(err)
	}

	images := getImages(eng, t, true, "")

	if len(images.Data[0].GetList("RepoTags")) != len(initialImages.Data[0].GetList("RepoTags"))+1 {
		t.Errorf("Expected %d images, %d found", len(initialImages.Data[0].GetList("RepoTags"))+1, len(images.Data[0].GetList("RepoTags")))
	}

	req, err := http.NewRequest("DELETE", "/images/"+unitTestImageID, nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusConflict {
		t.Fatalf("Expected http status 409-conflict, got %v", r.Code)
	}

	req2, err := http.NewRequest("DELETE", "/images/test:test", nil)
	if err != nil {
		t.Fatal(err)
	}

	r2 := httptest.NewRecorder()
	server.ServeRequest(eng, api.APIVERSION, r2, req2)
	assertHttpNotError(r2, t)
	if r2.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	outs := engine.NewTable("Created", 0)
	if _, err := outs.ReadListFrom(r2.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	if len(outs.Data) != 1 {
		t.Fatalf("Expected %d event (untagged), got %d", 1, len(outs.Data))
	}
	images = getImages(eng, t, false, "")

	if images.Len() != initialImages.Len() {
		t.Errorf("Expected %d image, %d found", initialImages.Len(), images.Len())
	}
}

func TestPostContainersCopy(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&runconfig.Config{
			Image: unitTestImageID,
			Cmd:   []string{"touch", "/test.txt"},
		},
		t,
	)
	containerRun(eng, containerID, t)

	r := httptest.NewRecorder()

	var copyData engine.Env
	copyData.Set("Resource", "/test.txt")
	copyData.Set("HostPath", ".")

	jsonData := bytes.NewBuffer(nil)
	if err := copyData.Encode(jsonData); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/"+containerID+"/copy", jsonData)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/json")
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)

	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	found := false
	for tarReader := tar.NewReader(r.Body); ; {
		h, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if h.Name == "test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("The created test file has not been found in the copied output")
	}
}

func TestPostContainersCopyWhenContainerNotFound(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	r := httptest.NewRecorder()

	var copyData engine.Env
	copyData.Set("Resource", "/test.txt")
	copyData.Set("HostPath", ".")

	jsonData := bytes.NewBuffer(nil)
	if err := copyData.Encode(jsonData); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/id_not_found/copy", jsonData)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/json")
	server.ServeRequest(eng, api.APIVERSION, r, req)
	if r.Code != http.StatusNotFound {
		t.Fatalf("404 expected for id_not_found Container, received %v", r.Code)
	}
}

// Regression test for https://github.com/docker/docker/issues/6231
func TestConstainersStartChunkedEncodingHostConfig(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	r := httptest.NewRecorder()

	var testData engine.Env
	testData.Set("Image", "docker-test-image")
	testData.SetAuto("Volumes", map[string]struct{}{"/foo": {}})
	testData.Set("Cmd", "true")
	jsonData := bytes.NewBuffer(nil)
	if err := testData.Encode(jsonData); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/create?name=chunk_test", jsonData)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/json")
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)

	var testData2 engine.Env
	testData2.SetAuto("Binds", []string{"/tmp:/foo"})
	jsonData = bytes.NewBuffer(nil)
	if err := testData2.Encode(jsonData); err != nil {
		t.Fatal(err)
	}

	req, err = http.NewRequest("POST", "/containers/chunk_test/start", jsonData)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/json")
	// This is a cheat to make the http request do chunked encoding
	// Otherwise (just setting the Content-Encoding to chunked) net/http will overwrite
	// http://golang.org/src/pkg/net/http/request.go?s=11980:12172
	req.ContentLength = -1
	server.ServeRequest(eng, api.APIVERSION, r, req)
	assertHttpNotError(r, t)

	type config struct {
		HostConfig struct {
			Binds []string
		}
	}

	req, err = http.NewRequest("GET", "/containers/chunk_test/json", nil)
	if err != nil {
		t.Fatal(err)
	}

	r2 := httptest.NewRecorder()
	req.Header.Add("Content-Type", "application/json")
	server.ServeRequest(eng, api.APIVERSION, r2, req)
	assertHttpNotError(r, t)

	c := config{}

	json.Unmarshal(r2.Body.Bytes(), &c)

	if len(c.HostConfig.Binds) == 0 {
		t.Fatal("Chunked Encoding not handled")
	}

	if c.HostConfig.Binds[0] != "/tmp:/foo" {
		t.Fatal("Chunked encoding not properly handled, execpted binds to be /tmp:/foo, got:", c.HostConfig.Binds[0])
	}
}

// Mocked types for tests
type NopConn struct {
	io.ReadCloser
	io.Writer
}

func (c *NopConn) LocalAddr() net.Addr                { return nil }
func (c *NopConn) RemoteAddr() net.Addr               { return nil }
func (c *NopConn) SetDeadline(t time.Time) error      { return nil }
func (c *NopConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *NopConn) SetWriteDeadline(t time.Time) error { return nil }

type hijackTester struct {
	*httptest.ResponseRecorder
	in  io.ReadCloser
	out io.Writer
}

func (t *hijackTester) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	bufrw := bufio.NewReadWriter(bufio.NewReader(t.in), bufio.NewWriter(t.out))
	conn := &NopConn{
		ReadCloser: t.in,
		Writer:     t.out,
	}
	return conn, bufrw, nil
}
