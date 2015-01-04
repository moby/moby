package main

import (
	"fmt"
	"os/exec"
	"regexp"
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

func TestHistoryImageWithComment(t *testing.T) {

	// make a image through docker commit <container id> [ -m messages ]
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", "-m=This is a comment", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		t.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	cleanedImageID := stripTrailingCharacters(out)
	deleteContainer(cleanedContainerID)
	defer deleteImages(cleanedImageID)

	// test docker history <image id> to check comment messages
	historyCmd := exec.Command(dockerBinary, "history", cleanedImageID)
	out, exitCode, err := runCommandWithOutput(historyCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to get image history: %s, %v", out, err)
	}

	expectedValue := "This is a comment"

	outputLine := strings.Split(out, "\n")[1]
	outputTabs := regexp.MustCompile("  +").Split(outputLine, -1)
	actualValue := outputTabs[len(outputTabs)-1]

	if !strings.Contains(actualValue, expectedValue) {
		t.Fatalf("Expected comments \"%s\", but found \"%s\"", expectedValue, actualValue)
	}

	logDone("history - history on image with comment")
}
