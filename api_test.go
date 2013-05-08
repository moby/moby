package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker/auth"
	"testing"
)

func init() {
	// Make it our Store root
	runtime, err := NewRuntimeFromDirectory(unitTestStoreBase, false)
	if err != nil {
		panic(err)
	}

	// Create the "Server"
	srv := &Server{
		runtime: runtime,
	}
	go ListenAndServe("0.0.0.0:4243", srv)

}

func TestAuth(t *testing.T) {
	var out auth.AuthConfig

	out.Username = "utest"
	out.Password = "utest"
	out.Email = "utest@yopmail.com"

	_, _, err := call("POST", "/auth", out)
	if err != nil {
		t.Fatal(err)
	}

	out.Username = ""
	out.Password = ""
	out.Email = ""

	body, _, err := call("GET", "/auth", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(body, &out)
	if err != nil {
		t.Fatal(err)
	}

	if out.Username != "utest" {
		t.Errorf("Expected username to be utest, %s found", out.Username)
	}
}

func TestVersion(t *testing.T) {
	body, _, err := call("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out ApiVersion
	err = json.Unmarshal(body, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != VERSION {
		t.Errorf("Excepted version %s, %s found", VERSION, out.Version)
	}
}

func TestImages(t *testing.T) {
	body, _, err := call("GET", "/images?quiet=0&all=0", nil)
	if err != nil {
		t.Fatal(err)
	}
	var outs []ApiImages
	err = json.Unmarshal(body, &outs)
	if err != nil {
		t.Fatal(err)
	}

	if len(outs) != 1 {
		t.Errorf("Excepted 1 image, %d found", len(outs))
	}

	if outs[0].Repository != "docker-ut" {
		t.Errorf("Excepted image docker-ut, %s found", outs[0].Repository)
	}
}

func TestInfo(t *testing.T) {
	body, _, err := call("GET", "/info", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out ApiInfo
	err = json.Unmarshal(body, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != VERSION {
		t.Errorf("Excepted version %s, %s found", VERSION, out.Version)
	}
}

func TestHistory(t *testing.T) {
	body, _, err := call("GET", "/images/"+unitTestImageName+"/history", nil)
	if err != nil {
		t.Fatal(err)
	}
	var outs []ApiHistory
	err = json.Unmarshal(body, &outs)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 {
		t.Errorf("Excepted 1 line, %d found", len(outs))
	}
}

func TestImagesSearch(t *testing.T) {
	body, _, err := call("GET", "/images/search?term=redis", nil)
	if err != nil {
		t.Fatal(err)
	}
	var outs []ApiSearch
	err = json.Unmarshal(body, &outs)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) < 2 {
		t.Errorf("Excepted at least 2 lines, %d found", len(outs))
	}
}

func TestGetImage(t *testing.T) {
	obj, _, err := call("GET", "/images/"+unitTestImageName+"/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out Image
	err = json.Unmarshal(obj, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Comment != "Imported from http://get.docker.io/images/busybox" {
		t.Errorf("Error inspecting image")
	}
}

func TestCreateListStartStopRestartKillWaitDelete(t *testing.T) {
	containers := testListContainers(t, -1)
	for _, container := range containers {
		testDeleteContainer(t, container.Id)
	}
	testCreateContainer(t)
	id := testListContainers(t, 1)[0].Id
	testContainerStart(t, id)
	testContainerStop(t, id)
	testContainerRestart(t, id)
	testContainerKill(t, id)
	testContainerWait(t, id)
	testDeleteContainer(t, id)
	testListContainers(t, 0)
}

func testCreateContainer(t *testing.T) {
	config, _, err := ParseRun([]string{unitTestImageName, "touch test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = call("POST", "/containers", *config)
	if err != nil {
		t.Fatal(err)
	}
}

func testListContainers(t *testing.T, expected int) []ApiContainers {
	body, _, err := call("GET", "/containers?quiet=1&all=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	var outs []ApiContainers
	err = json.Unmarshal(body, &outs)
	if err != nil {
		t.Fatal(err)
	}
	if expected >= 0 && len(outs) != expected {
		t.Errorf("Excepted %d container, %d found", expected, len(outs))
	}
	return outs
}

func testContainerStart(t *testing.T, id string) {
	_, _, err := call("POST", "/containers/"+id+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func testContainerRestart(t *testing.T, id string) {
	_, _, err := call("POST", "/containers/"+id+"/restart?t=1", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func testContainerStop(t *testing.T, id string) {
	_, _, err := call("POST", "/containers/"+id+"/stop?t=1", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func testContainerKill(t *testing.T, id string) {
	_, _, err := call("POST", "/containers/"+id+"/kill", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func testContainerWait(t *testing.T, id string) {
	_, _, err := call("POST", "/containers/"+id+"/wait", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func testDeleteContainer(t *testing.T, id string) {
	_, _, err := call("DELETE", "/containers/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func testContainerChanges(t *testing.T, id string) {
	_, _, err := call("GET", "/containers/"+id+"/changes", nil)
	if err != nil {
		t.Fatal(err)
	}
}
