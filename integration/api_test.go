package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/utils"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetVersion(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	var err error
	r := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	// FIXME getting the version should require an actual running Server
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)

	v := &docker.APIVersion{}
	if err = json.Unmarshal(r.Body.Bytes(), v); err != nil {
		t.Fatal(err)
	}
	if v.Version != docker.VERSION {
		t.Errorf("Expected version %s, %s found", docker.VERSION, v.Version)
	}
}

func TestGetInfo(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	initialImages, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()

	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)

	infos := &docker.APIInfo{}
	err = json.Unmarshal(r.Body.Bytes(), infos)
	if err != nil {
		t.Fatal(err)
	}
	if infos.Images != len(initialImages) {
		t.Errorf("Expected images: %d, %d found", len(initialImages), infos.Images)
	}
}

func TestGetEvents(t *testing.T) {
	eng := NewTestEngine(t)
	srv := mkServerFromEngine(eng, t)
	// FIXME: we might not need runtime, why not simply nuke
	// the engine?
	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)

	var events []*utils.JSONMessage
	for _, parts := range [][3]string{
		{"fakeaction", "fakeid", "fakeimage"},
		{"fakeaction2", "fakeid", "fakeimage"},
	} {
		action, id, from := parts[0], parts[1], parts[2]
		ev := srv.LogEvent(action, id, from)
		events = append(events, ev)
	}

	req, err := http.NewRequest("GET", "/events?since=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	setTimeout(t, "", 500*time.Millisecond, func() {
		if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
			t.Fatal(err)
		}
		assertHttpNotError(r, t)
	})

	dec := json.NewDecoder(r.Body)
	for i := 0; i < 2; i++ {
		var jm utils.JSONMessage
		if err := dec.Decode(&jm); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if jm != *events[i] {
			t.Fatalf("Event received it different than expected")
		}
	}

}

func TestGetImagesJSON(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// all=0

	initialImages, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/images/json?all=0", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()

	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)

	images := []docker.APIImages{}
	if err := json.Unmarshal(r.Body.Bytes(), &images); err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images))
	}

	found := false
	for _, img := range images {
		if strings.Contains(img.RepoTags[0], unitTestImageName) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected image %s, %+v found", unitTestImageName, images)
	}

	r2 := httptest.NewRecorder()

	// all=1

	initialImages, err = srv.Images(true, "")
	if err != nil {
		t.Fatal(err)
	}

	req2, err := http.NewRequest("GET", "/images/json?all=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r2, req2); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r2, t)

	images2 := []docker.APIImages{}
	if err := json.Unmarshal(r2.Body.Bytes(), &images2); err != nil {
		t.Fatal(err)
	}

	if len(images2) != len(initialImages) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images2))
	}

	found = false
	for _, img := range images2 {
		if img.ID == unitTestImageID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Retrieved image Id differs, expected %s, received %+v", unitTestImageID, images2)
	}

	r3 := httptest.NewRecorder()

	// filter=a
	req3, err := http.NewRequest("GET", "/images/json?filter=aaaaaaaaaa", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := docker.ServeRequest(srv, docker.APIVERSION, r3, req3); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r3, t)

	images3 := []docker.APIImages{}
	if err := json.Unmarshal(r3.Body.Bytes(), &images3); err != nil {
		t.Fatal(err)
	}

	if len(images3) != 0 {
		t.Errorf("Expected 0 image, %d found", len(images3))
	}

	r4 := httptest.NewRecorder()

	// all=foobar
	req4, err := http.NewRequest("GET", "/images/json?all=foobar", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := docker.ServeRequest(srv, docker.APIVERSION, r4, req4); err != nil {
		t.Fatal(err)
	}
	// Don't assert against HTTP error since we expect an error
	if r4.Code != http.StatusBadRequest {
		t.Fatalf("%d Bad Request expected, received %d\n", http.StatusBadRequest, r4.Code)
	}
}

func TestGetImagesHistory(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	r := httptest.NewRecorder()

	req, err := http.NewRequest("GET", fmt.Sprintf("/images/%s/history", unitTestImageName), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)

	history := []docker.APIHistory{}
	if err := json.Unmarshal(r.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 line, %d found", len(history))
	}
}

func TestGetImagesByName(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	req, err := http.NewRequest("GET", "/images/"+unitTestImageName+"/json", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)

	img := &docker.Image{}
	if err := json.Unmarshal(r.Body.Bytes(), img); err != nil {
		t.Fatal(err)
	}
	if img.ID != unitTestImageID {
		t.Errorf("Error inspecting image")
	}
}

func TestGetContainersJSON(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	beginLen := len(srv.Containers(true, false, -1, "", ""))

	containerID := createTestContainer(eng, &docker.Config{
		Image: unitTestImageID,
		Cmd:   []string{"echo", "test"},
	}, t)

	if containerID == "" {
		t.Fatalf("Received empty container ID")
	}

	req, err := http.NewRequest("GET", "/containers/json?all=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	containers := []docker.APIContainers{}
	if err := json.Unmarshal(r.Body.Bytes(), &containers); err != nil {
		t.Fatal(err)
	}
	if len(containers) != beginLen+1 {
		t.Fatalf("Expected %d container, %d found (started with: %d)", beginLen+1, len(containers), beginLen)
	}
	if containers[0].ID != containerID {
		t.Fatalf("Container ID mismatch. Expected: %s, received: %s\n", containerID, containers[0].ID)
	}
}

func TestGetContainersExport(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&docker.Config{
			Image: unitTestImageID,
			Cmd:   []string{"touch", "/test"},
		},
		t,
	)
	containerRun(eng, containerID, t)

	r := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/containers/"+containerID+"/export", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
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
		if h.Name == "./test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("The created test file has not been found in the exported image")
	}
}

func TestGetContainersChanges(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&docker.Config{
			Image: unitTestImageID,
			Cmd:   []string{"/bin/rm", "/etc/passwd"},
		},
		t,
	)
	containerRun(eng, containerID, t)

	r := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/containers/"+containerID+"/changes", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	changes := []docker.Change{}
	if err := json.Unmarshal(r.Body.Bytes(), &changes); err != nil {
		t.Fatal(err)
	}

	// Check the changelog
	success := false
	for _, elem := range changes {
		if elem.Path == "/etc/passwd" && elem.Kind == 2 {
			success = true
		}
	}
	if !success {
		t.Fatalf("/etc/passwd as been removed but is not present in the diff")
	}
}

func TestGetContainersTop(t *testing.T) {
	t.Skip("Fixme. Skipping test for now. Reported error when testing using dind: 'api_test.go:527: Expected 2 processes, found 0.'")
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/sh", "-c", "cat"},
			OpenStdin: true,
		},
		t,
	)
	defer func() {
		// Make sure the process dies before destroying runtime
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
	req, err := http.NewRequest("GET", "/"+containerID+"/top?ps_args=u", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	procs := docker.APITop{}
	if err := json.Unmarshal(r.Body.Bytes(), &procs); err != nil {
		t.Fatal(err)
	}

	if len(procs.Titles) != 11 {
		t.Fatalf("Expected 11 titles, found %d.", len(procs.Titles))
	}
	if procs.Titles[0] != "USER" || procs.Titles[10] != "COMMAND" {
		t.Fatalf("Expected Titles[0] to be USER and Titles[10] to be COMMAND, found %s and %s.", procs.Titles[0], procs.Titles[10])
	}

	if len(procs.Processes) != 2 {
		t.Fatalf("Expected 2 processes, found %d.", len(procs.Processes))
	}
	if procs.Processes[0][10] != "/bin/sh" && procs.Processes[0][10] != "cat" {
		t.Fatalf("Expected `cat` or `/bin/sh`, found %s.", procs.Processes[0][10])
	}
	if procs.Processes[1][10] != "/bin/sh" && procs.Processes[1][10] != "cat" {
		t.Fatalf("Expected `cat` or `/bin/sh`, found %s.", procs.Processes[1][10])
	}
}

func TestGetContainersByName(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&docker.Config{
			Image: unitTestImageID,
			Cmd:   []string{"echo", "test"},
		},
		t,
	)

	r := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/containers/"+containerID+"/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	outContainer := &docker.Container{}
	if err := json.Unmarshal(r.Body.Bytes(), outContainer); err != nil {
		t.Fatal(err)
	}
	if outContainer.ID != containerID {
		t.Fatalf("Wrong containers retrieved. Expected %s, received %s", containerID, outContainer.ID)
	}
}

func TestPostCommit(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&docker.Config{
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
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiID := &docker.APIID{}
	if err := json.Unmarshal(r.Body.Bytes(), apiID); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ImageInspect(apiID.ID); err != nil {
		t.Fatalf("The image has not been committed")
	}
}

func TestPostContainersCreate(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	configJSON, err := json.Marshal(&docker.Config{
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
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiRun := &docker.APIRun{}
	if err := json.Unmarshal(r.Body.Bytes(), apiRun); err != nil {
		t.Fatal(err)
	}
	containerID := apiRun.ID

	containerAssertExists(eng, containerID, t)
	containerRun(eng, containerID, t)

	if !containerFileExists(eng, containerID, "test", t) {
		t.Fatal("Test file was not created")
	}
}

func TestPostContainersKill(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
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
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
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
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
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
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
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
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(
		eng,
		&docker.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
		t,
	)

	hostConfigJSON, err := json.Marshal(&docker.HostConfig{})

	req, err := http.NewRequest("POST", "/containers/"+containerID+"/start", bytes.NewReader(hostConfigJSON))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	r := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	containerAssertExists(eng, containerID, t)
	// Give some time to the process to start
	// FIXME: use Wait once it's available as a job
	containerWaitTimeout(eng, containerID, t)
	if !containerRunning(eng, containerID, t) {
		t.Errorf("Container should be running")
	}

	r = httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	// Starting an already started container should return an error
	// FIXME: verify a precise error code. There is a possible bug here
	// which causes this to return 404 even though the container exists.
	assertHttpError(r, t)
	containerAssertExists(eng, containerID, t)
	containerKill(eng, containerID, t)
}

// Expected behaviour: using / as a bind mount source should throw an error
func TestRunErrorBindMountRootSource(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(
		eng,
		&docker.Config{
			Image:     unitTestImageID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
		t,
	)

	hostConfigJSON, err := json.Marshal(&docker.HostConfig{
		Binds: []string{"/:/tmp"},
	})

	req, err := http.NewRequest("POST", "/containers/"+containerID+"/start", bytes.NewReader(hostConfigJSON))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	r := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusInternalServerError {
		containerKill(eng, containerID, t)
		t.Fatal("should have failed to run when using / as a source for the bind mount")
	}
}

func TestPostContainersStop(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
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
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if containerRunning(eng, containerID, t) {
		t.Fatalf("The container hasn't been stopped")
	}
}

func TestPostContainersWait(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
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
		if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
			t.Fatal(err)
		}
		assertHttpNotError(r, t)
		apiWait := &docker.APIWait{}
		if err := json.Unmarshal(r.Body.Bytes(), apiWait); err != nil {
			t.Fatal(err)
		}
		if apiWait.StatusCode != 0 {
			t.Fatalf("Non zero exit code for sleep: %d\n", apiWait.StatusCode)
		}
	})

	if containerRunning(eng, containerID, t) {
		t.Fatalf("The container should be stopped after wait")
	}
}

func TestPostContainersAttach(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
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

		if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
			t.Fatal(err)
		}
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
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
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

		if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
			t.Fatal(err)
		}
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

// FIXME: Test deleting running container
// FIXME: Test deleting container with volume
// FIXME: Test deleting volume in use by other container
func TestDeleteContainers(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	containerID := createTestContainer(eng,
		&docker.Config{
			Image: unitTestImageID,
			Cmd:   []string{"touch", "/test"},
		},
		t,
	)
	req, err := http.NewRequest("DELETE", "/containers/"+containerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	containerAssertNotExists(eng, containerID, t)
}

func TestOptionsRoute(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)
	r := httptest.NewRecorder()
	req, err := http.NewRequest("OPTIONS", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r, t)
	if r.Code != http.StatusOK {
		t.Errorf("Expected response for OPTIONS request to be \"200\", %v found.", r.Code)
	}
}

func TestGetEnabledCors(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)
	r := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
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
	if allowHeaders != "Origin, X-Requested-With, Content-Type, Accept" {
		t.Errorf("Expected header Access-Control-Allow-Headers to be \"Origin, X-Requested-With, Content-Type, Accept\", %s found.", allowHeaders)
	}
	if allowMethods != "GET, POST, DELETE, PUT, OPTIONS" {
		t.Errorf("Expected hearder Access-Control-Allow-Methods to be \"GET, POST, DELETE, PUT, OPTIONS\", %s found.", allowMethods)
	}
}

func TestDeleteImages(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	initialImages, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := srv.ContainerTag(unitTestImageName, "test", "test", false); err != nil {
		t.Fatal(err)
	}
	images, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images[0].RepoTags) != len(initialImages[0].RepoTags)+1 {
		t.Errorf("Expected %d images, %d found", len(initialImages)+1, len(images))
	}

	req, err := http.NewRequest("DELETE", "/images/"+unitTestImageID, nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusConflict {
		t.Fatalf("Expected http status 409-conflict, got %v", r.Code)
	}

	req2, err := http.NewRequest("DELETE", "/images/test:test", nil)
	if err != nil {
		t.Fatal(err)
	}

	r2 := httptest.NewRecorder()
	if err := docker.ServeRequest(srv, docker.APIVERSION, r2, req2); err != nil {
		t.Fatal(err)
	}
	assertHttpNotError(r2, t)
	if r2.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	var outs []docker.APIRmi
	if err := json.Unmarshal(r2.Body.Bytes(), &outs); err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 {
		t.Fatalf("Expected %d event (untagged), got %d", 1, len(outs))
	}
	images, err = srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images[0].RepoTags) != len(initialImages[0].RepoTags) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images))
	}
}

func TestPostContainersCopy(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// Create a container and remove a file
	containerID := createTestContainer(eng,
		&docker.Config{
			Image: unitTestImageID,
			Cmd:   []string{"touch", "/test.txt"},
		},
		t,
	)
	containerRun(eng, containerID, t)

	r := httptest.NewRecorder()
	copyData := docker.APICopy{HostPath: ".", Resource: "/test.txt"}

	jsonData, err := json.Marshal(copyData)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/"+containerID+"/copy", bytes.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/json")
	if err := docker.ServeRequest(srv, docker.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
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
