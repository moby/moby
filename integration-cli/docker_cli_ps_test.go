package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestPsListContainers(c *check.C) {

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	firstID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "-d", "busybox", "top")
	secondID := strings.TrimSpace(out)

	// not long running
	out, _ = dockerCmd(c, "run", "-d", "busybox", "true")
	thirdID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "-d", "busybox", "top")
	fourthID := strings.TrimSpace(out)

	// make sure the second is running
	if err := waitRun(secondID); err != nil {
		c.Fatalf("waiting for container failed: %v", err)
	}

	// make sure third one is not running
	dockerCmd(c, "wait", thirdID)

	// make sure the forth is running
	if err := waitRun(fourthID); err != nil {
		c.Fatalf("waiting for container failed: %v", err)
	}

	// all
	out, _ = dockerCmd(c, "ps", "-a")
	if !assertContainerList(out, []string{fourthID, thirdID, secondID, firstID}) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// running
	out, _ = dockerCmd(c, "ps")
	if !assertContainerList(out, []string{fourthID, secondID, firstID}) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// from here all flag '-a' is ignored

	// limit
	out, _ = dockerCmd(c, "ps", "-n=2", "-a")
	expected := []string{fourthID, thirdID}
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "-n=2")
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// since
	out, _ = dockerCmd(c, "ps", "--since", firstID, "-a")
	expected = []string{fourthID, thirdID, secondID}
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID)
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// before
	out, _ = dockerCmd(c, "ps", "--before", thirdID, "-a")
	expected = []string{secondID, firstID}
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--before", thirdID)
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// since & before
	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID, "-a")
	expected = []string{thirdID, secondID}
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID)
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// since & limit
	out, _ = dockerCmd(c, "ps", "--since", firstID, "-n=2", "-a")
	expected = []string{fourthID, thirdID}

	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "-n=2")
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	// before & limit
	out, _ = dockerCmd(c, "ps", "--before", fourthID, "-n=1", "-a")
	expected = []string{thirdID}
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--before", fourthID, "-n=1")
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID, "-n=1", "-a")
	expected = []string{thirdID}
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID, "-n=1")
	if !assertContainerList(out, expected) {
		c.Errorf("Container list is not in the correct order: %s", out)
	}

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

func (s *DockerSuite) TestPsListContainersSize(c *check.C) {
	dockerCmd(c, "run", "-d", "busybox", "echo", "hello")

	baseOut, _ := dockerCmd(c, "ps", "-s", "-n=1")
	baseLines := strings.Split(strings.Trim(baseOut, "\n "), "\n")
	baseSizeIndex := strings.Index(baseLines[0], "SIZE")
	baseFoundsize := baseLines[1][baseSizeIndex:]
	baseBytes, err := strconv.Atoi(strings.Split(baseFoundsize, " ")[0])
	if err != nil {
		c.Fatal(err)
	}

	name := "test_size"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "sh", "-c", "echo 1 > test")
	id, err := getIDByName(name)
	if err != nil {
		c.Fatal(err)
	}

	runCmd := exec.Command(dockerBinary, "ps", "-s", "-n=1")

	wait := make(chan struct{})
	go func() {
		out, _, err = runCommandWithOutput(runCmd)
		close(wait)
	}()
	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		c.Fatalf("Calling \"docker ps -s\" timed out!")
	}
	if err != nil {
		c.Fatal(out, err)
	}
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines) != 2 {
		c.Fatalf("Expected 2 lines for 'ps -s -n=1' output, got %d", len(lines))
	}
	sizeIndex := strings.Index(lines[0], "SIZE")
	idIndex := strings.Index(lines[0], "CONTAINER ID")
	foundID := lines[1][idIndex : idIndex+12]
	if foundID != id[:12] {
		c.Fatalf("Expected id %s, got %s", id[:12], foundID)
	}
	expectedSize := fmt.Sprintf("%d B", (2 + baseBytes))
	foundSize := lines[1][sizeIndex:]
	if !strings.Contains(foundSize, expectedSize) {
		c.Fatalf("Expected size %q, got %q", expectedSize, foundSize)
	}

}

func (s *DockerSuite) TestPsListContainersFilterStatus(c *check.C) {
	// FIXME: this should test paused, but it makes things hang and its wonky
	// this is because paused containers can't be controlled by signals

	// start exited container
	out, _ := dockerCmd(c, "run", "-d", "busybox")
	firstID := strings.TrimSpace(out)

	// make sure the exited cintainer is not running
	dockerCmd(c, "wait", firstID)

	// start running container
	out, _ = dockerCmd(c, "run", "-itd", "busybox")
	secondID := strings.TrimSpace(out)

	// filter containers by exited
	out, _ = dockerCmd(c, "ps", "-q", "--filter=status=exited")
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID[:12] {
		c.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out)
	}

	out, _ = dockerCmd(c, "ps", "-a", "-q", "--filter=status=running")
	containerOut = strings.TrimSpace(out)
	if containerOut != secondID[:12] {
		c.Fatalf("Expected id %s, got %s for running filter, output: %q", secondID[:12], containerOut, out)
	}

}

func (s *DockerSuite) TestPsListContainersFilterID(c *check.C) {

	// start container
	out, _ := dockerCmd(c, "run", "-d", "busybox")
	firstID := strings.TrimSpace(out)

	// start another container
	dockerCmd(c, "run", "-d", "busybox", "top")

	// filter containers by id
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--filter=id="+firstID)
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID[:12] {
		c.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out)
	}

}

func (s *DockerSuite) TestPsListContainersFilterName(c *check.C) {

	// start container
	out, _ := dockerCmd(c, "run", "-d", "--name=a_name_to_match", "busybox")
	firstID := strings.TrimSpace(out)

	// start another container
	dockerCmd(c, "run", "-d", "--name=b_name_to_match", "busybox", "top")

	// filter containers by name
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--filter=name=a_name_to_match")
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID[:12] {
		c.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out)
	}

}

func (s *DockerSuite) TestPsListContainersFilterLabel(c *check.C) {
	// start container
	out, _ := dockerCmd(c, "run", "-d", "-l", "match=me", "-l", "second=tag", "busybox")
	firstID := strings.TrimSpace(out)

	// start another container
	out, _ = dockerCmd(c, "run", "-d", "-l", "match=me too", "busybox")
	secondID := strings.TrimSpace(out)

	// start third container
	out, _ = dockerCmd(c, "run", "-d", "-l", "nomatch=me", "busybox")
	thirdID := strings.TrimSpace(out)

	// filter containers by exact match
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match=me")
	containerOut := strings.TrimSpace(out)
	if containerOut != firstID {
		c.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID, containerOut, out)
	}

	// filter containers by two labels
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match=me", "--filter=label=second=tag")
	containerOut = strings.TrimSpace(out)
	if containerOut != firstID {
		c.Fatalf("Expected id %s, got %s for exited filter, output: %q", firstID, containerOut, out)
	}

	// filter containers by two labels, but expect not found because of AND behavior
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match=me", "--filter=label=second=tag-no")
	containerOut = strings.TrimSpace(out)
	if containerOut != "" {
		c.Fatalf("Expected nothing, got %s for exited filter, output: %q", containerOut, out)
	}

	// filter containers by exact key
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match")
	containerOut = strings.TrimSpace(out)
	if (!strings.Contains(containerOut, firstID) || !strings.Contains(containerOut, secondID)) || strings.Contains(containerOut, thirdID) {
		c.Fatalf("Expected ids %s,%s, got %s for exited filter, output: %q", firstID, secondID, containerOut, out)
	}
}

func (s *DockerSuite) TestPsListContainersFilterExited(c *check.C) {

	dockerCmd(c, "run", "-d", "--name", "top", "busybox", "top")

	dockerCmd(c, "run", "--name", "zero1", "busybox", "true")
	firstZero, err := getIDByName("zero1")
	if err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run", "--name", "zero2", "busybox", "true")
	secondZero, err := getIDByName("zero2")
	if err != nil {
		c.Fatal(err)
	}

	runCmd := exec.Command(dockerBinary, "run", "--name", "nonzero1", "busybox", "false")
	if out, _, err := runCommandWithOutput(runCmd); err == nil {
		c.Fatal("Should fail.", out, err)
	}

	firstNonZero, err := getIDByName("nonzero1")
	if err != nil {
		c.Fatal(err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--name", "nonzero2", "busybox", "false")
	if out, _, err := runCommandWithOutput(runCmd); err == nil {
		c.Fatal("Should fail.", out, err)
	}
	secondNonZero, err := getIDByName("nonzero2")
	if err != nil {
		c.Fatal(err)
	}

	// filter containers by exited=0
	out, _ := dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=exited=0")
	ids := strings.Split(strings.TrimSpace(out), "\n")
	if len(ids) != 2 {
		c.Fatalf("Should be 2 zero exited containers got %d: %s", len(ids), out)
	}
	if ids[0] != secondZero {
		c.Fatalf("First in list should be %q, got %q", secondZero, ids[0])
	}
	if ids[1] != firstZero {
		c.Fatalf("Second in list should be %q, got %q", firstZero, ids[1])
	}

	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=exited=1")
	ids = strings.Split(strings.TrimSpace(out), "\n")
	if len(ids) != 2 {
		c.Fatalf("Should be 2 zero exited containers got %d", len(ids))
	}
	if ids[0] != secondNonZero {
		c.Fatalf("First in list should be %q, got %q", secondNonZero, ids[0])
	}
	if ids[1] != firstNonZero {
		c.Fatalf("Second in list should be %q, got %q", firstNonZero, ids[1])
	}

}

func (s *DockerSuite) TestPsRightTagName(c *check.C) {
	tag := "asybox:shmatest"
	if out, err := exec.Command(dockerBinary, "tag", "busybox", tag).CombinedOutput(); err != nil {
		c.Fatalf("Failed to tag image: %s, out: %q", err, out)
	}

	var id1 string
	if out, err := exec.Command(dockerBinary, "run", "-d", "busybox", "top").CombinedOutput(); err != nil {
		c.Fatalf("Failed to run container: %s, out: %q", err, out)
	} else {
		id1 = strings.TrimSpace(string(out))
	}

	var id2 string
	if out, err := exec.Command(dockerBinary, "run", "-d", tag, "top").CombinedOutput(); err != nil {
		c.Fatalf("Failed to run container: %s, out: %q", err, out)
	} else {
		id2 = strings.TrimSpace(string(out))
	}

	var imageID string
	if out, err := exec.Command(dockerBinary, "inspect", "-f", "{{.Id}}", "busybox").CombinedOutput(); err != nil {
		c.Fatalf("failed to get the image ID of busybox: %s, %v", out, err)
	} else {
		imageID = strings.TrimSpace(string(out))
	}

	var id3 string
	if out, err := exec.Command(dockerBinary, "run", "-d", imageID, "top").CombinedOutput(); err != nil {
		c.Fatalf("Failed to run container: %s, out: %q", err, out)
	} else {
		id3 = strings.TrimSpace(string(out))
	}

	out, err := exec.Command(dockerBinary, "ps", "--no-trunc").CombinedOutput()
	if err != nil {
		c.Fatalf("Failed to run 'ps': %s, out: %q", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// skip header
	lines = lines[1:]
	if len(lines) != 3 {
		c.Fatalf("There should be 3 running container, got %d", len(lines))
	}
	for _, line := range lines {
		f := strings.Fields(line)
		switch f[0] {
		case id1:
			if f[1] != "busybox" {
				c.Fatalf("Expected %s tag for id %s, got %s", "busybox", id1, f[1])
			}
		case id2:
			if f[1] != tag {
				c.Fatalf("Expected %s tag for id %s, got %s", tag, id2, f[1])
			}
		case id3:
			if f[1] != imageID {
				c.Fatalf("Expected %s imageID for id %s, got %s", tag, id3, f[1])
			}
		default:
			c.Fatalf("Unexpected id %s, expected %s and %s and %s", f[0], id1, id2, id3)
		}
	}
}

func (s *DockerSuite) TestPsLinkedWithNoTrunc(c *check.C) {
	if out, err := exec.Command(dockerBinary, "run", "--name=first", "-d", "busybox", "top").CombinedOutput(); err != nil {
		c.Fatalf("Output: %s, err: %s", out, err)
	}
	if out, err := exec.Command(dockerBinary, "run", "--name=second", "--link=first:first", "-d", "busybox", "top").CombinedOutput(); err != nil {
		c.Fatalf("Output: %s, err: %s", out, err)
	}
	out, err := exec.Command(dockerBinary, "ps", "--no-trunc").CombinedOutput()
	if err != nil {
		c.Fatalf("Output: %s, err: %s", out, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// strip header
	lines = lines[1:]
	expected := []string{"second", "first,second/first"}
	var names []string
	for _, l := range lines {
		fields := strings.Fields(l)
		names = append(names, fields[len(fields)-1])
	}
	if !reflect.DeepEqual(expected, names) {
		c.Fatalf("Expected array: %v, got: %v", expected, names)
	}
}

func (s *DockerSuite) TestPsGroupPortRange(c *check.C) {

	portRange := "3800-3900"
	dockerCmd(c, "run", "-d", "--name", "porttest", "-p", portRange+":"+portRange, "busybox", "top")

	out, _ := dockerCmd(c, "ps")

	// check that the port range is in the output
	if !strings.Contains(string(out), portRange) {
		c.Fatalf("docker ps output should have had the port range %q: %s", portRange, string(out))
	}

}

func (s *DockerSuite) TestPsWithSize(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "sizetest", "busybox", "top"))
	if err != nil {
		c.Fatal(out, err)
	}
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "ps", "--size"))
	if err != nil {
		c.Fatal(out, err)
	}
	if !strings.Contains(out, "virtual") {
		c.Fatalf("docker ps with --size should show virtual size of container")
	}
}

func (s *DockerSuite) TestPsListContainersFilterCreated(c *check.C) {
	// create a container
	createCmd := exec.Command(dockerBinary, "create", "busybox")
	out, _, err := runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	cID := strings.TrimSpace(out)
	shortCID := cID[:12]

	// Make sure it DOESN'T show up w/o a '-a' for normal 'ps'
	runCmd := exec.Command(dockerBinary, "ps", "-q")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}
	if strings.Contains(out, shortCID) {
		c.Fatalf("Should have not seen '%s' in ps output:\n%s", shortCID, out)
	}

	// Make sure it DOES show up as 'Created' for 'ps -a'
	runCmd = exec.Command(dockerBinary, "ps", "-a")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	hits := 0
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, shortCID) {
			continue
		}
		hits++
		if !strings.Contains(line, "Created") {
			c.Fatalf("Missing 'Created' on '%s'", line)
		}
	}

	if hits != 1 {
		c.Fatalf("Should have seen '%s' in ps -a output once:%d\n%s", shortCID, hits, out)
	}

	// filter containers by 'create' - note, no -a needed
	runCmd = exec.Command(dockerBinary, "ps", "-q", "-f", "status=created")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}
	containerOut := strings.TrimSpace(out)
	if !strings.HasPrefix(cID, containerOut) {
		c.Fatalf("Expected id %s, got %s for filter, out: %s", cID, containerOut, out)
	}
}
