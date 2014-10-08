package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestVolumesLs(t *testing.T) {
	deleteAllVolumes()

	defer deleteAllVolumes()

	numberOfVolumes := 3

	for i := 0; i < numberOfVolumes; i++ {
		cmd := exec.Command(dockerBinary, "volume", "create")
		if _, err := runCommand(cmd); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(dockerBinary, "volume", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines)-1 < numberOfVolumes {
		t.Fatalf("Volumes are not showing up in list:\n%q", out)
	}

	logDone("volumes ls - volumes are being listed")
}
