package docker

import (
	"bytes"
	"encoding/json"
	"github.com/dotcloud/docker/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

// func init() {
// 	// Make it our Store root
// 	runtime, err := NewRuntimeFromDirectory(unitTestStoreBase, false)
// 	if err != nil {
// 		panic(err)
// 	}

// 	// Create the "Server"
// 	srv := &Server{
// 		runtime: runtime,
// 	}
// 	go ListenAndServe("0.0.0.0:4243", srv, false)
// }

func TestAuth(t *testing.T) {
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

	body, err := postAuth(srv, r, req)
	if err != nil {
		t.Fatal(err)
	}
	if body == nil {
		t.Fatalf("No body received\n")
	}
	if r.Code != http.StatusOK {
		t.Fatalf("%d OK expected, received %d\n", http.StatusOK, r.Code)
	}

	authConfig = &auth.AuthConfig{}

	req, err = http.NewRequest("GET", "/auth", nil)
	if err != nil {
		t.Fatal(err)
	}

	body, err = getAuth(srv, nil, req)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(body, authConfig)
	if err != nil {
		t.Fatal(err)
	}

	if authConfig.Username != "utest" {
		t.Errorf("Expected username to be utest, %s found", authConfig.Username)
	}
}

func TestVersion(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	body, err := getVersion(srv, nil, nil)
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

func TestImages(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	// FIXME: Do more tests with filter
	req, err := http.NewRequest("GET", "/images?quiet=0&all=0", nil)
	if err != nil {
		t.Fatal(err)
	}

	body, err := getImages(srv, nil, req)
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

func TestInfo(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	body, err := getInfo(srv, nil, nil)
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

// func TestHistory(t *testing.T) {
// 	runtime, err := newTestRuntime()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer nuke(runtime)

// 	srv := &Server{runtime: runtime}

// 	req, err := http.NewRequest("GET", "/images/"+unitTestImageName+"/history", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	router := mux.NewRouter()
// 	router.Path("/images/{name:.*}/history")
// 	vars := mux.Vars(req)
// 	router.
// 	vars["name"] = unitTestImageName

// 	body, err := getImagesHistory(srv, nil, req)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	var outs []ApiHistory
// 	err = json.Unmarshal(body, &outs)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if len(outs) != 1 {
// 		t.Errorf("Excepted 1 line, %d found", len(outs))
// 	}
// }

// func TestImagesSearch(t *testing.T) {
// 	body, _, err := call("GET", "/images/search?term=redis", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	var outs []ApiSearch
// 	err = json.Unmarshal(body, &outs)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if len(outs) < 2 {
// 		t.Errorf("Excepted at least 2 lines, %d found", len(outs))
// 	}
// }

// func TestGetImage(t *testing.T) {
// 	obj, _, err := call("GET", "/images/"+unitTestImageName+"/json", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	var out Image
// 	err = json.Unmarshal(obj, &out)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if out.Comment != "Imported from http://get.docker.io/images/busybox" {
// 		t.Errorf("Error inspecting image")
// 	}
// }

// func TestCreateListStartStopRestartKillWaitDelete(t *testing.T) {
// 	containers := testListContainers(t, -1)
// 	for _, container := range containers {
// 		testDeleteContainer(t, container.Id)
// 	}
// 	testCreateContainer(t)
// 	id := testListContainers(t, 1)[0].Id
// 	testContainerStart(t, id)
// 	testContainerStop(t, id)
// 	testContainerRestart(t, id)
// 	testContainerKill(t, id)
// 	testContainerWait(t, id)
// 	testDeleteContainer(t, id)
// 	testListContainers(t, 0)
// }

// func testCreateContainer(t *testing.T) {
// 	config, _, err := ParseRun([]string{unitTestImageName, "touch test"}, nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	_, _, err = call("POST", "/containers", *config)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testListContainers(t *testing.T, expected int) []ApiContainers {
// 	body, _, err := call("GET", "/containers?quiet=1&all=1", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	var outs []ApiContainers
// 	err = json.Unmarshal(body, &outs)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if expected >= 0 && len(outs) != expected {
// 		t.Errorf("Excepted %d container, %d found", expected, len(outs))
// 	}
// 	return outs
// }

// func testContainerStart(t *testing.T, id string) {
// 	_, _, err := call("POST", "/containers/"+id+"/start", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testContainerRestart(t *testing.T, id string) {
// 	_, _, err := call("POST", "/containers/"+id+"/restart?t=1", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testContainerStop(t *testing.T, id string) {
// 	_, _, err := call("POST", "/containers/"+id+"/stop?t=1", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testContainerKill(t *testing.T, id string) {
// 	_, _, err := call("POST", "/containers/"+id+"/kill", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testContainerWait(t *testing.T, id string) {
// 	_, _, err := call("POST", "/containers/"+id+"/wait", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testDeleteContainer(t *testing.T, id string) {
// 	_, _, err := call("DELETE", "/containers/"+id, nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

// func testContainerChanges(t *testing.T, id string) {
// 	_, _, err := call("GET", "/containers/"+id+"/changes", nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }
