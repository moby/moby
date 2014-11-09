package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPsListContainers(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	secondID := stripTrailingCharacters(out)

	// not long running
	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	thirdID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	fourthID := stripTrailingCharacters(out)

	// make sure third one is not running
	runCmd = exec.Command(dockerBinary, "wait", thirdID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// all
	runCmd = exec.Command(dockerBinary, "ps", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, []string{fourthID, thirdID, secondID, firstID}) {
		t.Error("Container list is not in the correct order")
	}

	// running
	runCmd = exec.Command(dockerBinary, "ps")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, []string{fourthID, secondID, firstID}) {
		t.Error("Container list is not in the correct order")
	}

	// from here all flag '-a' is ignored

	// limit
	runCmd = exec.Command(dockerBinary, "ps", "-n=2", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected := []string{fourthID, thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "-n=2")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected = []string{fourthID, thirdID, secondID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// before
	runCmd = exec.Command(dockerBinary, "ps", "--before", thirdID, "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected = []string{secondID, firstID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--before", thirdID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & before
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID, "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected = []string{thirdID, secondID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & limit
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "-n=2", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected = []string{fourthID, thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "-n=2")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// before & limit
	runCmd = exec.Command(dockerBinary, "ps", "--before", fourthID, "-n=1", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected = []string{thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--before", fourthID, "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & before & limit
	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID, "-n=1", "-a")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	expected = []string{thirdID}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	runCmd = exec.Command(dockerBinary, "ps", "--since", firstID, "--before", fourthID, "-n=1")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if !assertContainerList(out, expected) {
		t.Error("Container list is not in the correct order")
	}

	deleteAllContainers()

	logDone("ps - test ps options")
}

func assertContainerList(out string, expected []string) bool {
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines)-1 != len(expected) {
		return false
	}

	containerIDIndex := strings.Index(lines[0], "CONTAINER ID")
	for i := 0; i < len(expected); i++ {
		foundID := lines[i+1][containerIDIndex : containerIDIndex+12]
		if foundID != expected[i][:12] {
			return false
		}
	}

	return true
}

func TestPsListContainersSize(t *testing.T) {
	name := "test_size"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo 1 > test")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	id, err := getIDByName(name)
	if err != nil {
		t.Fatal(err)
	}

	runCmd = exec.Command(dockerBinary, "ps", "-s", "-n=1")
	wait := make(chan struct{})
	go func() {
		out, _, err = runCommandWithOutput(runCmd)
		close(wait)
	}()
	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		t.Fatalf("Calling \"docker ps -s\" timed out!")
	}
	if err != nil {
		t.Fatal(out, err)
	}
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	sizeIndex := strings.Index(lines[0], "SIZE")
	idIndex := strings.Index(lines[0], "CONTAINER ID")
	foundID := lines[1][idIndex : idIndex+12]
	if foundID != id[:12] {
		t.Fatalf("Expected id %s, got %s", id[:12], foundID)
	}
	expectedSize := "2 B"
	foundSize := lines[1][sizeIndex:]
	if foundSize != expectedSize {
		t.Fatalf("Expected size %q, got %q", expectedSize, foundSize)
	}

	deleteAllContainers()
	logDone("ps - test ps size")
}

func TestPsListContainersFilterStatus(t *testing.T) {
	// FIXME: this should test paused, but it makes things hang and its wonky
	// this is because paused containers can't be controlled by signals

	// start exited container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := stripTrailingCharacters(out)

	// make sure the exited cintainer is not running
	runCmd = exec.Command(dockerBinary, "wait", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// start running container
	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 360")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	secondID := stripTrailingCharacters(out)

	// filter containers by exited
	runCmd = exec.Command(dockerBinary, "ps", "-a", "-q", "--filter=status=exited")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID[:12] {
		t.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out)
	}

	runCmd = exec.Command(dockerBinary, "ps", "-a", "-q", "--filter=status=running")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerOut = strings.TrimSpace(out)
	if containerOut != secondID[:12] {
		t.Fatalf("Expected id %s, got %s for running filter, output: %q", secondID[:12], containerOut, out)
	}

	deleteAllContainers()

	logDone("ps - test ps filter status")
}

func TestPsListContainersFilterID(t *testing.T) {
	// start container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := stripTrailingCharacters(out)

	// start another container
	runCmd = exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 360")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// filter containers by id
	runCmd = exec.Command(dockerBinary, "ps", "-a", "-q", "--filter=id="+firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID[:12] {
		t.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out)
	}

	deleteAllContainers()

	logDone("ps - test ps filter id")
}

func TestPsListContainersFilterName(t *testing.T) {
	// start container
	runCmd := exec.Command(dockerBinary, "run", "-d", "--name=a_name_to_match", "busybox")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstID := stripTrailingCharacters(out)

	// start another container
	runCmd = exec.Command(dockerBinary, "run", "-d", "--name=b_name_to_match", "busybox", "sh", "-c", "sleep 360")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	// filter containers by name
	runCmd = exec.Command(dockerBinary, "ps", "-a", "-q", "--filter=name=a_name_to_match")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID[:12] {
		t.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out)
	}

	deleteAllContainers()

	logDone("ps - test ps filter name")
}

func TestPsListContainersFilterExited(t *testing.T) {
	deleteAllContainers()
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "run", "--name", "zero1", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	firstZero, err := getIDByName("zero1")
	if err != nil {
		t.Fatal(err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--name", "zero2", "busybox", "true")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	secondZero, err := getIDByName("zero2")
	if err != nil {
		t.Fatal(err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--name", "nonzero1", "busybox", "false")
	out, _, err = runCommandWithOutput(runCmd)
	if err == nil {
		t.Fatal("Should fail.", out, err)
	}
	firstNonZero, err := getIDByName("nonzero1")
	if err != nil {
		t.Fatal(err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--name", "nonzero2", "busybox", "false")
	out, _, err = runCommandWithOutput(runCmd)
	if err == nil {
		t.Fatal("Should fail.", out, err)
	}
	secondNonZero, err := getIDByName("nonzero2")
	if err != nil {
		t.Fatal(err)
	}

	// filter containers by exited=0
	runCmd = exec.Command(dockerBinary, "ps", "-a", "-q", "--no-trunc", "--filter=exited=0")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	ids := strings.Split(strings.TrimSpace(out), "\n")
	if len(ids) != 2 {
		t.Fatalf("Should be 2 zero exited containerst got %d", len(ids))
	}
	if ids[0] != secondZero {
		t.Fatalf("First in list should be %q, got %q", secondZero, ids[0])
	}
	if ids[1] != firstZero {
		t.Fatalf("Second in list should be %q, got %q", firstZero, ids[1])
	}

	runCmd = exec.Command(dockerBinary, "ps", "-a", "-q", "--no-trunc", "--filter=exited=1")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	ids = strings.Split(strings.TrimSpace(out), "\n")
	if len(ids) != 2 {
		t.Fatalf("Should be 2 zero exited containerst got %d", len(ids))
	}
	if ids[0] != secondNonZero {
		t.Fatalf("First in list should be %q, got %q", secondNonZero, ids[0])
	}
	if ids[1] != firstNonZero {
		t.Fatalf("Second in list should be %q, got %q", firstNonZero, ids[1])
	}
	logDone("ps - test ps filter exited")
}
