package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildHistoryRewriting(t *testing.T) {
	name := "testbuildhistoryrewriting"
	defer deleteImages(name)
	_, err := buildImage(name, `FROM busybox
RUN echo "A"
RUN echo "B"
MARK
RUN echo "C"
RUN echo "D"
RUN echo "E"
RUN echo "F"
RUN echo "G"
RUN echo "H"
SQUASH my new thing
RUN echo "I"
MARK
RUN echo "J"
SQUASH
RUN echo "K"
MARK
RUN echo "L"
RUN echo "M"
SQUASH ["test", "json"]
RUN echo "N"`,
		true)

	if err != nil {
		t.Fatal(err)
	}

	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, "history", "testbuildhistoryrewriting"))
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to get image history: %s, %v", out, err)
	}

	history := strings.Split(strings.TrimSpace(out), "\n")[1:]

	assertLine := func(n int, search string) {
		if !strings.Contains(history[n], search) {
			t.Fatalf("Expected to find `%s` in history line `%s`", search, history[n])
		}
	}
	assertLine(0, "echo \"N\"")
	assertLine(1, "test json")
	assertLine(2, "echo \"K\"")
	assertLine(3, "#(nop) SQUASH")
	assertLine(4, "echo \"I\"")
	assertLine(5, "my new thing")
	assertLine(6, "echo \"B\"")

	logDone("history - build history rewriting")
}

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
	name := "testhistoryimagewithcomment"
	defer deleteContainer(name)
	defer deleteImages(name)

	// make a image through docker commit <container id> [ -m messages ]
	//runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %s, %v", out, err)
	}

	waitCmd := exec.Command(dockerBinary, "wait", name)
	if out, _, err := runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}

	comment := "This_is_a_comment"

	commitCmd := exec.Command(dockerBinary, "commit", "-m="+comment, name, name)
	if out, _, err := runCommandWithOutput(commitCmd); err != nil {
		t.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	// test docker history <image id> to check comment messages
	historyCmd := exec.Command(dockerBinary, "history", name)
	out, exitCode, err := runCommandWithOutput(historyCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to get image history: %s, %v", out, err)
	}

	outputTabs := strings.Fields(strings.Split(out, "\n")[1])
	//outputTabs := regexp.MustCompile("  +").Split(outputLine, -1)
	actualValue := outputTabs[len(outputTabs)-1]

	if !strings.Contains(actualValue, comment) {
		t.Fatalf("Expected comments %q, but found %q", comment, actualValue)
	}

	logDone("history - history on image with comment")
}
