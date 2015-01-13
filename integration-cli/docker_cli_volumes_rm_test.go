package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestVolumesRm(t *testing.T) {
	defer deleteAllContainers()
	deleteAllVolumes()

	cmd := exec.Command(dockerBinary, "volume", "create", "--name", "kevin_flynn")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--name", "sam_flynn", "-v", "kevin_flynn:/foo", "busybox")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	// This should fail since a container is using it
	cmd = exec.Command(dockerBinary, "volume", "rm", "kevin_flynn")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil || !strings.Contains(out, "is being used") {
		t.Fatal(err, out)
	}

	cmd = exec.Command(dockerBinary, "rm", "sam_flynn")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "volume", "rm", "kevin_flynn")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines)-1 != 0 {
		t.Fatalf("Volumes not removed properly\n%q", out)
	}

	logDone("volume rm - volumes are removed")
}
