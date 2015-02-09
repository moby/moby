package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// This is a heisen-test.  Because the created timestamp of images and the behavior of
// sort is not predictable it doesn't always fail.
func TestBuildHistory(t *testing.T) {
	name := "testbuildhistory"
	defer deleteImages(name)
	_, err := buildImage(name, `FROM busybox
RUN echo "A"
RUN echo "B"
RUN echo "C"
RUN echo "D"
RUN echo "E"
RUN echo "F"
RUN echo "G"
RUN echo "H"
RUN echo "I"
RUN echo "J"
RUN echo "K"
RUN echo "L"
RUN echo "M"
RUN echo "N"
RUN echo "O"
RUN echo "P"
RUN echo "Q"
RUN echo "R"
RUN echo "S"
RUN echo "T"
RUN echo "U"
RUN echo "V"
RUN echo "W"
RUN echo "X"
RUN echo "Y"
RUN echo "Z"`,
		true)

	if err != nil {
		t.Fatal(err)
	}

	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, "history", "testbuildhistory"))
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to get image history: %s, %v", out, err)
	}

	actualValues := strings.Split(out, "\n")[1:27]
	expectedValues := [26]string{"Z", "Y", "X", "W", "V", "U", "T", "S", "R", "Q", "P", "O", "N", "M", "L", "K", "J", "I", "H", "G", "F", "E", "D", "C", "B", "A"}

	for i := 0; i < 26; i++ {
		echoValue := fmt.Sprintf("echo \"%s\"", expectedValues[i])
		actualValue := actualValues[i]

		if !strings.Contains(actualValue, echoValue) {
			t.Fatalf("Expected layer \"%s\", but was: %s", expectedValues[i], actualValue)
		}
	}

	logDone("history - build history")
}

func TestHistoryExistentImage(t *testing.T) {
	historyCmd := exec.Command(dockerBinary, "history", "busybox")
	_, exitCode, err := runCommandWithOutput(historyCmd)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to get image history")
	}
	logDone("history - history on existent image must pass")
}

func TestHistoryNonExistentImage(t *testing.T) {
	historyCmd := exec.Command(dockerBinary, "history", "testHistoryNonExistentImage")
	_, exitCode, err := runCommandWithOutput(historyCmd)
	if err == nil || exitCode == 0 {
		t.Fatal("history on a non-existent image didn't result in a non-zero exit status")
	}
	logDone("history - history on non-existent image must pass")
}
