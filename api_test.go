package docker

import (
	"bytes"
	"encoding/json"
	"github.com/dotcloud/docker/auth"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"
)

func TestGetAuth(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()

	authConfig := &auth.AuthConfig{
		Username: "utest",
		Password: "utest",
		Email:    "utest@yopmail.com",
	}

	authConfigJson, err := json.Marshal(authConfig)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/auth", bytes.NewReader(authConfigJson))
	if err != nil {
		t.Fatal(err)
	}

	body, err := postAuth(srv, r, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if body == nil {
		t.Fatalf("No body received\n")
	}
	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	if runtime.authConfig.Username != authConfig.Username ||
		runtime.authConfig.Password != authConfig.Password ||
		runtime.authConfig.Email != authConfig.Email {
		t.Fatalf("The auth configuration hasn't been set correctly")
	}
}

func TestGetVersion(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	body, err := getVersion(srv, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	v := &ApiVersion{}

	err = json.Unmarshal(body, v)
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != VERSION {
		t.Errorf("Excepted version %s, %s found", VERSION, v.Version)
	}
}

func TestGetInfo(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	body, err := getInfo(srv, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	infos := &ApiInfo{}
	err = json.Unmarshal(body, infos)
	if err != nil {
		t.Fatal(err)
	}
	if infos.Version != VERSION {
		t.Errorf("Excepted version %s, %s found", VERSION, infos.Version)
	}
}

func TestGetImagesJson(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	// FIXME: Do more tests with filter
	req, err := http.NewRequest("GET", "/images/json?quiet=0&all=0", nil)
	if err != nil {
		t.Fatal(err)
	}

	body, err := getImagesJson(srv, nil, req, nil)
	if err != nil {
		t.Fatal(err)
	}

	images := []ApiImages{}
	err = json.Unmarshal(body, &images)
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != 1 {
		t.Errorf("Excepted 1 image, %d found", len(images))
	}

	if images[0].Repository != "docker-ut" {
		t.Errorf("Excepted image docker-ut, %s found", images[0].Repository)
	}
}

func TestGetImagesViz(t *testing.T) {
	//FIXME: Implement this test (or remove this endpoint)
	t.Log("Test not implemented")
}

func TestGetImagesSearch(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	req, err := http.NewRequest("GET", "/images/search?term=redis", nil)
	if err != nil {
		t.Fatal(err)
	}

	body, err := getImagesSearch(srv, nil, req, nil)
	if err != nil {
		t.Fatal(err)
	}

	results := []ApiSearch{}

	err = json.Unmarshal(body, &results)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Errorf("Excepted at least 2 lines, %d found", len(results))
	}
}

func TestGetImagesHistory(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	body, err := getImagesHistory(srv, nil, nil, map[string]string{"name": unitTestImageName})
	if err != nil {
		t.Fatal(err)
	}

	history := []ApiHistory{}

	err = json.Unmarshal(body, &history)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Errorf("Excepted 1 line, %d found", len(history))
	}
}

func TestGetImagesByName(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	body, err := getImagesByName(srv, nil, nil, map[string]string{"name": unitTestImageName})
	if err != nil {
		t.Fatal(err)
	}

	img := &Image{}

	err = json.Unmarshal(body, img)
	if err != nil {
		t.Fatal(err)
	}
	if img.Comment != "Imported from http://get.docker.io/images/busybox" {
		t.Errorf("Error inspecting image")
	}
}

func TestGetContainersPs(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"echo", "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	req, err := http.NewRequest("GET", "/containers?quiet=1&all=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	body, err := getContainersPs(srv, nil, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	containers := []ApiContainers{}
	err = json.Unmarshal(body, &containers)
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("Excepted %d container, %d found", 1, len(containers))
	}
	if containers[0].Id != container.ShortId() {
		t.Fatalf("Container ID mismatch. Expected: %s, received: %s\n", container.ShortId(), containers[0].Id)
	}
}

func TestGetContainersExport(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestGetContainerChanges(t *testing.T) {
	// FIXME: Implement this test
	t.Log("Test on implemented")
	// r := httptest.NewRecorder()

	// req, err := http.NewRequest("GET", "/containers/"+id+"/changes", nil)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// body, err := getContainersChanges(srv, r, req, nil)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// if body == nil {
	// 	t.Fatalf("Body expected, received: nil\n")
	// }

func TestGetContainersByName(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestPostAuth(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	authConfigOrig := &auth.AuthConfig{
		Username: "utest",
		Email:    "utest@yopmail.com",
	}
	runtime.authConfig = authConfigOrig

	body, err := getAuth(srv, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	authConfig := &auth.AuthConfig{}
	err = json.Unmarshal(body, authConfig)
	if err != nil {
		t.Fatal(err)
	}

	if authConfig.Username != authConfigOrig.Username || authConfig.Email != authConfigOrig.Email {
		t.Errorf("The retrieve auth mismatch with the one set.")
	}
}

func TestPostCommit(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestPostBuild(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestPostImagesCreate(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestPostImagesInsert(t *testing.T) {
	//FIXME: Implement this test (or remove this endpoint)
	t.Log("Test not implemented")
}

func TestPostImagesPush(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestPostImagesTag(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test not implemented")
}

func TestPostContainersCreate(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()

	configJson, err := json.Marshal(&Config{
		Image:  GetTestImage(runtime).Id,
		Memory: 33554432,
		Cmd:    []string{"touch", "/test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/containers/create", bytes.NewReader(configJson))
	if err != nil {
		t.Fatal(err)
	}

	body, err := postContainersCreate(srv, r, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiRun := &ApiRun{}
	if err := json.Unmarshal(body, apiRun); err != nil {
		t.Fatal(err)
	}

	container := srv.runtime.Get(apiRun.Id)
	if container == nil {
		t.Fatalf("Container not created")
	}

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(container.rwPath(), "test")); err != nil {
		if os.IsNotExist(err) {
			Debugf("Err: %s", err)
			t.Fatalf("The test file has not been created")
		}
		t.Fatal(err)
	}
}

func TestPostContainersKill(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:     GetTestImage(runtime).Id,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	r := httptest.NewRecorder()

	body, err := postContainersKill(srv, r, nil, map[string]string{"name": container.Id})
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		t.Fatalf("No body expected, received: %s\n", body)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if container.State.Running {
		t.Fatalf("The container hasn't been killed")
	}
}

func TestPostContainersRestart(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:     GetTestImage(runtime).Id,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	r := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/containers/"+container.Id+"/restart?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	body, err := postContainersRestart(srv, r, req, map[string]string{"name": container.Id})
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		t.Fatalf("No body expected, received: %s\n", body)
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:     GetTestImage(runtime).Id,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	r := httptest.NewRecorder()

	body, err := postContainersStart(srv, r, nil, map[string]string{"name": container.Id})
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		t.Fatalf("No body expected, received: %s\n", body)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	if _, err = postContainersStart(srv, r, nil, map[string]string{"name": container.Id}); err == nil {
		t.Fatalf("A running containter should be able to be started")
	}

	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestPostContainersStop(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:     GetTestImage(runtime).Id,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	container.WaitTimeout(500 * time.Millisecond)

	if !container.State.Running {
		t.Errorf("Container should be running")
	}

	r := httptest.NewRecorder()

	// Note: as it is a POST request, it requires a body.
	req, err := http.NewRequest("POST", "/containers/"+container.Id+"/stop?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	body, err := postContainersStop(srv, r, req, map[string]string{"name": container.Id})
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		t.Fatalf("No body expected, received: %s\n", body)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}
	if container.State.Running {
		t.Fatalf("The container hasn't been stopped")
	}
}

func TestPostContainersWait(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(
		&Config{
			Image:     GetTestImage(runtime).Id,
			Cmd:       []string{"/bin/sleep", "1"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	setTimeout(t, "Wait timed out", 3*time.Second, func() {
		body, err := postContainersWait(srv, nil, nil, map[string]string{"name": container.Id})
		if err != nil {
			t.Fatal(err)
		}
		apiWait := &ApiWait{}
		if err := json.Unmarshal(body, apiWait); err != nil {
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
	//FIXME: Implement this test
	t.Log("Test on implemented")
}

// FIXME: Test deleting runnign container
// FIXME: Test deleting container with volume
// FIXME: Test deleting volume in use by other container
func TestDeleteContainers(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := NewBuilder(runtime).Create(&Config{
		Image: GetTestImage(runtime).Id,
		Cmd:   []string{"touch", "/test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	if err := container.Run(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()

	req, err := http.NewRequest("DELETE", "/containers/"+container.Id, nil)
	if err != nil {
		t.Fatal(err)
	}

	body, err := deleteContainers(srv, r, req, map[string]string{"name": container.Id})
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		t.Fatalf("No body expected, received: %s\n", body)
	}
	if r.Code != http.StatusNoContent {
		t.Fatalf("%d NO CONTENT expected, received %d\n", http.StatusNoContent, r.Code)
	}

	if c := runtime.Get(container.Id); c != nil {
		t.Fatalf("The container as not been deleted")
	}

	if _, err := os.Stat(path.Join(container.rwPath(), "test")); err == nil {
		t.Fatalf("The test file has not been deleted")
	}
}

func TestDeleteImages(t *testing.T) {
	//FIXME: Implement this test
	t.Log("Test on implemented")
}
