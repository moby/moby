package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestImagesEnsureImageIsListed(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "images")
	if !strings.Contains(out, "busybox") {
		c.Fatal("images should've listed busybox")
	}
}

func (s *DockerSuite) TestImagesEnsureImageWithTagIsListed(c *check.C) {
	testRequires(c, DaemonIsLinux)
	_, err := buildImage("imagewithtag:v1",
		`FROM scratch
		MAINTAINER dockerio1`, true)
	c.Assert(err, check.IsNil)

	_, err = buildImage("imagewithtag:v2",
		`FROM scratch
		MAINTAINER dockerio1`, true)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "images", "imagewithtag:v1")

	if !strings.Contains(out, "imagewithtag") || !strings.Contains(out, "v1") || strings.Contains(out, "v2") {
		c.Fatal("images should've listed imagewithtag:v1 and not imagewithtag:v2")
	}

	out, _ = dockerCmd(c, "images", "imagewithtag")

	if !strings.Contains(out, "imagewithtag") || !strings.Contains(out, "v1") || !strings.Contains(out, "v2") {
		c.Fatal("images should've listed imagewithtag:v1 and imagewithtag:v2")
	}
}

func (s *DockerSuite) TestImagesEnsureImageWithBadTagIsNotListed(c *check.C) {
	out, _ := dockerCmd(c, "images", "busybox:nonexistent")

	if strings.Contains(out, "busybox") {
		c.Fatal("images should not have listed busybox")
	}
}

func (s *DockerSuite) TestImagesOrderedByCreationDate(c *check.C) {
	testRequires(c, DaemonIsLinux)
	id1, err := buildImage("order:test_a",
		`FROM scratch
		MAINTAINER dockerio1`, true)
	if err != nil {
		c.Fatal(err)
	}
	time.Sleep(1 * time.Second)
	id2, err := buildImage("order:test_c",
		`FROM scratch
		MAINTAINER dockerio2`, true)
	if err != nil {
		c.Fatal(err)
	}
	time.Sleep(1 * time.Second)
	id3, err := buildImage("order:test_b",
		`FROM scratch
		MAINTAINER dockerio3`, true)
	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc")
	imgs := strings.Split(out, "\n")
	if imgs[0] != id3 {
		c.Fatalf("First image must be %s, got %s", id3, imgs[0])
	}
	if imgs[1] != id2 {
		c.Fatalf("Second image must be %s, got %s", id2, imgs[1])
	}
	if imgs[2] != id1 {
		c.Fatalf("Third image must be %s, got %s", id1, imgs[2])
	}
}

func (s *DockerSuite) TestImagesErrorWithInvalidFilterNameTest(c *check.C) {
	out, _, err := dockerCmdWithError("images", "-f", "FOO=123")
	if err == nil || !strings.Contains(out, "Invalid filter") {
		c.Fatalf("error should occur when listing images with invalid filter name FOO, %s", out)
	}
}

func (s *DockerSuite) TestImagesFilterLabel(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imageName1 := "images_filter_test1"
	imageName2 := "images_filter_test2"
	imageName3 := "images_filter_test3"
	image1ID, err := buildImage(imageName1,
		`FROM scratch
		 LABEL match me`, true)
	c.Assert(err, check.IsNil)

	image2ID, err := buildImage(imageName2,
		`FROM scratch
		 LABEL match="me too"`, true)
	c.Assert(err, check.IsNil)

	image3ID, err := buildImage(imageName3,
		`FROM scratch
		 LABEL nomatch me`, true)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match")
	out = strings.TrimSpace(out)
	c.Assert(out, check.Matches, fmt.Sprintf("[\\s\\w]*%s[\\s\\w]*", image1ID))
	c.Assert(out, check.Matches, fmt.Sprintf("[\\s\\w]*%s[\\s\\w]*", image2ID))
	c.Assert(out, check.Not(check.Matches), fmt.Sprintf("[\\s\\w]*%s[\\s\\w]*", image3ID))

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

func (s *DockerSuite) TestImagesFilterSpaceTrimCase(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imageName := "images_filter_test"
	buildImage(imageName,
		`FROM scratch
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
				fmt.Printf("out %d", idx)
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
	dockerCmd(c, "tag", "-f", "busybox", "foobox")

	out, _ = dockerCmd(c, "images", "-q", "-f", "dangling=true")
	if e, a := 1, strings.Count(out, imageID); e != a {
		c.Fatalf("expected 1 dangling image, got %d: %s", a, out)
	}
}

func (s *DockerSuite) TestImagesWithIncorrectFilter(c *check.C) {
	out, _, err := dockerCmdWithError("images", "-f", "dangling=invalid")
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestImagesEnsureOnlyHeadsImagesShown(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerfile := `
        FROM scratch
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
	if strings.Contains(out, intermediate) {
		c.Fatalf("images shouldn't show non-heads images, got %s in %s", intermediate, out)
	}
	if !strings.Contains(out, head[:12]) {
		c.Fatalf("images should contain final built images, want %s in out, got %s", head[:12], out)
	}
}

func (s *DockerSuite) TestImagesEnsureImagesFromScratchShown(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerfile := `
        FROM scratch
        MAINTAINER docker`

	id, _, err := buildImageWithOut("scratch-image", dockerfile, false)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "images")
	if !strings.Contains(out, id[:12]) {
		c.Fatalf("images should contain images built from scratch (e.g. %s), got %s", id[:12], out)
	}
}
