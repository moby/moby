package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/registry"
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

func TestGetAuth(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{
		runtime: runtime,
	}

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

	if err := postAuth(srv, API_VERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}

	if r.Code != http.StatusOK && r.Code != 0 {
		t.Fatalf("%d OK or 0 expected, received %d\n", http.StatusOK, r.Code)
	}

	newAuthConfig := registry.NewRegistry(runtime.root).GetAuthConfig(false)
	if newAuthConfig.Username != authConfig.Username ||
		newAuthConfig.Email != authConfig.Email {
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

	r := httptest.NewRecorder()

	if err := getVersion(srv, API_VERSION, r, nil, nil); err != nil {
		t.Fatal(err)
	}

	v := &ApiVersion{}
	if err = json.Unmarshal(r.Body.Bytes(), v); err != nil {
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

	r := httptest.NewRecorder()

	if err := getInfo(srv, API_VERSION, r, nil, nil); err != nil {
		t.Fatal(err)
	}

	infos := &ApiInfo{}
	err = json.Unmarshal(r.Body.Bytes(), infos)
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

	// all=0
	req, err := http.NewRequest("GET", "/images/json?all=0", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()

	if err := getImagesJson(srv, API_VERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}

	images := []ApiImages{}
	if err := json.Unmarshal(r.Body.Bytes(), &images); err != nil {
		t.Fatal(err)
	}

	if len(images) != 1 {
		t.Errorf("Excepted 1 image, %d found", len(images))
	}

	if images[0].Repository != unitTestImageName {
		t.Errorf("Excepted image %s, %s found", unitTestImageName, images[0].Repository)
	}

	r2 := httptest.NewRecorder()

	// all=1
	req2, err := http.NewRequest("GET", "/images/json?all=true", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := getImagesJson(srv, API_VERSION, r2, req2, nil); err != nil {
		t.Fatal(err)
	}

	images2 := []ApiImages{}
	if err := json.Unmarshal(r2.Body.Bytes(), &images2); err != nil {
		t.Fatal(err)
	}

	if len(images2) != 1 {
		t.Errorf("Excepted 1 image, %d found", len(images2))
	}

	if images2[0].Id != GetTestImage(runtime).Id {
		t.Errorf("Retrieved image Id differs, expected %s, received %s", GetTestImage(runtime).Id, images2[0].Id)
	}

	r3 := httptest.NewRecorder()

	// filter=a
	req3, err := http.NewRequest("GET", "/images/json?filter=a", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := getImagesJson(srv, API_VERSION, r3, req3, nil); err != nil {
		t.Fatal(err)
	}

	images3 := []ApiImages{}
	if err := json.Unmarshal(r3.Body.Bytes(), &images3); err != nil {
		t.Fatal(err)
	}

	if len(images3) != 0 {
		t.Errorf("Excepted 1 image, %d found", len(images3))
	}

	r4 := httptest.NewRecorder()

	// all=foobar
	req4, err := http.NewRequest("GET", "/images/json?all=foobar", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = getImagesJson(srv, API_VERSION, r4, req4, nil)
	if err == nil {
		t.Fatalf("Error expected, received none")
	}

	httpError(r4, err)
	if r4.Code != http.StatusBadRequest {
		t.Fatalf("%d Bad Request expected, received %d\n", http.StatusBadRequest, r4.Code)
	}
}

func TestGetImagesViz(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	r := httptest.NewRecorder()
	if err := getImagesViz(srv, API_VERSION, r, nil, nil); err != nil {
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
		t.Errorf("Excepted digraph docker {\n, %s found", line)
	}
}

func TestGetImagesSearch(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{
		runtime: runtime,
	}

	r := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/images/search?term=redis", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := getImagesSearch(srv, API_VERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}

	results := []ApiSearch{}
	if err := json.Unmarshal(r.Body.Bytes(), &results); err != nil {
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

	r := httptest.NewRecorder()

	if err := getImagesHistory(srv, API_VERSION, r, nil, map[string]string{"name": unitTestImageName}); err != nil {
		t.Fatal(err)
	}

	history := []ApiHistory{}
	if err := json.Unmarshal(r.Body.Bytes(), &history); err != nil {
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

	r := httptest.NewRecorder()
	if err := getImagesByName(srv, API_VERSION, r, nil, map[string]string{"name": unitTestImageName}); err != nil {
		t.Fatal(err)
	}

	img := &Image{}
	if err := json.Unmarshal(r.Body.Bytes(), img); err != nil {
		t.Fatal(err)
	}
	if img.Id != GetTestImage(runtime).Id || img.Comment != "Imported from http://get.docker.io/images/busybox" {
		t.Errorf("Error inspecting image")
	}
}

func TestGetContainersJson(t *testing.T) {
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

	req, err := http.NewRequest("GET", "/containers/json?all=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := getContainersJson(srv, API_VERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}
	containers := []ApiContainers{}
	if err := json.Unmarshal(r.Body.Bytes(), &containers); err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("Excepted %d container, %d found", 1, len(containers))
	}
	if containers[0].Id != container.Id {
		t.Fatalf("Container ID mismatch. Expected: %s, received: %s\n", container.Id, containers[0].Id)
	}
}

func TestGetContainersExport(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	builder := NewBuilder(runtime)

	// Create a container and remove a file
	container, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
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
	if err = getContainersExport(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err != nil {
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
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	builder := NewBuilder(runtime)

	// Create a container and remove a file
	container, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
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
	if err := getContainersChanges(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err != nil {
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

func TestGetContainersByName(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	builder := NewBuilder(runtime)

	// Create a container and remove a file
	container, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
			Cmd:   []string{"echo", "test"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	r := httptest.NewRecorder()
	if err := getContainersByName(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err != nil {
		t.Fatal(err)
	}
	outContainer := &Container{}
	if err := json.Unmarshal(r.Body.Bytes(), outContainer); err != nil {
		t.Fatal(err)
	}
	if outContainer.Id != container.Id {
		t.Fatalf("Wrong containers retrieved. Expected %s, recieved %s", container.Id, outContainer.Id)
	}
}

func TestPostAuth(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{
		runtime: runtime,
	}

	config := &auth.AuthConfig{
		Username: "utest",
		Email:    "utest@yopmail.com",
	}

	authStr := auth.EncodeAuth(config)
	auth.SaveConfig(runtime.root, authStr, config.Email)

	r := httptest.NewRecorder()
	if err := getAuth(srv, API_VERSION, r, nil, nil); err != nil {
		t.Fatal(err)
	}

	authConfig := &auth.AuthConfig{}
	if err := json.Unmarshal(r.Body.Bytes(), authConfig); err != nil {
		t.Fatal(err)
	}

	if authConfig.Username != config.Username || authConfig.Email != config.Email {
		t.Errorf("The retrieve auth mismatch with the one set.")
	}
}

func TestPostCommit(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	builder := NewBuilder(runtime)

	// Create a container and remove a file
	container, err := builder.Create(
		&Config{
			Image: GetTestImage(runtime).Id,
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

	req, err := http.NewRequest("POST", "/commit?repo=testrepo&testtag=tag&container="+container.Id, bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	if err := postCommit(srv, API_VERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiId := &ApiId{}
	if err := json.Unmarshal(r.Body.Bytes(), apiId); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.graph.Get(apiId.Id); err != nil {
		t.Fatalf("The image has not been commited")
	}
}

func TestPostImagesCreate(t *testing.T) {
	// FIXME: Use the staging in order to perform tests

	// runtime, err := newTestRuntime()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// defer nuke(runtime)

	// srv := &Server{runtime: runtime}

	// stdin, stdinPipe := io.Pipe()
	// stdout, stdoutPipe := io.Pipe()

	// c1 := make(chan struct{})
	// go func() {
	// 	defer close(c1)

	// 	r := &hijackTester{
	// 		ResponseRecorder: httptest.NewRecorder(),
	// 		in:               stdin,
	// 		out:              stdoutPipe,
	// 	}

	// 	req, err := http.NewRequest("POST", "/images/create?fromImage="+unitTestImageName, bytes.NewReader([]byte{}))
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}

	// 	body, err := postImagesCreate(srv, r, req, nil)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	if body != nil {
	// 		t.Fatalf("No body expected, received: %s\n", body)
	// 	}
	// }()

	// // Acknowledge hijack
	// setTimeout(t, "hijack acknowledge timed out", 2*time.Second, func() {
	// 	stdout.Read([]byte{})
	// 	stdout.Read(make([]byte, 4096))
	// })

	// setTimeout(t, "Waiting for imagesCreate output", 5*time.Second, func() {
	// 	reader := bufio.NewReader(stdout)
	// 	line, err := reader.ReadString('\n')
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	if !strings.HasPrefix(line, "Pulling repository d from") {
	// 		t.Fatalf("Expected Pulling repository docker-ut from..., found %s", line)
	// 	}
	// })

	// // Close pipes (client disconnects)
	// if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
	// 	t.Fatal(err)
	// }

	// // Wait for imagesCreate to finish, the client disconnected, therefore, Create finished his job
	// setTimeout(t, "Waiting for imagesCreate timed out", 10*time.Second, func() {
	// 	<-c1
	// })
}

func TestPostImagesInsert(t *testing.T) {
	// runtime, err := newTestRuntime()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// defer nuke(runtime)

	// srv := &Server{runtime: runtime}

	// stdin, stdinPipe := io.Pipe()
	// stdout, stdoutPipe := io.Pipe()

	// // Attach to it
	// c1 := make(chan struct{})
	// go func() {
	// 	defer close(c1)
	// 	r := &hijackTester{
	// 		ResponseRecorder: httptest.NewRecorder(),
	// 		in:               stdin,
	// 		out:              stdoutPipe,
	// 	}

	// 	req, err := http.NewRequest("POST", "/images/"+unitTestImageName+"/insert?path=%2Ftest&url=https%3A%2F%2Fraw.github.com%2Fdotcloud%2Fdocker%2Fmaster%2FREADME.md", bytes.NewReader([]byte{}))
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	if err := postContainersCreate(srv, r, req, nil); err != nil {
	// 		t.Fatal(err)
	// 	}
	// }()

	// // Acknowledge hijack
	// setTimeout(t, "hijack acknowledge timed out", 5*time.Second, func() {
	// 	stdout.Read([]byte{})
	// 	stdout.Read(make([]byte, 4096))
	// })

	// id := ""
	// setTimeout(t, "Waiting for imagesInsert output", 10*time.Second, func() {
	// 	for {
	// 		reader := bufio.NewReader(stdout)
	// 		id, err = reader.ReadString('\n')
	// 		if err != nil {
	// 			t.Fatal(err)
	// 		}
	// 	}
	// })

	// // Close pipes (client disconnects)
	// if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
	// 	t.Fatal(err)
	// }

	// // Wait for attach to finish, the client disconnected, therefore, Attach finished his job
	// setTimeout(t, "Waiting for CmdAttach timed out", 2*time.Second, func() {
	// 	<-c1
	// })

	// img, err := srv.runtime.repositories.LookupImage(id)
	// if err != nil {
	// 	t.Fatalf("New image %s expected but not found", id)
	// }

	// layer, err := img.layer()
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// if _, err := os.Stat(path.Join(layer, "test")); err != nil {
	// 	t.Fatalf("The test file has not been found")
	// }

	// if err := srv.runtime.graph.Delete(img.Id); err != nil {
	// 	t.Fatal(err)
	// }
}

func TestPostImagesPush(t *testing.T) {
	//FIXME: Use staging in order to perform tests
	// runtime, err := newTestRuntime()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// defer nuke(runtime)

	// srv := &Server{runtime: runtime}

	// stdin, stdinPipe := io.Pipe()
	// stdout, stdoutPipe := io.Pipe()

	// c1 := make(chan struct{})
	// go func() {
	// 	r := &hijackTester{
	// 		ResponseRecorder: httptest.NewRecorder(),
	// 		in:               stdin,
	// 		out:              stdoutPipe,
	// 	}

	// 	req, err := http.NewRequest("POST", "/images/docker-ut/push", bytes.NewReader([]byte{}))
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}

	// 	body, err := postImagesPush(srv, r, req, map[string]string{"name": "docker-ut"})
	// 	close(c1)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	if body != nil {
	// 		t.Fatalf("No body expected, received: %s\n", body)
	// 	}
	// }()

	// // Acknowledge hijack
	// setTimeout(t, "hijack acknowledge timed out", 2*time.Second, func() {
	// 	stdout.Read([]byte{})
	// 	stdout.Read(make([]byte, 4096))
	// })

	// setTimeout(t, "Waiting for imagesCreate output", 5*time.Second, func() {
	// 	reader := bufio.NewReader(stdout)
	// 	line, err := reader.ReadString('\n')
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	if !strings.HasPrefix(line, "Processing checksum") {
	// 		t.Fatalf("Processing checksum..., found %s", line)
	// 	}
	// })

	// // Close pipes (client disconnects)
	// if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
	// 	t.Fatal(err)
	// }

	// // Wait for imagesPush to finish, the client disconnected, therefore, Push finished his job
	// setTimeout(t, "Waiting for imagesPush timed out", 10*time.Second, func() {
	// 	<-c1
	// })
}

func TestPostImagesTag(t *testing.T) {
	// FIXME: Use staging in order to perform tests

	// runtime, err := newTestRuntime()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// defer nuke(runtime)

	// srv := &Server{runtime: runtime}

	// r := httptest.NewRecorder()

	// req, err := http.NewRequest("POST", "/images/docker-ut/tag?repo=testrepo&tag=testtag", bytes.NewReader([]byte{}))
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// body, err := postImagesTag(srv, r, req, map[string]string{"name": "docker-ut"})
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// if body != nil {
	// 	t.Fatalf("No body expected, received: %s\n", body)
	// }
	// if r.Code != http.StatusCreated {
	// 	t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	// }
}

func TestPostContainersCreate(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

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

	r := httptest.NewRecorder()
	if err := postContainersCreate(srv, API_VERSION, r, req, nil); err != nil {
		t.Fatal(err)
	}
	if r.Code != http.StatusCreated {
		t.Fatalf("%d Created expected, received %d\n", http.StatusCreated, r.Code)
	}

	apiRun := &ApiRun{}
	if err := json.Unmarshal(r.Body.Bytes(), apiRun); err != nil {
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
			utils.Debugf("Err: %s", err)
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
	if err := postContainersKill(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err != nil {
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

	req, err := http.NewRequest("POST", "/containers/"+container.Id+"/restart?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := postContainersRestart(srv, API_VERSION, r, req, map[string]string{"name": container.Id}); err != nil {
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
	if err := postContainersStart(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err != nil {
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
	if err = postContainersStart(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err == nil {
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

	// Note: as it is a POST request, it requires a body.
	req, err := http.NewRequest("POST", "/containers/"+container.Id+"/stop?t=1", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := postContainersStop(srv, API_VERSION, r, req, map[string]string{"name": container.Id}); err != nil {
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
		r := httptest.NewRecorder()
		if err := postContainersWait(srv, API_VERSION, r, nil, map[string]string{"name": container.Id}); err != nil {
			t.Fatal(err)
		}
		apiWait := &ApiWait{}
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

	// Start the process
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	// Attach to it
	c1 := make(chan struct{})
	go func() {
		defer close(c1)

		r := &hijackTester{
			ResponseRecorder: httptest.NewRecorder(),
			in:               stdin,
			out:              stdoutPipe,
		}

		req, err := http.NewRequest("POST", "/containers/"+container.Id+"/attach?stream=1&stdin=1&stdout=1&stderr=1", bytes.NewReader([]byte{}))
		if err != nil {
			t.Fatal(err)
		}

		if err := postContainersAttach(srv, API_VERSION, r, req, map[string]string{"name": container.Id}); err != nil {
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
	setTimeout(t, "Waiting for CmdAttach timed out", 2*time.Second, func() {
		<-c1
	})

	// We closed stdin, expect /bin/cat to still be running
	// Wait a little bit to make sure container.monitor() did his thing
	err = container.WaitTimeout(500 * time.Millisecond)
	if err == nil || !container.State.Running {
		t.Fatalf("/bin/cat is not running after closing stdin")
	}

	// Try to avoid the timeoout in destroy. Best effort, don't check error
	cStdin, _ := container.StdinPipe()
	cStdin.Close()
	container.Wait()
}

// FIXME: Test deleting running container
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

	req, err := http.NewRequest("DELETE", "/containers/"+container.Id, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	if err := deleteContainers(srv, API_VERSION, r, req, map[string]string{"name": container.Id}); err != nil {
		t.Fatal(err)
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
	t.Log("Test not implemented")
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
