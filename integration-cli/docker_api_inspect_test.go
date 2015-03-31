package main

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestInspectApiContainerResponse(t *testing.T) {
	defer deleteAllContainers()

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

		keys := []string{"State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings", "ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "ExecDriver", "MountLabel", "ProcessLabel", "Volumes", "VolumesRW"}

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

	logDone("container json - check keys in container json response")
}

func TestInspectApiExecResponse(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sleep", "60")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %s, %v", out, err)
	}

	defer deleteAllContainers()

	cID := stripTrailingCharacters(out)

	// Start an exec cmd
	execCmd := exec.Command(dockerBinary, "exec", "-d", cID, "ssh", "-c", "whoami")
	out, _, err = runCommandWithOutput(execCmd)
	if err != nil {
		t.Fatalf("failed to create exec: %s, %v", out, err)
	}

	// Get exec cmd's config (inspect) info
	execID := stripTrailingCharacters(out)
	body, err := sockRequest("GET", "/exec/"+execID+"/json", nil)
	if err != nil {
		t.Fatalf("sockRequest failed for: %v", err)
	}

	var inspectJSON map[string]interface{}
	if err = json.Unmarshal(body, &inspectJSON); err != nil {
		t.Fatalf("unable to unmarshal body: %v", err)
	}

	keys := []string{"ID", "Running", "ExitCode", "ProcessConfig", "OpenStdin", "OpenStderr", "OpenStdout", "Container"}

	// Just verify the fields are there, irrespective of their values
	for _, key := range keys {
		if _, ok := inspectJSON[key]; !ok {
			t.Fatalf("%s does not exist in reponse", key)
		}
	}

	// Now make sure we can get the same data using a short version of the ID
	shortID := execID[:10]
	body, err = sockRequest("GET", "/exec/"+shortID+"/json", nil)
	if err != nil {
		t.Fatalf("sockRequest failed using shortID: %v", err)
	}

	// Now kill the container and make sure the exec inspect fails
	runCmd = exec.Command(dockerBinary, "rm", "-f", cID)
	runCommand(runCmd)

	body, err = sockRequest("GET", "/exec/"+execID+"/json", nil)
	if err == nil {
		t.Fatalf("sockRequest was supposed to fail but didn't")
	}

	// And for fun make sure the shortID fails too
	body, err = sockRequest("GET", "/exec/"+shortID+"/json", nil)
	if err == nil {
		t.Fatalf("sockRequest was supposed to fail on shortID but didn't")
	}

	logDone("exec json - check keys in exec json response")
}
