package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-check/check"
	"sort"

	"github.com/docker/docker/pkg/stringid"
)

func (s *DockerSuite) TestPsListContainersBase(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	c.Assert(waitRun(secondID), check.IsNil)

	// make sure third one is not running
	dockerCmd(c, "wait", thirdID)

	// make sure the forth is running
	c.Assert(waitRun(fourthID), check.IsNil)

	// all
	out, _ = dockerCmd(c, "ps", "-a")
	if !assertContainerList(out, []string{fourthID, thirdID, secondID, firstID}) {
		c.Errorf("ALL: Container list is not in the correct order: \n%s", out)
	}

	// running
	out, _ = dockerCmd(c, "ps")
	if !assertContainerList(out, []string{fourthID, secondID, firstID}) {
		c.Errorf("RUNNING: Container list is not in the correct order: \n%s", out)
	}

	// from here all flag '-a' is ignored

	// limit
	out, _ = dockerCmd(c, "ps", "-n=2", "-a")
	expected := []string{fourthID, thirdID}
	if !assertContainerList(out, expected) {
		c.Errorf("LIMIT & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "-n=2")
	if !assertContainerList(out, expected) {
		c.Errorf("LIMIT: Container list is not in the correct order: \n%s", out)
	}

	// since
	out, _ = dockerCmd(c, "ps", "--since", firstID, "-a")
	expected = []string{fourthID, thirdID, secondID}
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID)
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE: Container list is not in the correct order: \n%s", out)
	}

	// before
	out, _ = dockerCmd(c, "ps", "--before", thirdID, "-a")
	expected = []string{secondID, firstID}
	if !assertContainerList(out, expected) {
		c.Errorf("BEFORE & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--before", thirdID)
	if !assertContainerList(out, expected) {
		c.Errorf("BEFORE: Container list is not in the correct order: \n%s", out)
	}

	// since & before
	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID, "-a")
	expected = []string{thirdID, secondID}
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE, BEFORE & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID)
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE, BEFORE: Container list is not in the correct order: \n%s", out)
	}

	// since & limit
	out, _ = dockerCmd(c, "ps", "--since", firstID, "-n=2", "-a")
	expected = []string{fourthID, thirdID}

	if !assertContainerList(out, expected) {
		c.Errorf("SINCE, LIMIT & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "-n=2")
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE, LIMIT: Container list is not in the correct order: \n%s", out)
	}

	// before & limit
	out, _ = dockerCmd(c, "ps", "--before", fourthID, "-n=1", "-a")
	expected = []string{thirdID}
	if !assertContainerList(out, expected) {
		c.Errorf("BEFORE, LIMIT & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--before", fourthID, "-n=1")
	if !assertContainerList(out, expected) {
		c.Errorf("BEFORE, LIMIT: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID, "-n=1", "-a")
	expected = []string{thirdID}
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE, BEFORE, LIMIT & ALL: Container list is not in the correct order: \n%s", out)
	}

	out, _ = dockerCmd(c, "ps", "--since", firstID, "--before", fourthID, "-n=1")
	if !assertContainerList(out, expected) {
		c.Errorf("SINCE, BEFORE, LIMIT: Container list is not in the correct order: \n%s", out)
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
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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

	out, _, _ = dockerCmdWithTimeout(time.Second*60, "ps", "-a", "-q", "--filter=status=rubbish")
	if !strings.Contains(out, "Unrecognised filter value for status") {
		c.Fatalf("Expected error response due to invalid status filter output: %q", out)
	}

}

func (s *DockerSuite) TestPsListContainersFilterID(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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

// Test for the ancestor filter for ps.
// There is also the same test but with image:tag@digest in docker_cli_by_digest_test.go
//
// What the test setups :
// - Create 2 image based on busybox using the same repository but different tags
// - Create an image based on the previous image (images_ps_filter_test2)
// - Run containers for each of those image (busybox, images_ps_filter_test1, images_ps_filter_test2)
// - Filter them out :P
func (s *DockerSuite) TestPsListContainersFilterAncestorImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Build images
	imageName1 := "images_ps_filter_test1"
	imageID1, err := buildImage(imageName1,
		`FROM busybox
		 LABEL match me 1`, true)
	c.Assert(err, check.IsNil)

	imageName1Tagged := "images_ps_filter_test1:tag"
	imageID1Tagged, err := buildImage(imageName1Tagged,
		`FROM busybox
		 LABEL match me 1 tagged`, true)
	c.Assert(err, check.IsNil)

	imageName2 := "images_ps_filter_test2"
	imageID2, err := buildImage(imageName2,
		fmt.Sprintf(`FROM %s
		 LABEL match me 2`, imageName1), true)
	c.Assert(err, check.IsNil)

	// start containers
	out, _ := dockerCmd(c, "run", "-d", "busybox", "echo", "hello")
	firstID := strings.TrimSpace(out)

	// start another container
	out, _ = dockerCmd(c, "run", "-d", "busybox", "echo", "hello")
	secondID := strings.TrimSpace(out)

	// start third container
	out, _ = dockerCmd(c, "run", "-d", imageName1, "echo", "hello")
	thirdID := strings.TrimSpace(out)

	// start fourth container
	out, _ = dockerCmd(c, "run", "-d", imageName1Tagged, "echo", "hello")
	fourthID := strings.TrimSpace(out)

	// start fifth container
	out, _ = dockerCmd(c, "run", "-d", imageName2, "echo", "hello")
	fifthID := strings.TrimSpace(out)

	var filterTestSuite = []struct {
		filterName  string
		expectedIDs []string
	}{
		// non existent stuff
		{"nonexistent", []string{}},
		{"nonexistent:tag", []string{}},
		// image
		{"busybox", []string{firstID, secondID, thirdID, fourthID, fifthID}},
		{imageName1, []string{thirdID, fifthID}},
		{imageName2, []string{fifthID}},
		// image:tag
		{fmt.Sprintf("%s:latest", imageName1), []string{thirdID, fifthID}},
		{imageName1Tagged, []string{fourthID}},
		// short-id
		{stringid.TruncateID(imageID1), []string{thirdID, fifthID}},
		{stringid.TruncateID(imageID2), []string{fifthID}},
		// full-id
		{imageID1, []string{thirdID, fifthID}},
		{imageID1Tagged, []string{fourthID}},
		{imageID2, []string{fifthID}},
	}

	for _, filter := range filterTestSuite {
		out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=ancestor="+filter.filterName)
		checkPsAncestorFilterOutput(c, out, filter.filterName, filter.expectedIDs)
	}

	// Multiple ancestor filter
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=ancestor="+imageName2, "--filter=ancestor="+imageName1Tagged)
	checkPsAncestorFilterOutput(c, out, imageName2+","+imageName1Tagged, []string{fourthID, fifthID})
}

func checkPsAncestorFilterOutput(c *check.C, out string, filterName string, expectedIDs []string) {
	actualIDs := []string{}
	if out != "" {
		actualIDs = strings.Split(out[:len(out)-1], "\n")
	}
	sort.Strings(actualIDs)
	sort.Strings(expectedIDs)

	if len(actualIDs) != len(expectedIDs) {
		c.Fatalf("Expected filtered container(s) for %s ancestor filter to be %v:%v, got %v:%v", filterName, len(expectedIDs), expectedIDs, len(actualIDs), actualIDs)
	}
	if len(expectedIDs) > 0 {
		same := true
		for i := range expectedIDs {
			if actualIDs[i] != expectedIDs[i] {
				c.Logf("%s, %s", actualIDs[i], expectedIDs[i])
				same = false
				break
			}
		}
		if !same {
			c.Fatalf("Expected filtered container(s) for %s ancestor filter to be %v, got %v", filterName, expectedIDs, actualIDs)
		}
	}
}

func (s *DockerSuite) TestPsListContainersFilterLabel(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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

	if out, _, err := dockerCmdWithError("run", "--name", "nonzero1", "busybox", "false"); err == nil {
		c.Fatal("Should fail.", out, err)
	}

	firstNonZero, err := getIDByName("nonzero1")
	if err != nil {
		c.Fatal(err)
	}

	if out, _, err := dockerCmdWithError("run", "--name", "nonzero2", "busybox", "false"); err == nil {
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
	testRequires(c, DaemonIsLinux)
	tag := "asybox:shmatest"
	dockerCmd(c, "tag", "busybox", tag)

	var id1 string
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id1 = strings.TrimSpace(string(out))

	var id2 string
	out, _ = dockerCmd(c, "run", "-d", tag, "top")
	id2 = strings.TrimSpace(string(out))

	var imageID string
	out, _ = dockerCmd(c, "inspect", "-f", "{{.Id}}", "busybox")
	imageID = strings.TrimSpace(string(out))

	var id3 string
	out, _ = dockerCmd(c, "run", "-d", imageID, "top")
	id3 = strings.TrimSpace(string(out))

	out, _ = dockerCmd(c, "ps", "--no-trunc")
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
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name=first", "-d", "busybox", "top")
	dockerCmd(c, "run", "--name=second", "--link=first:first", "-d", "busybox", "top")

	out, _ := dockerCmd(c, "ps", "--no-trunc")
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
	testRequires(c, DaemonIsLinux)
	portRange := "3800-3900"
	dockerCmd(c, "run", "-d", "--name", "porttest", "-p", portRange+":"+portRange, "busybox", "top")

	out, _ := dockerCmd(c, "ps")

	// check that the port range is in the output
	if !strings.Contains(string(out), portRange) {
		c.Fatalf("docker ps output should have had the port range %q: %s", portRange, string(out))
	}

}

func (s *DockerSuite) TestPsWithSize(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "sizetest", "busybox", "top")

	out, _ := dockerCmd(c, "ps", "--size")
	if !strings.Contains(out, "virtual") {
		c.Fatalf("docker ps with --size should show virtual size of container")
	}
}

func (s *DockerSuite) TestPsListContainersFilterCreated(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// create a container
	out, _ := dockerCmd(c, "create", "busybox")
	cID := strings.TrimSpace(out)
	shortCID := cID[:12]

	// Make sure it DOESN'T show up w/o a '-a' for normal 'ps'
	out, _ = dockerCmd(c, "ps", "-q")
	if strings.Contains(out, shortCID) {
		c.Fatalf("Should have not seen '%s' in ps output:\n%s", shortCID, out)
	}

	// Make sure it DOES show up as 'Created' for 'ps -a'
	out, _ = dockerCmd(c, "ps", "-a")

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
	out, _ = dockerCmd(c, "ps", "-q", "-f", "status=created")
	containerOut := strings.TrimSpace(out)
	if !strings.HasPrefix(cID, containerOut) {
		c.Fatalf("Expected id %s, got %s for filter, out: %s", cID, containerOut, out)
	}
}

func (s *DockerSuite) TestPsFormatMultiNames(c *check.C) {
	testRequires(c, DaemonIsLinux)
	//create 2 containers and link them
	dockerCmd(c, "run", "--name=child", "-d", "busybox", "top")
	dockerCmd(c, "run", "--name=parent", "--link=child:linkedone", "-d", "busybox", "top")

	//use the new format capabilities to only list the names and --no-trunc to get all names
	out, _ := dockerCmd(c, "ps", "--format", "{{.Names}}", "--no-trunc")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	expected := []string{"parent", "child,parent/linkedone"}
	var names []string
	for _, l := range lines {
		names = append(names, l)
	}
	if !reflect.DeepEqual(expected, names) {
		c.Fatalf("Expected array with non-truncated names: %v, got: %v", expected, names)
	}

	//now list without turning off truncation and make sure we only get the non-link names
	out, _ = dockerCmd(c, "ps", "--format", "{{.Names}}")
	lines = strings.Split(strings.TrimSpace(string(out)), "\n")
	expected = []string{"parent", "child"}
	var truncNames []string
	for _, l := range lines {
		truncNames = append(truncNames, l)
	}
	if !reflect.DeepEqual(expected, truncNames) {
		c.Fatalf("Expected array with truncated names: %v, got: %v", expected, truncNames)
	}

}

func (s *DockerSuite) TestPsFormatHeaders(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// make sure no-container "docker ps" still prints the header row
	out, _ := dockerCmd(c, "ps", "--format", "table {{.ID}}")
	if out != "CONTAINER ID\n" {
		c.Fatalf(`Expected 'CONTAINER ID\n', got %v`, out)
	}

	// verify that "docker ps" with a container still prints the header row also
	dockerCmd(c, "run", "--name=test", "-d", "busybox", "top")
	out, _ = dockerCmd(c, "ps", "--format", "table {{.Names}}")
	if out != "NAMES\ntest\n" {
		c.Fatalf(`Expected 'NAMES\ntest\n', got %v`, out)
	}
}

func (s *DockerSuite) TestPsDefaultFormatAndQuiet(c *check.C) {
	testRequires(c, DaemonIsLinux)
	config := `{
		"psFormat": "{{ .ID }} default"
}`
	d, err := ioutil.TempDir("", "integration-cli-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(d)

	err = ioutil.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0644)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "run", "--name=test", "-d", "busybox", "top")
	id := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "--config", d, "ps", "-q")
	if !strings.HasPrefix(id, strings.TrimSpace(out)) {
		c.Fatalf("Expected to print only the container id, got %v\n", out)
	}
}

// Test for GitHub issue #12595
func (s *DockerSuite) TestPsImageIDAfterUpdate(c *check.C) {
	testRequires(c, DaemonIsLinux)

	originalImageName := "busybox:TestPsImageIDAfterUpdate-original"
	updatedImageName := "busybox:TestPsImageIDAfterUpdate-updated"

	runCmd := exec.Command(dockerBinary, "tag", "busybox:latest", originalImageName)
	out, _, err := runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)

	originalImageID, err := getIDByName(originalImageName)
	c.Assert(err, check.IsNil)

	runCmd = exec.Command(dockerBinary, "run", "-d", originalImageName, "top")
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	containerID := strings.TrimSpace(out)

	linesOut, err := exec.Command(dockerBinary, "ps", "--no-trunc").CombinedOutput()
	c.Assert(err, check.IsNil)

	lines := strings.Split(strings.TrimSpace(string(linesOut)), "\n")
	// skip header
	lines = lines[1:]
	c.Assert(len(lines), check.Equals, 1)

	for _, line := range lines {
		f := strings.Fields(line)
		c.Assert(f[1], check.Equals, originalImageName)
	}

	runCmd = exec.Command(dockerBinary, "commit", containerID, updatedImageName)
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)

	runCmd = exec.Command(dockerBinary, "tag", "-f", updatedImageName, originalImageName)
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)

	linesOut, err = exec.Command(dockerBinary, "ps", "--no-trunc").CombinedOutput()
	c.Assert(err, check.IsNil)

	lines = strings.Split(strings.TrimSpace(string(linesOut)), "\n")
	// skip header
	lines = lines[1:]
	c.Assert(len(lines), check.Equals, 1)

	for _, line := range lines {
		f := strings.Fields(line)
		c.Assert(f[1], check.Equals, originalImageID)
	}

}
