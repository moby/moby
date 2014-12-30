// +build !test_execdriver_lxc

package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDiffEnsureOnlyKmsgAndPtmx(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sleep", "0")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanCID := stripTrailingCharacters(out)

	diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
	out, _, err = runCommandWithOutput(diffCmd)
	if err != nil {
		t.Fatalf("failed to run diff: %s, %v", out, err)
	}
	deleteContainer(cleanCID)

	expected := map[string]bool{
		"C /dev":      true,
		"A /dev/full": true, // busybox
		"C /dev/ptmx": true, // libcontainer
		"A /dev/kmsg": true, // lxc
	}

	for _, line := range strings.Split(out, "\n") {
		if line != "" && !expected[line] {
			t.Errorf("%q is shown in the diff but shouldn't", line)
		}
	}

	logDone("diff - ensure that only kmsg and ptmx in diff")
}
