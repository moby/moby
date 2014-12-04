package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestInspectApiContainerResponse(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	// test on json marshal version
	// and latest version
	testVersions := []string{"v1.11", "latest"}

	for _, testVersion := range testVersions {
		endpoint := "/containers/" + cleanedContainerID + "/json"
		if testVersion != "latest" {
			endpoint = "/" + testVersion + endpoint
		}
		body, err := sockRequest("GET", endpoint, nil)
		if err != nil {
			t.Fatalf("sockRequest failed for %s version: %v", testVersion, err)
		}

		var inspectJSON map[string]interface{}
		if err = json.Unmarshal(body, &inspectJSON); err != nil {
			t.Fatalf("unable to unmarshal body for %s version: %v", testVersion, err)
		}

		keys := []string{"State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings", "ResolvConfPath", "HostnamePath", "HostsPath", "Name", "Driver", "ExecDriver", "MountLabel", "ProcessLabel", "Volumes", "VolumesRW"}

		if testVersion == "v1.11" {
			keys = append(keys, "ID")
		} else {
			keys = append(keys, "Id")
		}

		for _, key := range keys {
			if _, ok := inspectJSON[key]; !ok {
				t.Fatalf("%s does not exist in reponse for %s version", key, testVersion)
			}
		}
		//Issue #6830: type not properly converted to JSON/back
		if _, ok := inspectJSON["Path"].(bool); ok {
			t.Fatalf("Path of `true` should not be converted to boolean `true` via JSON marshalling")
		}
	}

	deleteAllContainers()

	logDone("container json - check keys in container json response")
}

func TestInspectApiContainerErr(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "inspect", "000111000111")
	out, _, err := runCommandWithOutput(runCmd)
	if err == nil {
		t.Fatalf("Inspect was supposed to fail, but didn't")
	}

	// Just make sure that we don't show an empty JSON array
	if strings.Contains(out, "[]") {
		t.Fatal("Should not have an empty array: %s", out)
	}
}
