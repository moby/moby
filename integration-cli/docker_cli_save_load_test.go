package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/cli/build"
	digest "github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

// save a repo using gz compression and try to load it using stdout
func (s *DockerSuite) TestSaveXzAndLoadRepoStdout(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-save-xz-and-load-repo-stdout"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test-xz-gz"
	out, _ := dockerCmd(c, "commit", name, repoName)

	dockerCmd(c, "inspect", repoName)

	repoTarball, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	assert.NilError(c, err, "failed to save repo: %v %v", out, err)
	deleteImages(repoName)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "load"},
		Stdin:   strings.NewReader(repoTarball),
	}).Assert(c, icmd.Expected{
		ExitCode: 1,
	})

	after, _, err := dockerCmdWithError("inspect", repoName)
	assert.ErrorContains(c, err, "", "the repo should not exist: %v", after)
}

// save a repo using xz+gz compression and try to load it using stdout
func (s *DockerSuite) TestSaveXzGzAndLoadRepoStdout(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-save-xz-gz-and-load-repo-stdout"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test-xz-gz"
	dockerCmd(c, "commit", name, repoName)

	dockerCmd(c, "inspect", repoName)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("xz", "-c"),
		exec.Command("gzip", "-c"))
	assert.NilError(c, err, "failed to save repo: %v %v", out, err)

	deleteImages(repoName)

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "load"},
		Stdin:   strings.NewReader(out),
	}).Assert(c, icmd.Expected{
		ExitCode: 1,
	})

	after, _, err := dockerCmdWithError("inspect", repoName)
	assert.ErrorContains(c, err, "", "the repo should not exist: %v", after)
}

func (s *DockerSuite) TestSaveSingleTag(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "foobar-save-single-tag-test"
	dockerCmd(c, "tag", "busybox:latest", fmt.Sprintf("%v:latest", repoName))

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc", repoName)
	cleanedImageID := strings.TrimSpace(out)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v:latest", repoName)),
		exec.Command("tar", "t"),
		exec.Command("grep", "-E", fmt.Sprintf("(^repositories$|%v)", cleanedImageID)))
	assert.NilError(c, err, "failed to save repo with image ID and 'repositories' file: %s, %v", out, err)
}

func (s *DockerSuite) TestSaveCheckTimes(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "busybox:latest"
	out, _ := dockerCmd(c, "inspect", repoName)
	var data []struct {
		ID      string
		Created time.Time
	}
	err := json.Unmarshal([]byte(out), &data)
	assert.NilError(c, err, "failed to marshal from %q: err %v", repoName, err)
	assert.Assert(c, len(data) != 0, "failed to marshal the data from %q", repoName)
	tarTvTimeFormat := "2006-01-02 15:04"
	out, err = RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command("tar", "tv"),
		exec.Command("grep", "-E", fmt.Sprintf("%s %s", data[0].Created.Format(tarTvTimeFormat), digest.Digest(data[0].ID).Hex())))
	assert.NilError(c, err, "failed to save repo with image ID and 'repositories' file: %s, %v", out, err)
}

func (s *DockerSuite) TestSaveImageId(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "foobar-save-image-id-test"
	dockerCmd(c, "tag", "emptyfs:latest", fmt.Sprintf("%v:latest", repoName))

	out, _ := dockerCmd(c, "images", "-q", "--no-trunc", repoName)
	cleanedLongImageID := strings.TrimPrefix(strings.TrimSpace(out), "sha256:")

	out, _ = dockerCmd(c, "images", "-q", repoName)
	cleanedShortImageID := strings.TrimSpace(out)

	// Make sure IDs are not empty
	assert.Assert(c, cleanedLongImageID != "", "Id should not be empty.")
	assert.Assert(c, cleanedShortImageID != "", "Id should not be empty.")

	saveCmd := exec.Command(dockerBinary, "save", cleanedShortImageID)
	tarCmd := exec.Command("tar", "t")

	var err error
	tarCmd.Stdin, err = saveCmd.StdoutPipe()
	assert.Assert(c, err == nil, "cannot set stdout pipe for tar: %v", err)
	grepCmd := exec.Command("grep", cleanedLongImageID)
	grepCmd.Stdin, err = tarCmd.StdoutPipe()
	assert.Assert(c, err == nil, "cannot set stdout pipe for grep: %v", err)

	assert.Assert(c, tarCmd.Start() == nil, "tar failed with error: %v", err)
	assert.Assert(c, saveCmd.Start() == nil, "docker save failed with error: %v", err)
	defer func() {
		saveCmd.Wait()
		tarCmd.Wait()
		dockerCmd(c, "rmi", repoName)
	}()

	out, _, err = runCommandWithOutput(grepCmd)

	assert.Assert(c, err == nil, "failed to save repo with image ID: %s, %v", out, err)
}

// save a repo and try to load it using flags
func (s *DockerSuite) TestSaveAndLoadRepoFlags(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-save-and-load-repo-flags"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	repoName := "foobar-save-load-test"

	deleteImages(repoName)
	dockerCmd(c, "commit", name, repoName)

	before, _ := dockerCmd(c, "inspect", repoName)

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName),
		exec.Command(dockerBinary, "load"))
	assert.NilError(c, err, "failed to save and load repo: %s, %v", out, err)

	after, _ := dockerCmd(c, "inspect", repoName)
	assert.Equal(c, before, after, "inspect is not the same after a save / load")
}

func (s *DockerSuite) TestSaveWithNoExistImage(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	imgName := "foobar-non-existing-image"

	out, _, err := dockerCmdWithError("save", "-o", "test-img.tar", imgName)
	assert.ErrorContains(c, err, "", "save image should fail for non-existing image")
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("No such image: %s", imgName)))
}

func (s *DockerSuite) TestSaveMultipleNames(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	repoName := "foobar-save-multi-name-test"

	// Make one image
	dockerCmd(c, "tag", "emptyfs:latest", fmt.Sprintf("%v-one:latest", repoName))

	// Make two images
	dockerCmd(c, "tag", "emptyfs:latest", fmt.Sprintf("%v-two:latest", repoName))

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", fmt.Sprintf("%v-one", repoName), fmt.Sprintf("%v-two:latest", repoName)),
		exec.Command("tar", "xO", "repositories"),
		exec.Command("grep", "-q", "-E", "(-one|-two)"),
	)
	assert.NilError(c, err, "failed to save multiple repos: %s, %v", out, err)
}

func (s *DockerSuite) TestSaveRepoWithMultipleImages(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	makeImage := func(from string, tag string) string {
		var (
			out string
		)
		out, _ = dockerCmd(c, "run", "-d", from, "true")
		cleanedContainerID := strings.TrimSpace(out)

		out, _ = dockerCmd(c, "commit", cleanedContainerID, tag)
		imageID := strings.TrimSpace(out)
		return imageID
	}

	repoName := "foobar-save-multi-images-test"
	tagFoo := repoName + ":foo"
	tagBar := repoName + ":bar"

	idFoo := makeImage("busybox:latest", tagFoo)
	idBar := makeImage("busybox:latest", tagBar)

	deleteImages(repoName)

	// create the archive
	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", repoName, "busybox:latest"),
		exec.Command("tar", "t"))
	assert.NilError(c, err, "failed to save multiple images: %s, %v", out, err)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var actual []string
	for _, l := range lines {
		if regexp.MustCompile(`^[a-f0-9]{64}\.json$`).Match([]byte(l)) {
			actual = append(actual, strings.TrimSuffix(l, ".json"))
		}
	}

	// make the list of expected layers
	out = inspectField(c, "busybox:latest", "Id")
	expected := []string{strings.TrimSpace(out), idFoo, idBar}

	// prefixes are not in tar
	for i := range expected {
		expected[i] = digest.Digest(expected[i]).Hex()
	}

	sort.Strings(actual)
	sort.Strings(expected)
	assert.Assert(c, is.DeepEqual(actual, expected), "archive does not contains the right layers: got %v, expected %v, output: %q", actual, expected, out)
}

// Issue #6722 #5892 ensure directories are included in changes
func (s *DockerSuite) TestSaveDirectoryPermissions(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	layerEntries := []string{"opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}
	layerEntriesAUFS := []string{"./", ".wh..wh.aufs", ".wh..wh.orph/", ".wh..wh.plnk/", "opt/", "opt/a/", "opt/a/b/", "opt/a/b/c"}

	name := "save-directory-permissions"
	tmpDir, err := os.MkdirTemp("", "save-layers-with-directories")
	assert.Assert(c, err == nil, "failed to create temporary directory: %s", err)
	extractionDirectory := filepath.Join(tmpDir, "image-extraction-dir")
	os.Mkdir(extractionDirectory, 0777)

	defer os.RemoveAll(tmpDir)
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
	RUN adduser -D user && mkdir -p /opt/a/b && chown -R user:user /opt/a
	RUN touch /opt/a/b/c && chown user:user /opt/a/b/c`))

	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", name),
		exec.Command("tar", "-xf", "-", "-C", extractionDirectory),
	)
	assert.NilError(c, err, "failed to save and extract image: %s", out)

	dirs, err := os.ReadDir(extractionDirectory)
	assert.NilError(c, err, "failed to get a listing of the layer directories: %s", err)

	found := false
	for _, entry := range dirs {
		var entriesSansDev []string
		if entry.IsDir() {
			layerPath := filepath.Join(extractionDirectory, entry.Name(), "layer.tar")

			f, err := os.Open(layerPath)
			assert.NilError(c, err, "failed to open %s: %s", layerPath, err)

			defer f.Close()

			entries, err := listTar(f)
			for _, e := range entries {
				if !strings.Contains(e, "dev/") {
					entriesSansDev = append(entriesSansDev, e)
				}
			}
			assert.NilError(c, err, "encountered error while listing tar entries: %s", err)

			if reflect.DeepEqual(entriesSansDev, layerEntries) || reflect.DeepEqual(entriesSansDev, layerEntriesAUFS) {
				found = true
				break
			}
		}
	}

	assert.Assert(c, found, "failed to find the layer with the right content listing")
}

func listTar(f io.Reader) ([]string, error) {
	tr := tar.NewReader(f)
	var entries []string

	for {
		th, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
		entries = append(entries, th.Name)
	}
}

// Test loading a weird image where one of the layers is of zero size.
// The layer.tar file is actually zero bytes, no padding or anything else.
// See issue: 18170
func (s *DockerSuite) TestLoadZeroSizeLayer(c *testing.T) {
	// this will definitely not work if using remote daemon
	// very weird test
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)

	dockerCmd(c, "load", "-i", "testdata/emptyLayer.tar")
}

func (s *DockerSuite) TestSaveLoadParents(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	makeImage := func(from string, addfile string) string {
		var (
			out string
		)
		out, _ = dockerCmd(c, "run", "-d", from, "touch", addfile)
		cleanedContainerID := strings.TrimSpace(out)

		out, _ = dockerCmd(c, "commit", cleanedContainerID)
		imageID := strings.TrimSpace(out)

		dockerCmd(c, "rm", "-f", cleanedContainerID)
		return imageID
	}

	idFoo := makeImage("busybox", "foo")
	idBar := makeImage(idFoo, "bar")

	tmpDir, err := os.MkdirTemp("", "save-load-parents")
	assert.NilError(c, err)
	defer os.RemoveAll(tmpDir)

	c.Log("tmpdir", tmpDir)

	outfile := filepath.Join(tmpDir, "out.tar")

	dockerCmd(c, "save", "-o", outfile, idBar, idFoo)
	dockerCmd(c, "rmi", idBar)
	dockerCmd(c, "load", "-i", outfile)

	inspectOut := inspectField(c, idBar, "Parent")
	assert.Equal(c, inspectOut, idFoo)

	inspectOut = inspectField(c, idFoo, "Parent")
	assert.Equal(c, inspectOut, "")
}

func (s *DockerSuite) TestSaveLoadNoTag(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	name := "saveloadnotag"

	buildImageSuccessfully(c, name, build.WithDockerfile("FROM busybox\nENV foo=bar"))
	id := inspectField(c, name, "Id")

	// Test to make sure that save w/o name just shows imageID during load
	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", id),
		exec.Command(dockerBinary, "load"))
	assert.NilError(c, err, "failed to save and load repo: %s, %v", out, err)

	// Should not show 'name' but should show the image ID during the load
	assert.Assert(c, !strings.Contains(out, "Loaded image: "))
	assert.Assert(c, strings.Contains(out, "Loaded image ID:"))
	assert.Assert(c, strings.Contains(out, id))
	// Test to make sure that save by name shows that name during load
	out, err = RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "save", name),
		exec.Command(dockerBinary, "load"))
	assert.NilError(c, err, "failed to save and load repo: %s, %v", out, err)

	assert.Assert(c, strings.Contains(out, "Loaded image: "+name+":latest"))
	assert.Assert(c, !strings.Contains(out, "Loaded image ID:"))
}
