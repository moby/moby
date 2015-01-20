package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

func TestContainerApiGetAll(t *testing.T) {
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

	body, err := sockRequest("GET", "/containers/json?all=1", nil)
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

	deleteAllContainers()

	logDone("container REST API - check GET json/all=1")
}

func TestContainerApiGetExport(t *testing.T) {
	name := "exportcontainer"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "touch", "/test")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	body, err := sockRequest("GET", "/containers/"+name+"/export", nil)
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
	deleteAllContainers()

	logDone("container REST API - check GET containers/export")
}

func TestContainerApiGetChanges(t *testing.T) {
	name := "changescontainer"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "rm", "/etc/passwd")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	body, err := sockRequest("GET", "/containers/"+name+"/changes", nil)
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

	deleteAllContainers()

	logDone("container REST API - check GET containers/changes")
}

func TestContainerApiStartVolumeBinds(t *testing.T) {
	defer deleteAllContainers()
	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	if _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && !strings.Contains(err.Error(), "201 Created") {
		t.Fatal(err)
	}

	bindPath, err := ioutil.TempDir(os.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}

	config = map[string]interface{}{
		"Binds": []string{bindPath + ":/tmp"},
	}
	if _, err := sockRequest("POST", "/containers/"+name+"/start", config); err != nil && !strings.Contains(err.Error(), "204 No Content") {
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

	if _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && !strings.Contains(err.Error(), "201 Created") {
		t.Fatal(err)
	}

	config = map[string]interface{}{
		"VolumesFrom": []string{volName},
	}
	if _, err := sockRequest("POST", "/containers/"+name+"/start", config); err != nil && !strings.Contains(err.Error(), "204 No Content") {
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

	if _, err := sockRequest("POST", "/containers/create?name="+name, config); err != nil && !strings.Contains(err.Error(), "201 Created") {
		t.Fatal(err)
	}

	bindPath, err := ioutil.TempDir(os.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}

	config = map[string]interface{}{
		"VolumesFrom": []string{volName},
		"Binds":       []string{bindPath + ":/tmp"},
	}
	if _, err := sockRequest("POST", "/containers/"+name+"/start", config); err != nil && !strings.Contains(err.Error(), "204 No Content") {
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
