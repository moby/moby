package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// start an exec session and return its ID
func createExec(containerID string, config string) (string, error) {
	endpoint := "/containers/" + containerID + "/exec"
	response, err := sockRequestPayload("POST", endpoint, bytes.NewBufferString(config))
	if err != nil {
		return "", fmt.Errorf("execCreate Request failed %v", err)
	}

	// get the exec ID from create response
	var createResp map[string]interface{}
	if err := json.Unmarshal(response, &createResp); err != nil {
		return "", fmt.Errorf("execCreate invalid response %q, %v", string(response), err)
	}
	execID, ok := createResp["Id"].(string)
	if !ok {
		return "", fmt.Errorf("missing Id in response %q", string(response))
	}

	// start the exec task
	endpoint = "/exec/" + execID + "/start"
	_, err = sockRequestPayload("POST", endpoint, bytes.NewBufferString(config))
	if err != nil {
		return "", fmt.Errorf("execStart Request failed %v", err)
	}
	return execID, nil
}

func TestExecApiStop(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	defer deleteAllContainers()
	cleanedContainerID := stripTrailingCharacters(out)

	// create an exec task
	execConfig := fmt.Sprintf(`{"Cmd":[ "/bin/sleep", "10" ], "Container":"%s"}`, cleanedContainerID)
	execID, err := createExec(cleanedContainerID, execConfig)
	if err != nil {
		t.Fatal(err)
	}

	// assert that we have the required tasks
	runCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	if !strings.Contains(out, "/bin/sleep 10") {
		t.Fatalf("failed to start exec task. Current processes: %s", out)
	}

	// stop the exec task
	_, err = sockRequestPayload("POST", "/exec/"+execID+"/stop", nil)
	if err != nil {
		t.Fatalf("execStop Request failed %v", err)
	}

	// assert that we do not have nsenter-exec tasks
	runCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	if strings.Contains(out, "/bin/sleep 10") {
		t.Fatalf("failed to stop exec task. Current processes: %s", out)
	}

	logDone("exec REST API - stop an exec process")
}

func TestExecApiKill(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	defer deleteAllContainers()
	cleanedContainerID := stripTrailingCharacters(out)

	// create an exec task
	execConfig := fmt.Sprintf(`{"Cmd":[ "/bin/sleep", "10" ], "Container":"%s"}`, cleanedContainerID)
	execID, err := createExec(cleanedContainerID, execConfig)
	if err != nil {
		t.Fatal(err)
	}

	// assert that we have the exec tasks running
	runCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	if !strings.Contains(out, "/bin/sleep 10") {
		t.Fatalf("failed to start exec task. Current processes: %s", out)
	}

	// kill the exec task
	_, err = sockRequestPayload("POST", "/exec/"+execID+"/kill", nil)
	if err != nil {
		t.Fatalf("execKill Request failed %v", err)
	}

	// assert that we do not have task
	runCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	if strings.Contains(out, "/bin/sleep 10") {
		t.Fatalf("failed to kill exec task. Current processes: %s", out)
	}

	logDone("exec REST API - kill an exec process")
}
