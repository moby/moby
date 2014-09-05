package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
)

func TestInspectContainerResponse(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to create a container: %v %v", out, err))

	cleanedContainerID := stripTrailingCharacters(out)

	// test on json marshal version
	// and latest version
	testVersions := []string{"v1.11", "latest"}

	for _, testVersion := range testVersions {
		endpoint := "/containers/" + cleanedContainerID + "/json"
		if testVersion != "latest" {
			endpoint = "/" + testVersion + endpoint
		}
		body, err := sockRequest("GET", endpoint)
		if err != nil {
			t.Fatal("sockRequest failed for %s version: %v", testVersion, err)
		}

		var inspect_json map[string]interface{}
		if err = json.Unmarshal(body, &inspect_json); err != nil {
			t.Fatalf("unable to unmarshal body for %s version: %v", testVersion, err)
		}

		keys := []string{"State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings", "ResolvConfPath", "HostnamePath", "HostsPath", "Name", "Driver", "ExecDriver", "MountLabel", "ProcessLabel", "Volumes", "VolumesRW"}

		if testVersion == "v1.11" {
			keys = append(keys, "ID")
		} else {
			keys = append(keys, "Id")
		}

		for _, key := range keys {
			if _, ok := inspect_json[key]; !ok {
				t.Fatalf("%s does not exist in reponse for %s version", key, testVersion)
			}
		}
	}

	deleteAllContainers()

	logDone("container json - check keys in container json response")
}
