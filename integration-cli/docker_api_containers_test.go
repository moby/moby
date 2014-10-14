package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"testing"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

func TestContainerApiGetAll(t *testing.T) {
	startCount, err := getContainerCount()
	if err != nil {
		t.Fatalf("Cannot query container count: %v", err)
	}

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	testContainerId := stripTrailingCharacters(out)

	body, err := sockRequest("GET", "/containers/json?all=1")
	if err != nil {
		t.Fatalf("GET all containers sockRequest failed: %v", err)
	}

	var inspectJSON []map[string]interface{}
	if err = json.Unmarshal(body, &inspectJSON); err != nil {
		t.Fatalf("unable to unmarshal response body: %v", err)
	}

	if len(inspectJSON) != startCount+1 {
		t.Fatalf("Expected %d container(s), %d found (started with: %d)", startCount+1, len(inspectJSON), startCount)
	}
	if id, _ := inspectJSON[0]["Id"]; id != testContainerId {
		t.Fatalf("Container ID mismatch. Expected: %s, received: %s\n", testContainerId, id)
	}

	deleteAllContainers()

	logDone("container REST API - check GET json/all=1")
}

func TestContainerApiGetExport(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "touch", "/test")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	testContainerId := stripTrailingCharacters(out)

	body, err := sockRequest("GET", "/containers/"+testContainerId+"/export")
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
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "rm", "/etc/passwd")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error on container creation: %v, output: %q", err, out)
	}

	testContainerId := stripTrailingCharacters(out)

	body, err := sockRequest("GET", "/containers/"+testContainerId+"/changes")
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
