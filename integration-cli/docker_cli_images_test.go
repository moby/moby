package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestImagesEnsureImageIsListed(c *check.C) {
	imagesCmd := exec.Command(dockerBinary, "images")
	out, _, err := runCommandWithOutput(imagesCmd)
	if err != nil {
		c.Fatalf("listing images failed with errors: %s, %v", out, err)
	}

	if !strings.Contains(out, "busybox") {
		c.Fatal("images should've listed busybox")
	}

}

func (s *DockerSuite) TestImagesOrderedByCreationDate(c *check.C) {
	id1, err := buildImage("order:test_a",
		`FROM scratch
		MAINTAINER dockerio1`, true)
	if err != nil {
		c.Fatal(err)
	}
	time.Sleep(time.Second)
	id2, err := buildImage("order:test_c",
		`FROM scratch
		MAINTAINER dockerio2`, true)
	if err != nil {
		c.Fatal(err)
	}
	time.Sleep(time.Second)
	id3, err := buildImage("order:test_b",
		`FROM scratch
		MAINTAINER dockerio3`, true)
	if err != nil {
		c.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "images", "-q", "--no-trunc"))
	if err != nil {
		c.Fatalf("listing images failed with errors: %s, %v", out, err)
	}
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
	imagesCmd := exec.Command(dockerBinary, "images", "-f", "FOO=123")
	out, _, err := runCommandWithOutput(imagesCmd)
	if !strings.Contains(out, "Invalid filter") {
		c.Fatalf("error should occur when listing images with invalid filter name FOO, %s, %v", out, err)
	}

}

func (s *DockerSuite) TestImagesFilterLabel(c *check.C) {
	imageName1 := "images_filter_test1"
	imageName2 := "images_filter_test2"
	imageName3 := "images_filter_test3"
	image1ID, err := buildImage(imageName1,
		`FROM scratch
		 LABEL match me`, true)
	if err != nil {
		c.Fatal(err)
	}

	image2ID, err := buildImage(imageName2,
		`FROM scratch
		 LABEL match="me too"`, true)
	if err != nil {
		c.Fatal(err)
	}

	image3ID, err := buildImage(imageName3,
		`FROM scratch
		 LABEL nomatch me`, true)
	if err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "images", "--no-trunc", "-q", "-f", "label=match")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
	out = strings.TrimSpace(out)

	if (!strings.Contains(out, image1ID) && !strings.Contains(out, image2ID)) || strings.Contains(out, image3ID) {
		c.Fatalf("Expected ids %s,%s got %s", image1ID, image2ID, out)
	}

	cmd = exec.Command(dockerBinary, "images", "--no-trunc", "-q", "-f", "label=match=me too")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
	out = strings.TrimSpace(out)

	if out != image2ID {
		c.Fatalf("Expected %s got %s", image2ID, out)
	}

}

func (s *DockerSuite) TestImagesFilterSpaceTrimCase(c *check.C) {
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
		cmd := exec.Command(dockerBinary, "images", "-q", "-f", filter)
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			c.Fatal(err)
		}
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

	// create container 1
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("error running busybox: %s, %v", out, err)
	}
	containerId1 := strings.TrimSpace(out)

	// tag as foobox
	cmd = exec.Command(dockerBinary, "commit", containerId1, "foobox")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("error tagging foobox: %s", err)
	}
	imageId := stringid.TruncateID(strings.TrimSpace(out))

	// overwrite the tag, making the previous image dangling
	cmd = exec.Command(dockerBinary, "tag", "-f", "busybox", "foobox")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("error tagging foobox: %s", err)
	}

	cmd = exec.Command(dockerBinary, "images", "-q", "-f", "dangling=true")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("listing images failed with errors: %s, %v", out, err)
	}

	if e, a := 1, strings.Count(out, imageId); e != a {
		c.Fatalf("expected 1 dangling image, got %d: %s", a, out)
	}

}
