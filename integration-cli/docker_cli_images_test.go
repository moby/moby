package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestImagesEnsureImageIsListed(c *check.C) {
	imagesOut, _ := dockerCmd(c, "images")
	c.Assert(imagesOut, checker.Contains, "busybox")
}

func (s *DockerSuite) TestImagesEnsureImageWithTagIsListed(c *check.C) {
	name := "imagewithtag"
	dockerCmd(c, "tag", "busybox", name+":v1")
	dockerCmd(c, "tag", "busybox", name+":v1v1")
	dockerCmd(c, "tag", "busybox", name+":v2")

	imagesOut, _ := dockerCmd(c, "images", name+":v1")
	c.Assert(imagesOut, checker.Contains, name)
	c.Assert(imagesOut, checker.Contains, "v1")
	c.Assert(imagesOut, checker.Not(checker.Contains), "v2")
	c.Assert(imagesOut, checker.Not(checker.Contains), "v1v1")

	imagesOut, _ = dockerCmd(c, "images", name)
	c.Assert(imagesOut, checker.Contains, name)
	c.Assert(imagesOut, checker.Contains, "v1")
	c.Assert(imagesOut, checker.Contains, "v1v1")
	c.Assert(imagesOut, checker.Contains, "v2")
}

func (s *DockerSuite) TestImagesEnsureImageWithBadTagIsNotListed(c *check.C) {
	imagesOut, _ := dockerCmd(c, "images", "busybox:nonexistent")
	c.Assert(imagesOut, checker.Not(checker.Contains), "busybox")
}

func (s *DockerSuite) TestImagesOrderedByCreationDate(c *check.C) {
	id1, err := buildImage("order:test_a",
		`FROM busybox
                MAINTAINER dockerio1`, true)
	c.Assert(err, checker.IsNil)
	time.Sleep(1 * time.Second)
	id2, err := buildImage("order:test_c",
		`FROM busybox
                MAINTAINER dockerio2`, true)
	c.Assert(err, checker.IsNil)
	time.Sleep(1 * time.Second)
	id3, err := buildImage("order:test_b",
		`FROM busybox
                MAINTAINER dockerio3`, true)
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc")
	imgs := strings.Split(out, "\n")
	c.Assert(imgs[0], checker.Equals, id3, check.Commentf("First image must be %s, got %s", id3, imgs[0]))
	c.Assert(imgs[1], checker.Equals, id2, check.Commentf("First image must be %s, got %s", id2, imgs[1]))
	c.Assert(imgs[2], checker.Equals, id1, check.Commentf("First image must be %s, got %s", id1, imgs[2]))
}

func (s *DockerSuite) TestImagesErrorWithInvalidFilterNameTest(c *check.C) {
	out, _, err := dockerCmdWithError("images", "-f", "FOO=123")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestImagesFilterLabelMatch(c *check.C) {
	imageName1 := "images_filter_test1"
	imageName2 := "images_filter_test2"
	imageName3 := "images_filter_test3"
	image1ID, err := buildImage(imageName1,
		`FROM busybox
                 LABEL match me`, true)
	c.Assert(err, check.IsNil)

	image2ID, err := buildImage(imageName2,
		`FROM busybox
                 LABEL match="me too"`, true)
	c.Assert(err, check.IsNil)

	image3ID, err := buildImage(imageName3,
		`FROM busybox
                 LABEL nomatch me`, true)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match")
	out = strings.TrimSpace(out)
	c.Assert(out, check.Matches, fmt.Sprintf("[\\s\\w:]*%s[\\s\\w:]*", image1ID))
	c.Assert(out, check.Matches, fmt.Sprintf("[\\s\\w:]*%s[\\s\\w:]*", image2ID))
	c.Assert(out, check.Not(check.Matches), fmt.Sprintf("[\\s\\w:]*%s[\\s\\w:]*", image3ID))

	out, _ = dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match=me too")
	out = strings.TrimSpace(out)
	c.Assert(out, check.Equals, image2ID)
}

// Regression : #15659
func (s *DockerSuite) TestImagesFilterLabelWithCommit(c *check.C) {
	// Create a container
	dockerCmd(c, "run", "--name", "bar", "busybox", "/bin/sh")
	// Commit with labels "using changes"
	out, _ := dockerCmd(c, "commit", "-c", "LABEL foo.version=1.0.0-1", "-c", "LABEL foo.name=bar", "-c", "LABEL foo.author=starlord", "bar", "bar:1.0.0-1")
	imageID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=foo.version=1.0.0-1")
	out = strings.TrimSpace(out)
	c.Assert(out, check.Equals, imageID)
}

func (s *DockerSuite) TestImagesFilterSinceAndBefore(c *check.C) {
	imageID1, err := buildImage("image:1", `FROM `+minimalBaseImage()+`
LABEL number=1`, true)
	c.Assert(err, checker.IsNil)
	imageID2, err := buildImage("image:2", `FROM `+minimalBaseImage()+`
LABEL number=2`, true)
	c.Assert(err, checker.IsNil)
	imageID3, err := buildImage("image:3", `FROM `+minimalBaseImage()+`
LABEL number=3`, true)
	c.Assert(err, checker.IsNil)

	expected := []string{imageID3, imageID2}

	out, _ := dockerCmd(c, "images", "-f", "since=image:1", "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "since="+imageID1, "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	expected = []string{imageID3}

	out, _ = dockerCmd(c, "images", "-f", "since=image:2", "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "since="+imageID2, "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	expected = []string{imageID2, imageID1}

	out, _ = dockerCmd(c, "images", "-f", "before=image:3", "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "before="+imageID3, "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))

	expected = []string{imageID1}

	out, _ = dockerCmd(c, "images", "-f", "before=image:2", "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "before="+imageID2, "image")
	c.Assert(assertImageList(out, expected), checker.Equals, true, check.Commentf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))
}

func assertImageList(out string, expected []string) bool {
	lines := strings.Split(strings.Trim(out, "\n "), "\n")

	if len(lines)-1 != len(expected) {
		return false
	}

	imageIDIndex := strings.Index(lines[0], "IMAGE ID")
	for i := 0; i < len(expected); i++ {
		imageID := lines[i+1][imageIDIndex : imageIDIndex+12]
		found := false
		for _, e := range expected {
			if imageID == e[7:19] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (s *DockerSuite) TestImagesFilterSpaceTrimCase(c *check.C) {
	imageName := "images_filter_test"
	buildImage(imageName,
		`FROM busybox
                 RUN touch /test/foo
                 RUN touch /test/bar
                 RUN touch /test/baz`, true)

	filters := []string{
		"dangling=true",
		"Dangling=true",
		" dangling=true",
		"dangling=true ",
		"dangling = true",
	}

	imageListings := make([][]string, 5, 5)
	for idx, filter := range filters {
		out, _ := dockerCmd(c, "images", "-q", "-f", filter)
		listing := strings.Split(out, "\n")
		sort.Strings(listing)
		imageListings[idx] = listing
	}

	for idx, listing := range imageListings {
		if idx < 4 && !reflect.DeepEqual(listing, imageListings[idx+1]) {
			for idx, errListing := range imageListings {
				fmt.Printf("out %d\n", idx)
				for _, image := range errListing {
					fmt.Print(image)
				}
				fmt.Print("")
			}
			c.Fatalf("All output must be the same")
		}
	}
}

func (s *DockerSuite) TestImagesEnsureDanglingImageOnlyListedOnce(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// create container 1
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	containerID1 := strings.TrimSpace(out)

	// tag as foobox
	out, _ = dockerCmd(c, "commit", containerID1, "foobox")
	imageID := stringid.TruncateID(strings.TrimSpace(out))

	// overwrite the tag, making the previous image dangling
	dockerCmd(c, "tag", "busybox", "foobox")

	out, _ = dockerCmd(c, "images", "-q", "-f", "dangling=true")
	// Expect one dangling image
	c.Assert(strings.Count(out, imageID), checker.Equals, 1)

	out, _ = dockerCmd(c, "images", "-q", "-f", "dangling=false")
	//dangling=false would not include dangling images
	c.Assert(out, checker.Not(checker.Contains), imageID)

	out, _ = dockerCmd(c, "images")
	//docker images still include dangling images
	c.Assert(out, checker.Contains, imageID)

}

func (s *DockerSuite) TestImagesWithIncorrectFilter(c *check.C) {
	out, _, err := dockerCmdWithError("images", "-f", "dangling=invalid")
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestImagesEnsureOnlyHeadsImagesShown(c *check.C) {
	dockerfile := `
        FROM busybox
        MAINTAINER docker
        ENV foo bar`

	head, out, err := buildImageWithOut("scratch-image", dockerfile, false)
	c.Assert(err, check.IsNil)

	// this is just the output of docker build
	// we're interested in getting the image id of the MAINTAINER instruction
	// and that's located at output, line 5, from 7 to end
	split := strings.Split(out, "\n")
	intermediate := strings.TrimSpace(split[5][7:])

	out, _ = dockerCmd(c, "images")
	// images shouldn't show non-heads images
	c.Assert(out, checker.Not(checker.Contains), intermediate)
	// images should contain final built images
	c.Assert(out, checker.Contains, stringid.TruncateID(head))
}

func (s *DockerSuite) TestImagesEnsureImagesFromScratchShown(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support FROM scratch

	dockerfile := `
        FROM scratch
        MAINTAINER docker`

	id, _, err := buildImageWithOut("scratch-image", dockerfile, false)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "images")
	// images should contain images built from scratch
	c.Assert(out, checker.Contains, stringid.TruncateID(id))
}

// For W2W - equivalent to TestImagesEnsureImagesFromScratchShown but Windows
// doesn't support from scratch
func (s *DockerSuite) TestImagesEnsureImagesFromBusyboxShown(c *check.C) {
	dockerfile := `
        FROM busybox
        MAINTAINER docker`

	id, _, err := buildImageWithOut("busybox-image", dockerfile, false)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "images")
	// images should contain images built from busybox
	c.Assert(out, checker.Contains, stringid.TruncateID(id))
}

// #18181
func (s *DockerSuite) TestImagesFilterNameWithPort(c *check.C) {
	tag := "a.b.c.d:5000/hello"
	dockerCmd(c, "tag", "busybox", tag)
	out, _ := dockerCmd(c, "images", tag)
	c.Assert(out, checker.Contains, tag)

	out, _ = dockerCmd(c, "images", tag+":latest")
	c.Assert(out, checker.Contains, tag)

	out, _ = dockerCmd(c, "images", tag+":no-such-tag")
	c.Assert(out, checker.Not(checker.Contains), tag)
}

func (s *DockerSuite) TestImagesFormat(c *check.C) {
	// testRequires(c, DaemonIsLinux)
	tag := "myimage"
	dockerCmd(c, "tag", "busybox", tag+":v1")
	dockerCmd(c, "tag", "busybox", tag+":v2")

	out, _ := dockerCmd(c, "images", "--format", "{{.Repository}}", tag)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	expected := []string{"myimage", "myimage"}
	var names []string
	names = append(names, lines...)
	c.Assert(expected, checker.DeepEquals, names, check.Commentf("Expected array with truncated names: %v, got: %v", expected, names))
}

// ImagesDefaultFormatAndQuiet
func (s *DockerSuite) TestImagesFormatDefaultFormat(c *check.C) {
	testRequires(c, DaemonIsLinux)

	// create container 1
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	containerID1 := strings.TrimSpace(out)

	// tag as foobox
	out, _ = dockerCmd(c, "commit", containerID1, "myimage")
	imageID := stringid.TruncateID(strings.TrimSpace(out))

	config := `{
		"imagesFormat": "{{ .ID }} default"
}`
	d, err := ioutil.TempDir("", "integration-cli-")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(d)

	err = ioutil.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0644)
	c.Assert(err, checker.IsNil)

	out, _ = dockerCmd(c, "--config", d, "images", "-q", "myimage")
	c.Assert(out, checker.Equals, imageID+"\n", check.Commentf("Expected to print only the image id, got %v\n", out))
}
