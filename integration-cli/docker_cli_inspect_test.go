package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestInspectBoolPath(t *testing.T) {
	name := "test_inspect_bool"
	buildCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	exitCode, err := runCommand(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v", err))
	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}
	defer func() {
		cmd := exec.Command(dockerBinary, "rm", name)
		exitCode, err = runCommand(cmd)
		if err != nil || exitCode != 0 {
			t.Fatal("[rm] err: %v, exitcode: %d", err, exitCode)
		}
	}()

	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", "{{json .Path}}", name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		t.Fatal("[inspect] err: %v, exitcode: %d", err, exitCode)
	}
	path := strings.TrimSpace(out)
	if path != `"true"` {
		t.Fatalf("Path %q, expected %q", path, `"true"`)
	}
	logDone("inspect - test 'true' serializing as string")
}
