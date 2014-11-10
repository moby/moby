package main

import (
	"os/exec"
	"strings"
	"testing"
  "fmt"
)

// ensure docker job list works
func TestJobsEnsureSucceeds(t *testing.T) {
  t.Fatal("always occure a error");

	versionCmd := exec.Command(dockerBinary, "jobs")
	out, _, err := runCommandWithOutput(versionCmd)
	if err != nil {
		t.Fatal("failed to execute docker version: %s, %v", out, err)
	}

  fmt.Print(out)

	stringsToCheck := []string{
    "JOB\tSTATUS",
	}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			t.Errorf("couldn't find string %v in output", linePrefix)
		}
	}

	logDone("jobs - verify that it works and that the output is properly formatted")
}
