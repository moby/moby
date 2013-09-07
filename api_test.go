package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/dotcloud/docker/utils"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"
)

func TestGetBoolParam(t *testing.T) {
	if ret, err := getBoolParam("true"); err != nil || !ret {
		t.Fatalf("true -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("True"); err != nil || !ret {
		t.Fatalf("True -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("1"); err != nil || !ret {
		t.Fatalf("1 -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam(""); err != nil || ret {
		t.Fatalf("\"\" -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("false"); err != nil || ret {
		t.Fatalf("false -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("0"); err != nil || ret {
		t.Fatalf("0 -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("faux"); err == nil || ret {
		t.Fatalf("faux -> false, err | got %t %s", ret, err)
	}
}

func TestGetVersion(t *testing.T) {
	var err error
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()

	if err := getVersion(srv, APIVERSION, r, nil, nil); err != nil {
		t.Fatal(err)
	}

	v := &APIVersion{}
	if err = json.Unmarshal(r.Body.Bytes(), v); err != nil {
		t.Fatal(err)
	}
	if v.Version != VERSION {
		t.Errorf("Expected version %s, %s found", VERSION, v.Version)
	}
}

func TestGetInfo(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	initialImages, err := srv.runtime.graph.All()
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()

	if err := getInfo(srv, APIVERSION, r, nil, nil); err != nil {
		t.Fatal(err)
	}

	infos := &APIInfo{}
	err = json.Unmarshal(r.Body.Bytes(), infos)
	if err != nil {
		t.Fatal(err)
	}
	if infos.Images != len(initialImages) {
		t.Errorf("Expected images: %d, %d found", len(initialImages), infos.Images)
	}
}

func TestGetEvents(t *testing.T) {
	runtime := mkRuntime(t)
	srv := &Server{
		runtime:   runtime,
		events:    make([]utils.JSONMessage, 0, 64),
		listeners: make(map[string]chan utils.JSONMessage),
	}

	srv.LogEvent("fakeaction", "fakeid", "fakeimage")
	srv.LogEvent("fakeaction2", "fakeid", "fakeimage")

	req, err := http.NewRequest("GET", "/events?since=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	setTimeout(t, "", 500*time.Millisecond, func() {
		if err := getEvents(srv, APIVERSION, r, req, nil); err != nil {
			t.Fatal(err)
		}
	})

	dec := json.NewDecoder(r.Body)
	for i := 0; i < 2; i++ {
		var jm utils.JSONMessage
		if err := dec.Decode(&jm); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if jm != srv.events[i] {
			t.Fatalf("Event received it different than expected")
		}
	}

}

func TestGetImagesJSON(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

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

	if err := getImagesJSON(srv, APIVERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}

	images := []APIImages{}
	if err := json.Unmarshal(r.Body.Bytes(), &images); err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images))
	}

	found := false
	for _, img := range images {
		if img.Repository == unitTestImageName {
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

	if err := getImagesJSON(srv, APIVERSION, r2, req2, nil); err != nil {
		t.Fatal(err)
	}

	images2 := []APIImages{}
	if err := json.Unmarshal(r2.Body.Bytes(), &images2); err != nil {
		t.Fatal(err)
	}

	if len(images2) != len(initialImages) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images2))
	}

	found = false
	for _, img := range images2 {
		if img.ID == GetTestImage(runtime).ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Retrieved image Id differs, expected %s, received %+v", GetTestImage(runtime).ID, images2)
	}

	r3 := httptest.NewRecorder()

	// filter=a
	req3, err := http.NewRequest("GET", "/images/json?filter=aaaaaaaaaa", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := getImagesJSON(srv, APIVERSION, r3, req3, nil); err != nil {
		t.Fatal(err)
	}

	images3 := []APIImages{}
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

	err = getImagesJSON(srv, APIVERSION, r4, req4, nil)
	if err == nil {
		t.Fatalf("Error expected, received none")
	}

	httpError(r4, err)
	if r4.Code != http.StatusBadRequest {
		t.Fatalf("%d Bad Request expected, received %d\n", http.StatusBadRequest, r4.Code)
	}
}

func TestGetImagesViz(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()
	if err := getImagesViz(srv, APIVERSION, r, nil, nil); err != nil {
		t.Fatal(err)
	}

	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	reader := bufio.NewReader(r.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "digraph docker {\n" {
		t.Errorf("Expected digraph docker {\n, %s found", line)
	}
}

func TestGetImagesHistory(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()

	if err := getImagesHistory(srv, APIVERSION, r, nil, map[string]string{"name": unitTestImageName}); err != nil {
		t.Fatal(err)
	}

	history := []APIHistory{}
	if err := json.Unmarshal(r.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 line, %d found", len(history))
	}
}

func TestGetImagesByName(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()
	if err := getImagesByName(srv, APIVERSION, r, nil, map[string]string{"name": unitTestImageName}); err != nil {
		t.Fatal(err)
	}

	img := &Image{}
	if err := json.Unmarshal(r.Body.Bytes(), img); err != nil {
		t.Fatal(err)
	}
	if img.ID != unitTestImageID {
		t.Errorf("Error inspecting image")
	}
}

func TestGetContainersJSON(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"echo", "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	req, err := http.NewRequest("GET", "/containers/json?all=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := getContainersJSON(srv, APIVERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}
	containers := []APIContainers{}
	if err := json.Unmarshal(r.Body.Bytes(), &containers); err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("Expected %d container, %d found", 1, len(containers))
	}
	if containers[0].ID != container.ID {
		t.Fatalf("Container ID mismatch. Expected: %s, received: %s\n", container.ID, containers[0].ID)
	}
}

func TestGetContainersExport(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	// Create a container and remove a file
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"touch", "/test"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err = getContainersExport(srv, APIVERSION, r, nil, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}

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
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	// Create a container and remove a file
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"/bin/rm", "/etc/passwd"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := getContainersChanges(srv, APIVERSION, r, nil, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	changes := []Change{}
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/sh", "-c", "cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)
	defer func() {
		// Make sure the process dies before destroying runtime
		container.stdin.Close()
		container.WaitTimeout(2 * time.Second)
	}()

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Waiting for the container to be started timed out", 10*time.Second, func() {
		for {
			if container.State.Running {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	if !container.State.Running {
		t.Fatalf("Container should be running")
	}

	// Make sure sh spawn up cat
	setTimeout(t, "read/write assertion timed out", 2*time.Second, func() {
		in, _ := container.StdinPipe()
		out, _ := container.StdoutPipe()
		if err := assertPipe("hello\n", "hello", out, in, 15); err != nil {
			t.Fatal(err)
		}
	})

	r := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/"+container.ID+"/top?ps_args=u", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := getContainersTop(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	procs := APITop{}
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
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	// Create a container and remove a file
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"echo", "test"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	r := httptest.NewRecorder()
	if err := getContainersByName(srv, APIVERSION, r, nil, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	outContainer := &Container{}
	if err := json.Unmarshal(r.Body.Bytes(), outContainer); err != nil {
		t.Fatal(err)
	}
	if outContainer.ID != container.ID {
		t.Fatalf("Wrong containers retrieved. Expected %s, received %s", container.ID, outContainer.ID)
	}
}

func TestPostCommit(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}


	// Create a container and remove a file
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"touch", "/test"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/commit?repo=testrepo&testtag=tag&container="+container.ID, bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := postCommit(srv, APIVERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiID := &APIID{}
	if err := json.Unmarshal(r.Body.Bytes(), apiID); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.graph.Get(apiID.ID); err != nil {
		t.Fatalf("The image has not been commited")
	}
}

func TestPostContainersCreate(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	configJSON, err := json.Marshal(&Config{
		Image:  GetTestImage(runtime).ID,
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
	if err := postContainersCreate(srv, APIVERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiRun := &APIRun{}
	if err := json.Unmarshal(r.Body.Bytes(), apiRun); err != nil {
		t.Fatal(err)
	}

	container := srv.runtime.Get(apiRun.ID)
	if container == nil {
		t.Fatalf("Container not created")
	}

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(container.rwPath(), "test")); err != nil {
		if os.IsNotExist(err) {
			utils.Debugf("Err: %s", err)
			t.Fatalf("The test file has not been created")
		}
		t.Fatal(err)
	}
}

func TestPostContainersKill(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	r := httptest.NewRecorder()
	if err := postContainersKill(srv, APIVERSION, r, nil, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if container.State.Running {
		t.Fatalf("The container hasn't been killed")
	}
}

func TestPostContainersRestart(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	req, err := http.NewRequest("POST", "/containers/"+container.ID+"/restart?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := postContainersRestart(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	// Give some time to the process to restart
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Fatalf("Container should be running")
	}

	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestPostContainersStart(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	hostConfigJSON, err := json.Marshal(&HostConfig{})

	req, err := http.NewRequest("POST", "/containers/"+container.ID+"/start", bytes.NewReader(hostConfigJSON))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := postContainersStart(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	r = httptest.NewRecorder()
	if err = postContainersStart(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err == nil {
		t.Fatalf("A running container should be able to be started")
	}

	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestPostContainersStop(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	// Note: as it is a POST request, it requires a body.
	req, err := http.NewRequest("POST", "/containers/"+container.ID+"/stop?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := postContainersStop(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if container.State.Running {
		t.Fatalf("The container hasn't been stopped")
	}
}

func TestPostContainersWait(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/sleep", "1"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Wait timed out", 3*time.Second, func() {
		r := httptest.NewRecorder()
		if err := postContainersWait(srv, APIVERSION, r, nil, map[string]string{"name": container.ID}); err != nil {
			t.Fatal(err)
		}
		apiWait := &APIWait{}
		if err := json.Unmarshal(r.Body.Bytes(), apiWait); err != nil {
			t.Fatal(err)
		}
		if apiWait.StatusCode != 0 {
			t.Fatalf("Non zero exit code for sleep: %d\n", apiWait.StatusCode)
		}
	})

	if container.State.Running {
		t.Fatalf("The container should be stopped after wait")
	}
}

func TestPostContainersAttach(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	// Start the process
	hostConfig := &HostConfig{}
	if err := container.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	// Try to avoid the timeout in destroy. Best effort, don't check error
	defer func() {
		closeWrap(stdin, stdinPipe, stdout, stdoutPipe)
		container.Kill()
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

		req, err := http.NewRequest("POST", "/containers/"+container.ID+"/attach?stream=1&stdin=1&stdout=1&stderr=1", bytes.NewReader([]byte{}))
		if err != nil {
			t.Fatal(err)
		}

		if err := postContainersAttach(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
			t.Fatal(err)
		}
	}()

	// Acknowledge hijack
	setTimeout(t, "hijack acknowledge timed out", 2*time.Second, func() {
		stdout.Read([]byte{})
		stdout.Read(make([]byte, 4096))
	})

	setTimeout(t, "read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
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
	err = container.WaitTimeout(500 * time.Millisecond)
	if err == nil || !container.State.Running {
		t.Fatalf("/bin/cat is not running after closing stdin")
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin, _ := container.StdinPipe()
	cStdin.Close()
	container.Wait()
}

// FIXME: Test deleting running container
// FIXME: Test deleting container with volume
// FIXME: Test deleting volume in use by other container
func TestDeleteContainers(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(&Config{
		Image: GetTestImage(runtime).ID,
		Cmd:   []string{"touch", "/test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("DELETE", "/containers/"+container.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := deleteContainers(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	if c := runtime.Get(container.ID); c != nil {
		t.Fatalf("The container as not been deleted")
	}

	if _, err := os.Stat(path.Join(container.rwPath(), "test")); err == nil {
		t.Fatalf("The test file has not been deleted")
	}
}

func TestOptionsRoute(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime, enableCors: true}

	r := httptest.NewRecorder()
	router, err := createRouter(srv, false)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("OPTIONS", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(r, req)
	if r.Code != http.StatusOK {
		t.Errorf("Expected response for OPTIONS request to be \"200\", %v found.", r.Code)
	}
}

func TestGetEnabledCors(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime, enableCors: true}

	r := httptest.NewRecorder()

	router, err := createRouter(srv, false)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(r, req)
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
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	initialImages, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := srv.runtime.repositories.Set("test", "test", unitTestImageName, true); err != nil {
		t.Fatal(err)
	}

	images, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages)+1 {
		t.Errorf("Expected %d images, %d found", len(initialImages)+1, len(images))
	}

	req, err := http.NewRequest("DELETE", "/images/"+unitTestImageID, nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := deleteImages(srv, APIVERSION, r, req, map[string]string{"name": unitTestImageID}); err == nil {
		t.Fatalf("Expected conflict error, got none")
	}

	req2, err := http.NewRequest("DELETE", "/images/test:test", nil)
	if err != nil {
		t.Fatal(err)
	}

	r2 := httptest.NewRecorder()
	if err := deleteImages(srv, APIVERSION, r2, req2, map[string]string{"name": "test:test"}); err != nil {
		t.Fatal(err)
	}
	if r2.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	var outs []APIRmi
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

	if len(images) != len(initialImages) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images))
	}

	/*	if c := runtime.Get(container.Id); c != nil {
			t.Fatalf("The container as not been deleted")
		}

		if _, err := os.Stat(path.Join(container.rwPath(), "test")); err == nil {
			t.Fatalf("The test file has not been deleted")
		} */
}

func TestJsonContentType(t *testing.T) {
	if !matchesContentType("application/json", "application/json") {
		t.Fail()
	}

	if !matchesContentType("application/json; charset=utf-8", "application/json") {
		t.Fail()
	}

	if matchesContentType("dockerapplication/json", "application/json") {
		t.Fail()
	}
}

func TestPostContainersCopy(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	// Create a container and remove a file
	container, err := runtime.Create(
		&Config{
			Image: GetTestImage(runtime).ID,
			Cmd:   []string{"touch", "/test.txt"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	copyData := APICopy{HostPath: ".", Resource: "/test.txt"}

	jsonData, err := json.Marshal(copyData)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/"+container.ID+"/copy", bytes.NewReader(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/json")
	if err = postContainersCopy(srv, APIVERSION, r, req, map[string]string{"name": container.ID}); err != nil {
		t.Fatal(err)
	}

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
