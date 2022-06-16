package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/pkg/stringid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

type DockerCLIImagesSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIImagesSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLIImagesSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImageIsListed(c *testing.T) {
	imagesOut, _ := dockerCmd(c, "images")
	assert.Assert(c, strings.Contains(imagesOut, "busybox"))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImageWithTagIsListed(c *testing.T) {
	name := "imagewithtag"
	dockerCmd(c, "tag", "busybox", name+":v1")
	dockerCmd(c, "tag", "busybox", name+":v1v1")
	dockerCmd(c, "tag", "busybox", name+":v2")

	imagesOut, _ := dockerCmd(c, "images", name+":v1")
	assert.Assert(c, strings.Contains(imagesOut, name))
	assert.Assert(c, strings.Contains(imagesOut, "v1"))
	assert.Assert(c, !strings.Contains(imagesOut, "v2"))
	assert.Assert(c, !strings.Contains(imagesOut, "v1v1"))
	imagesOut, _ = dockerCmd(c, "images", name)
	assert.Assert(c, strings.Contains(imagesOut, name))
	assert.Assert(c, strings.Contains(imagesOut, "v1"))
	assert.Assert(c, strings.Contains(imagesOut, "v1v1"))
	assert.Assert(c, strings.Contains(imagesOut, "v2"))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImageWithBadTagIsNotListed(c *testing.T) {
	imagesOut, _ := dockerCmd(c, "images", "busybox:nonexistent")
	assert.Assert(c, !strings.Contains(imagesOut, "busybox"))
}

func (s *DockerCLIImagesSuite) TestImagesOrderedByCreationDate(c *testing.T) {
	buildImageSuccessfully(c, "order:test_a", build.WithDockerfile(`FROM busybox
                MAINTAINER dockerio1`))
	id1 := getIDByName(c, "order:test_a")
	time.Sleep(1 * time.Second)
	buildImageSuccessfully(c, "order:test_c", build.WithDockerfile(`FROM busybox
                MAINTAINER dockerio2`))
	id2 := getIDByName(c, "order:test_c")
	time.Sleep(1 * time.Second)
	buildImageSuccessfully(c, "order:test_b", build.WithDockerfile(`FROM busybox
                MAINTAINER dockerio3`))
	id3 := getIDByName(c, "order:test_b")

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc")
	imgs := strings.Split(out, "\n")
	assert.Equal(c, imgs[0], id3, fmt.Sprintf("First image must be %s, got %s", id3, imgs[0]))
	assert.Equal(c, imgs[1], id2, fmt.Sprintf("First image must be %s, got %s", id2, imgs[1]))
	assert.Equal(c, imgs[2], id1, fmt.Sprintf("First image must be %s, got %s", id1, imgs[2]))
}

func (s *DockerCLIImagesSuite) TestImagesErrorWithInvalidFilterNameTest(c *testing.T) {
	out, _, err := dockerCmdWithError("images", "-f", "FOO=123")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "invalid filter"))
}

func (s *DockerCLIImagesSuite) TestImagesFilterLabelMatch(c *testing.T) {
	imageName1 := "images_filter_test1"
	imageName2 := "images_filter_test2"
	imageName3 := "images_filter_test3"
	buildImageSuccessfully(c, imageName1, build.WithDockerfile(`FROM busybox
                 LABEL match me`))
	image1ID := getIDByName(c, imageName1)

	buildImageSuccessfully(c, imageName2, build.WithDockerfile(`FROM busybox
                 LABEL match="me too"`))
	image2ID := getIDByName(c, imageName2)

	buildImageSuccessfully(c, imageName3, build.WithDockerfile(`FROM busybox
                 LABEL nomatch me`))
	image3ID := getIDByName(c, imageName3)

	out, _ := dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match")
	out = strings.TrimSpace(out)
	assert.Assert(c, is.Regexp(fmt.Sprintf("^[\\s\\w:]*%s[\\s\\w:]*$", image1ID), out))

	assert.Assert(c, is.Regexp(fmt.Sprintf("^[\\s\\w:]*%s[\\s\\w:]*$", image2ID), out))

	assert.Assert(c, !is.Regexp(fmt.Sprintf("^[\\s\\w:]*%s[\\s\\w:]*$", image3ID), out)().Success())

	out, _ = dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match=me too")
	out = strings.TrimSpace(out)
	assert.Equal(c, out, image2ID)
}

// Regression : #15659
func (s *DockerCLIImagesSuite) TestCommitWithFilterLabel(c *testing.T) {
	// Create a container
	dockerCmd(c, "run", "--name", "bar", "busybox", "/bin/sh")
	// Commit with labels "using changes"
	out, _ := dockerCmd(c, "commit", "-c", "LABEL foo.version=1.0.0-1", "-c", "LABEL foo.name=bar", "-c", "LABEL foo.author=starlord", "bar", "bar:1.0.0-1")
	imageID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=foo.version=1.0.0-1")
	out = strings.TrimSpace(out)
	assert.Equal(c, out, imageID)
}

func (s *DockerCLIImagesSuite) TestImagesFilterSinceAndBefore(c *testing.T) {
	buildImageSuccessfully(c, "image:1", build.WithDockerfile(`FROM `+minimalBaseImage()+`
LABEL number=1`))
	imageID1 := getIDByName(c, "image:1")
	buildImageSuccessfully(c, "image:2", build.WithDockerfile(`FROM `+minimalBaseImage()+`
LABEL number=2`))
	imageID2 := getIDByName(c, "image:2")
	buildImageSuccessfully(c, "image:3", build.WithDockerfile(`FROM `+minimalBaseImage()+`
LABEL number=3`))
	imageID3 := getIDByName(c, "image:3")

	expected := []string{imageID3, imageID2}

	out, _ := dockerCmd(c, "images", "-f", "since=image:1", "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "since="+imageID1, "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	expected = []string{imageID3}

	out, _ = dockerCmd(c, "images", "-f", "since=image:2", "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "since="+imageID2, "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("SINCE filter: Image list is not in the correct order: %v\n%s", expected, out))

	expected = []string{imageID2, imageID1}

	out, _ = dockerCmd(c, "images", "-f", "before=image:3", "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "before="+imageID3, "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))

	expected = []string{imageID1}

	out, _ = dockerCmd(c, "images", "-f", "before=image:2", "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))

	out, _ = dockerCmd(c, "images", "-f", "before="+imageID2, "image")
	assert.Equal(c, assertImageList(out, expected), true, fmt.Sprintf("BEFORE filter: Image list is not in the correct order: %v\n%s", expected, out))
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

// FIXME(vdemeester) should be a unit test on `docker image ls`
func (s *DockerCLIImagesSuite) TestImagesFilterSpaceTrimCase(c *testing.T) {
	imageName := "images_filter_test"
	// Build a image and fail to build so that we have dangling images ?
	buildImage(imageName, build.WithDockerfile(`FROM busybox
                 RUN touch /test/foo
                 RUN touch /test/bar
                 RUN touch /test/baz`)).Assert(c, icmd.Expected{
		ExitCode: 1,
	})

	filters := []string{
		"dangling=true",
		"Dangling=true",
		" dangling=true",
		"dangling=true ",
		"dangling = true",
	}

	imageListings := make([][]string, 5)
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

func (s *DockerCLIImagesSuite) TestImagesEnsureDanglingImageOnlyListedOnce(c *testing.T) {
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
	assert.Equal(c, strings.Count(out, imageID), 1)

	out, _ = dockerCmd(c, "images", "-q", "-f", "dangling=false")
	// dangling=false would not include dangling images
	assert.Assert(c, !strings.Contains(out, imageID))
	out, _ = dockerCmd(c, "images")
	// docker images still include dangling images
	assert.Assert(c, strings.Contains(out, imageID))
}

// FIXME(vdemeester) should be a unit test for `docker image ls`
func (s *DockerCLIImagesSuite) TestImagesWithIncorrectFilter(c *testing.T) {
	out, _, err := dockerCmdWithError("images", "-f", "dangling=invalid")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "invalid filter"))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureOnlyHeadsImagesShown(c *testing.T) {
	dockerfile := `
        FROM busybox
        MAINTAINER docker
        ENV foo bar`
	name := "scratch-image"
	result := buildImage(name, build.WithDockerfile(dockerfile))
	result.Assert(c, icmd.Success)
	id := getIDByName(c, name)

	// this is just the output of docker build
	// we're interested in getting the image id of the MAINTAINER instruction
	// and that's located at output, line 5, from 7 to end
	split := strings.Split(result.Combined(), "\n")
	intermediate := strings.TrimSpace(split[5][7:])

	out, _ := dockerCmd(c, "images")
	// images shouldn't show non-heads images
	assert.Assert(c, !strings.Contains(out, intermediate))
	// images should contain final built images
	assert.Assert(c, strings.Contains(out, stringid.TruncateID(id)))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImagesFromScratchShown(c *testing.T) {
	testRequires(c, DaemonIsLinux) // Windows does not support FROM scratch
	dockerfile := `
        FROM scratch
        MAINTAINER docker`

	name := "scratch-image"
	buildImageSuccessfully(c, name, build.WithDockerfile(dockerfile))
	id := getIDByName(c, name)

	out, _ := dockerCmd(c, "images")
	// images should contain images built from scratch
	assert.Assert(c, strings.Contains(out, stringid.TruncateID(id)))
}

// For W2W - equivalent to TestImagesEnsureImagesFromScratchShown but Windows
// doesn't support from scratch
func (s *DockerCLIImagesSuite) TestImagesEnsureImagesFromBusyboxShown(c *testing.T) {
	dockerfile := `
        FROM busybox
        MAINTAINER docker`
	name := "busybox-image"

	buildImageSuccessfully(c, name, build.WithDockerfile(dockerfile))
	id := getIDByName(c, name)

	out, _ := dockerCmd(c, "images")
	// images should contain images built from busybox
	assert.Assert(c, strings.Contains(out, stringid.TruncateID(id)))
}

// #18181
func (s *DockerCLIImagesSuite) TestImagesFilterNameWithPort(c *testing.T) {
	tag := "a.b.c.d:5000/hello"
	dockerCmd(c, "tag", "busybox", tag)
	out, _ := dockerCmd(c, "images", tag)
	assert.Assert(c, strings.Contains(out, tag))
	out, _ = dockerCmd(c, "images", tag+":latest")
	assert.Assert(c, strings.Contains(out, tag))
	out, _ = dockerCmd(c, "images", tag+":no-such-tag")
	assert.Assert(c, !strings.Contains(out, tag))
}

func (s *DockerCLIImagesSuite) TestImagesFormat(c *testing.T) {
	// testRequires(c, DaemonIsLinux)
	tag := "myimage"
	dockerCmd(c, "tag", "busybox", tag+":v1")
	dockerCmd(c, "tag", "busybox", tag+":v2")

	out, _ := dockerCmd(c, "images", "--format", "{{.Repository}}", tag)
	lines := strings.Split(strings.TrimSpace(out), "\n")

	expected := []string{"myimage", "myimage"}
	var names []string
	names = append(names, lines...)
	assert.Assert(c, is.DeepEqual(names, expected), "Expected array with truncated names: %v, got: %v", expected, names)
}

// ImagesDefaultFormatAndQuiet
func (s *DockerCLIImagesSuite) TestImagesFormatDefaultFormat(c *testing.T) {
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
	d, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(d)

	err = os.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0644)
	assert.NilError(c, err)

	out, _ = dockerCmd(c, "--config", d, "images", "-q", "myimage")
	assert.Equal(c, out, imageID+"\n", "Expected to print only the image id, got %v\n", out)
}
