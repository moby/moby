package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client/pkg/stringid"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/integration-cli/cli/build"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

type DockerCLIImagesSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIImagesSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLIImagesSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImageIsListed(c *testing.T) {
	imagesOut := cli.DockerCmd(c, "images").Stdout()
	assert.Assert(c, is.Contains(imagesOut, "busybox"))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImageWithTagIsListed(c *testing.T) {
	const name = "imagewithtag"
	cli.DockerCmd(c, "tag", "busybox", name+":v1")
	cli.DockerCmd(c, "tag", "busybox", name+":v1v1")
	cli.DockerCmd(c, "tag", "busybox", name+":v2")

	imagesOut := cli.DockerCmd(c, "images", name+":v1").Stdout()
	assert.Assert(c, is.Contains(imagesOut, name))
	assert.Assert(c, is.Contains(imagesOut, "v1"))
	assert.Assert(c, !strings.Contains(imagesOut, "v2"))
	assert.Assert(c, !strings.Contains(imagesOut, "v1v1"))
	imagesOut = cli.DockerCmd(c, "images", name).Stdout()
	assert.Assert(c, is.Contains(imagesOut, name))
	assert.Assert(c, is.Contains(imagesOut, "v1"))
	assert.Assert(c, is.Contains(imagesOut, "v1v1"))
	assert.Assert(c, is.Contains(imagesOut, "v2"))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImageWithBadTagIsNotListed(c *testing.T) {
	imagesOut := cli.DockerCmd(c, "images", "busybox:nonexistent").Stdout()
	assert.Assert(c, !strings.Contains(imagesOut, "busybox"))
}

func (s *DockerCLIImagesSuite) TestImagesOrderedByCreationDate(c *testing.T) {
	cli.BuildCmd(c, "order:test_a", build.WithDockerfile("FROM busybox\nRUN echo a > /result.txt\n"))
	imageID1 := getIDByName(c, "order:test_a")

	time.Sleep(1 * time.Second) // need some delay to make sure images sort predictable
	cli.BuildCmd(c, "order:test_b", build.WithDockerfile("FROM busybox\nRUN echo bb > /result.txt\n"))
	imageID2 := getIDByName(c, "order:test_b")

	time.Sleep(1 * time.Second) // need some delay to make sure images sort predictable
	cli.BuildCmd(c, "order:test_c", build.WithDockerfile("FROM busybox\nRUN echo ccc > /result.txt\n"))
	imageID3 := getIDByName(c, "order:test_c")

	out := cli.DockerCmd(c, "image", "ls", "--format", `{{.Tag}}\t{{.ID}}`, "--no-trunc", "order").Stdout()
	c.Log(out)
	actual := getImageIDs(out)
	expected := []string{imageID3, imageID2, imageID1}
	assert.DeepEqual(c, actual, expected)
}

func (s *DockerCLIImagesSuite) TestImagesErrorWithInvalidFilterNameTest(c *testing.T) {
	out, _, err := dockerCmdWithError("images", "-f", "FOO=123")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "invalid filter"))
}

func (s *DockerCLIImagesSuite) TestImagesFilterLabelMatch(c *testing.T) {
	const imageName1 = "images_filter_test1"
	const imageName2 = "images_filter_test2"
	const imageName3 = "images_filter_test3"
	cli.BuildCmd(c, imageName1, build.WithDockerfile(`FROM busybox
                 LABEL match me`))
	image1ID := getIDByName(c, imageName1)

	cli.BuildCmd(c, imageName2, build.WithDockerfile(`FROM busybox
                 LABEL match="me too"`))
	image2ID := getIDByName(c, imageName2)

	cli.BuildCmd(c, imageName3, build.WithDockerfile(`FROM busybox
                 LABEL nomatch me`))
	image3ID := getIDByName(c, imageName3)

	out := cli.DockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match").Stdout()
	out = strings.TrimSpace(out)
	assert.Assert(c, is.Regexp(fmt.Sprintf("^[\\s\\w:]*%s[\\s\\w:]*$", image1ID), out))

	assert.Assert(c, is.Regexp(fmt.Sprintf("^[\\s\\w:]*%s[\\s\\w:]*$", image2ID), out))

	assert.Assert(c, !is.Regexp(fmt.Sprintf("^[\\s\\w:]*%s[\\s\\w:]*$", image3ID), out)().Success())

	out = cli.DockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=match=me too").Stdout()
	out = strings.TrimSpace(out)
	assert.Equal(c, out, image2ID)
}

// Regression : #15659
func (s *DockerCLIImagesSuite) TestCommitWithFilterLabel(c *testing.T) {
	// Create a container
	cli.DockerCmd(c, "run", "--name", "bar", "busybox", "/bin/sh")
	// Commit with labels "using changes"
	imageID := cli.DockerCmd(c, "commit", "-c", "LABEL foo.version=1.0.0-1", "-c", "LABEL foo.name=bar", "-c", "LABEL foo.author=starlord", "bar", "bar:1.0.0-1").Stdout()
	imageID = strings.TrimSpace(imageID)

	out := cli.DockerCmd(c, "images", "--no-trunc", "-q", "-f", "label=foo.version=1.0.0-1").Stdout()
	out = strings.TrimSpace(out)
	assert.Equal(c, out, imageID)
}

func (s *DockerCLIImagesSuite) TestImagesFilterSinceAndBefore(c *testing.T) {
	cli.BuildCmd(c, "testfilter:test_1", build.WithDockerfile("FROM busybox\nRUN echo 1 > /result.txt\n"))
	imageID1 := getIDByName(c, "testfilter:test_1")

	time.Sleep(1 * time.Second) // need some delay to make sure images sort predictable
	cli.BuildCmd(c, "testfilter:test_2", build.WithDockerfile("FROM busybox\nRUN echo 22 > /result.txt\n"))
	imageID2 := getIDByName(c, "testfilter:test_2")

	time.Sleep(1 * time.Second) // need some delay to make sure images sort predictable
	cli.BuildCmd(c, "testfilter:test_3", build.WithDockerfile("FROM busybox\nRUN echo 333 > /result.txt\n"))
	imageID3 := getIDByName(c, "testfilter:test_3")

	out := cli.DockerCmd(c, "image", "ls", "--format", `{{.Tag}}\t{{.ID}}\t{{.CreatedAt}}`, "--no-trunc", "testfilter").Stdout()
	c.Log(out)

	tests := []struct {
		name     string
		filter   string
		expected []string
	}{
		{
			name:     "since image 1",
			filter:   "since=testfilter:test_1",
			expected: []string{imageID3, imageID2},
		},
		{
			name:     "since image 1 digest",
			filter:   "since=" + imageID1,
			expected: []string{imageID3, imageID2},
		},
		{
			name:     "since image 2",
			filter:   "since=testfilter:test_2",
			expected: []string{imageID3},
		},
		{
			name:     "since image 2 digest",
			filter:   "since=" + imageID2,
			expected: []string{imageID3},
		},
		{
			name:     "before image 3",
			filter:   "before=testfilter:test_3",
			expected: []string{imageID2, imageID1},
		},
		{
			name:     "before image 3 digest",
			filter:   "before=" + imageID3,
			expected: []string{imageID2, imageID1},
		},
		{
			name:     "before image 2",
			filter:   "before=testfilter:test_2",
			expected: []string{imageID1},
		},
		{
			name:     "before image 2 digest",
			filter:   "before=" + imageID2,
			expected: []string{imageID1},
		},
	}

	for _, tc := range tests {
		c.Run(tc.filter, func(t *testing.T) {
			out = cli.DockerCmd(t, "image", "ls", "--format", `{{.Tag}}\t{{.ID}}`, "--no-trunc", "--filter", tc.filter, "testfilter").Stdout()
			actual := getImageIDs(out)
			assert.Check(t, is.DeepEqual(actual, tc.expected), "image list is not in the correct order")
		})
	}
}

func getImageIDs(out string) []string {
	var actual []string
	imgs := strings.SplitSeq(out, "\n")
	for l := range imgs {
		imgTag, imgDigest, _ := strings.Cut(l, "\t")
		if strings.HasPrefix(imgTag, "test_") {
			actual = append(actual, imgDigest)
		}
	}
	return actual
}

// FIXME(vdemeester) should be a unit test on `docker image ls`
func (s *DockerCLIImagesSuite) TestImagesFilterSpaceTrimCase(c *testing.T) {
	const imageName = "images_filter_test"
	// Build a image and fail to build so that we have dangling images ?
	cli.Docker(cli.Args("build", "-t", imageName), build.WithDockerfile(`FROM busybox
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
		out := cli.DockerCmd(c, "images", "-q", "-f", filter).Stdout()
		listing := strings.Split(out, "\n")
		sort.Strings(listing)
		imageListings[idx] = listing
	}

	for idx, listing := range imageListings {
		if idx < 4 && !reflect.DeepEqual(listing, imageListings[idx+1]) {
			for idx, errListing := range imageListings {
				fmt.Printf("out %d\n", idx)
				for _, img := range errListing {
					fmt.Print(img)
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
	containerID1 := cli.DockerCmd(c, "run", "-d", "busybox", "true").Stdout()
	containerID1 = strings.TrimSpace(containerID1)

	// tag as foobox
	imageID := cli.DockerCmd(c, "commit", containerID1, "foobox").Stdout()
	imageID = stringid.TruncateID(strings.TrimSpace(imageID))

	// overwrite the tag, making the previous image dangling
	cli.DockerCmd(c, "tag", "busybox", "foobox")

	out := cli.DockerCmd(c, "images", "-q", "-f", "dangling=true").Stdout()
	// Expect one dangling image
	assert.Equal(c, strings.Count(out, imageID), 1)

	out = cli.DockerCmd(c, "images", "-q", "-f", "dangling=false").Stdout()
	// dangling=false would not include dangling images
	assert.Assert(c, !strings.Contains(out, imageID))
	out = cli.DockerCmd(c, "images").Stdout()
	// docker images still include dangling images
	assert.Assert(c, is.Contains(out, imageID))
}

// FIXME(vdemeester) should be a unit test for `docker image ls`
func (s *DockerCLIImagesSuite) TestImagesWithIncorrectFilter(c *testing.T) {
	out, _, err := dockerCmdWithError("images", "-f", "dangling=invalid")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "invalid filter"))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureOnlyHeadsImagesShown(c *testing.T) {
	const dockerfile = `
        FROM busybox
        MAINTAINER docker
        ENV foo bar`
	const name = "scratch-image"
	result := cli.Docker(cli.Args("build", "-t", name), build.WithDockerfile(dockerfile))
	result.Assert(c, icmd.Success)
	id := getIDByName(c, name)

	// this is just the output of docker build
	// we're interested in getting the image id of the MAINTAINER instruction
	// and that's located at output, line 5, from 7 to end
	split := strings.Split(result.Combined(), "\n")
	intermediate := strings.TrimSpace(split[5][7:])

	out := cli.DockerCmd(c, "images").Stdout()
	// images shouldn't show non-heads images
	assert.Assert(c, !strings.Contains(out, intermediate))
	// images should contain final built images
	assert.Assert(c, is.Contains(out, stringid.TruncateID(id)))
}

func (s *DockerCLIImagesSuite) TestImagesEnsureImagesFromScratchShown(c *testing.T) {
	testRequires(c, DaemonIsLinux) // Windows does not support FROM scratch
	const dockerfile = `
        FROM scratch
        MAINTAINER docker`

	const name = "scratch-image"
	cli.BuildCmd(c, name, build.WithDockerfile(dockerfile))
	id := getIDByName(c, name)

	out := cli.DockerCmd(c, "images").Stdout()
	// images should contain images built from scratch
	assert.Assert(c, is.Contains(out, stringid.TruncateID(id)))
}

// For W2W - equivalent to TestImagesEnsureImagesFromScratchShown but Windows
// doesn't support from scratch
func (s *DockerCLIImagesSuite) TestImagesEnsureImagesFromBusyboxShown(c *testing.T) {
	const dockerfile = `
        FROM busybox
        MAINTAINER docker`
	const name = "busybox-image"

	cli.BuildCmd(c, name, build.WithDockerfile(dockerfile))
	id := getIDByName(c, name)

	out := cli.DockerCmd(c, "images").Stdout()
	// images should contain images built from busybox
	assert.Assert(c, is.Contains(out, stringid.TruncateID(id)))
}

// #18181
func (s *DockerCLIImagesSuite) TestImagesFilterNameWithPort(c *testing.T) {
	const tag = "a.b.c.d:5000/hello"
	cli.DockerCmd(c, "tag", "busybox", tag)
	out := cli.DockerCmd(c, "images", tag).Stdout()
	assert.Assert(c, is.Contains(out, tag))
	out = cli.DockerCmd(c, "images", tag+":latest").Stdout()
	assert.Assert(c, is.Contains(out, tag))
	out = cli.DockerCmd(c, "images", tag+":no-such-tag").Stdout()
	assert.Assert(c, !strings.Contains(out, tag))
}

func (s *DockerCLIImagesSuite) TestImagesFormat(c *testing.T) {
	// testRequires(c, DaemonIsLinux)
	const imageName = "myimage"
	cli.DockerCmd(c, "tag", "busybox", imageName+":v1")
	cli.DockerCmd(c, "tag", "busybox", imageName+":v2")

	out := cli.DockerCmd(c, "images", "--format", "{{.Repository}}", imageName).Stdout()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	expected := []string{imageName, imageName}
	var names []string
	names = append(names, lines...)
	assert.Assert(c, is.DeepEqual(names, expected), "Expected array with truncated names: %v, got: %v", expected, names)
}

// ImagesDefaultFormatAndQuiet
func (s *DockerCLIImagesSuite) TestImagesFormatDefaultFormat(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	// create container 1
	containerID1 := cli.DockerCmd(c, "run", "-d", "busybox", "true").Stdout()
	containerID1 = strings.TrimSpace(containerID1)

	// tag as foobox
	imageID := cli.DockerCmd(c, "commit", containerID1, "myimage").Stdout()
	imageID = stringid.TruncateID(strings.TrimSpace(imageID))

	const config = `{
		"imagesFormat": "{{ .ID }} default"
}`
	d, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(d)

	err = os.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0o644)
	assert.NilError(c, err)

	out := cli.DockerCmd(c, "--config", d, "images", "-q", "myimage").Stdout()
	assert.Equal(c, out, imageID+"\n", "Expected to print only the image id, got %v\n", out)
}
